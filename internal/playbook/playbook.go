package playbook

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/parser"
	"github.com/gwillem/whip/internal/runners"
	"github.com/mitchellh/mapstructure"

	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

const (
// defaultAssetPath = "files"
)

var StringToSliceSep = regexp.MustCompile(`,\s*`)

func Load(path string) (*model.Playbook, error) {
	rawData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(rawData, filepath.Dir(path))
}

// Parse turns playbook source into a playbook. dir is the directory relative
// paths inside it -- `include` and `vars_files` -- are resolved against, which
// is the playbook's own directory and not the working directory: a playbook
// has to mean the same thing wherever whip is invoked from.
func Parse(rawData []byte, dir string) (*model.Playbook, error) {
	return parseDepth(rawData, dir, 0)
}

// maxIncludeDepth bounds `include` recursion. A cycle would otherwise be an
// out-of-memory kill with no message.
const maxIncludeDepth = 16

func parseDepth(rawData []byte, dir string, depth int) (*model.Playbook, error) {
	if depth > maxIncludeDepth {
		return nil, fmt.Errorf("include nested more than %d deep; is there a cycle?", maxIncludeDepth)
	}

	var anyMap any
	if e := yaml.Unmarshal(rawData, &anyMap); e != nil {
		return nil, e
	}

	entries, ok := anyMap.([]any)
	if !ok {
		return nil, fmt.Errorf("a playbook must be a list of plays, got %T", anyMap)
	}

	// Expand `- include: other.yml` entries before decoding, so an included
	// file is an ordinary playbook and needs no special case anywhere else.
	expanded := make([]any, 0, len(entries))
	for _, entry := range entries {
		m, ok := entry.(map[string]any)
		if !ok {
			expanded = append(expanded, entry)
			continue
		}
		inc, ok := m["include"]
		if !ok {
			expanded = append(expanded, entry)
			continue
		}
		if len(m) != 1 {
			return nil, fmt.Errorf("include must be the only key in its list entry, got %v", keysOf(m))
		}
		name, ok := inc.(string)
		if !ok {
			return nil, fmt.Errorf("include takes a path, got %T", inc)
		}
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("include %s: %w", name, err)
		}
		var sub any
		if e := yaml.Unmarshal(data, &sub); e != nil {
			return nil, fmt.Errorf("include %s: %w", name, e)
		}
		subEntries, ok := sub.([]any)
		if !ok {
			return nil, fmt.Errorf("include %s: a playbook must be a list of plays", name)
		}
		// Recurse so an included file may itself include, resolving relative
		// to its own directory.
		nested, err := parseDepth(data, filepath.Dir(path), depth+1)
		if err != nil {
			return nil, err
		}
		_ = subEntries
		for i := range *nested {
			expanded = append(expanded, marker{play: (*nested)[i]})
		}
	}

	pb, err := decodeEntries(expanded)
	if err != nil {
		return nil, fmt.Errorf("yaml error: %w", err)
	}

	if err := loadVarsFiles(pb, dir); err != nil {
		return nil, err
	}
	if err := validate(pb); err != nil {
		return nil, err
	}

	expandPlaybookLoops(pb)
	return pb, nil
}

// marker carries an already-decoded play through the entry list, so an
// included playbook is not decoded twice.
type marker struct{ play model.Play }

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func decodeEntries(entries []any) (*model.Playbook, error) {
	pb := model.Playbook{}
	raw := make([]any, 0, len(entries))
	for _, e := range entries {
		if m, ok := e.(marker); ok {
			// Flush anything pending, then append the decoded play in order.
			if len(raw) > 0 {
				sub, err := yamlToPlaybook(raw)
				if err != nil {
					return nil, err
				}
				pb = append(pb, *sub...)
				raw = raw[:0]
			}
			pb = append(pb, m.play)
			continue
		}
		raw = append(raw, e)
	}
	if len(raw) > 0 {
		sub, err := yamlToPlaybook(raw)
		if err != nil {
			return nil, err
		}
		pb = append(pb, *sub...)
	}
	return &pb, nil
}

// loadVarsFiles merges each play's vars_files into its Vars. Files are merged
// in order and the play's own Vars win, which is the least surprising
// precedence: what you can see in the playbook beats what you cannot.
func loadVarsFiles(pb *model.Playbook, dir string) error {
	for i := range *pb {
		play := &(*pb)[i]
		if len(play.VarsFiles) == 0 {
			continue
		}
		merged := map[string]any{}
		for _, f := range play.VarsFiles {
			path := f
			if !filepath.IsAbs(path) {
				path = filepath.Join(dir, path)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("vars_files %s: %w", f, err)
			}
			var vars map[string]any
			if e := yaml.Unmarshal(data, &vars); e != nil {
				return fmt.Errorf("vars_files %s: %w", f, e)
			}
			maps.Copy(merged, vars)
		}
		maps.Copy(merged, play.Vars)
		play.Vars = merged
	}
	return nil
}

func yamlToPlaybook(y any) (*model.Playbook, error) {
	pb := model.Playbook{}
	md := mapstructure.Metadata{}

	config := &mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           &pb,
		Metadata:         &md,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			parseTasksFunc(),
			parseStringToSlice(),
		),
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, err
	}

	err = decoder.Decode(y)
	if err != nil {
		return nil, err
	}

	// A field whip does not recognise is an error, not a warning.
	//
	// It used to be logged at Warn, which cmd/whip suppresses at its default
	// verbosity, so a misspelled key simply did nothing: `unles:` never
	// guarded anything, `notifiy:` never triggered a handler, and the deploy
	// reported success. Every one of those is a silent behaviour change on a
	// production host, and catching them is most of why the honeypot fleet
	// carries a 395-line playbook linter.
	if unknown, refused := classifyUnused(md.Unused); len(refused) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(refused, "\n"))
	} else if len(unknown) > 0 {
		return nil, fmt.Errorf("unknown field(s) in playbook: %s", strings.Join(unknown, ", "))
	}
	return &pb, nil
}

func parseTasksFunc() mapstructure.DecodeHookFunc {
	return func(f, t reflect.Type, data any) (any, error) {
		if t != reflect.TypeFor[model.Task]() {
			return data, nil
		}
		if f != reflect.TypeFor[map[string]any]() {
			return nil, fmt.Errorf("expected map[string]any{}, got %v", f)
		}

		task := data.(map[string]any)
		specificArgs := map[string]any{}

		// parse runner argument
		for k, v := range task {
			if !slices.Contains(runners.All(), k) {
				continue
			}

			if task["runner"] != nil {
				return nil, fmt.Errorf("single task cannot have multiple runners (%s and %s)", task["runner"], k)
			}

			delete(task, k)
			task["runner"] = k

			// this is the value of the runner argument, so "shell: echo hello"
			switch v := v.(type) {
			case string:
				// A runner that declares a stringArg wants the value intact.
				// Everything else keeps the historic key=value dialect, which
				// is what apt package names and tree prefixes are written in.
				if key, literal := runners.StringArg(k); literal {
					specificArgs = map[string]any{key: v}
				} else {
					specificArgs = parser.ParseArgString(v)
				}
			case map[string]any:
				specificArgs = v
			default:
				return nil, fmt.Errorf("unexpected type for task arg: %v", v)
			}
			continue
		}

		// changed_when is a runner argument rather than a Task field, because
		// the runner is what evaluates it. Written at task level -- which is
		// where every other tool puts it, and where a reader expects it --
		// it would otherwise be rejected as an unknown field, so it is
		// hoisted here.
		if v, ok := task["changed_when"]; ok {
			delete(task, "changed_when")
			specificArgs["changed_when"] = v
		}

		switch v := task["args"].(type) {
		case string:
			specificArgs["oldArgs"] = v
			task["args"] = specificArgs
		case map[string]any:
			maps.Copy(v, specificArgs)
		case nil:
			task["args"] = specificArgs
		default:
			return nil, fmt.Errorf("unexpected type for task arg: %v", v)
		}
		return data, nil
	}
}

func parseStringToSlice() mapstructure.DecodeHookFunc {
	return func(f, t reflect.Kind, data any) (any, error) {
		if f != reflect.String || t != reflect.Slice {
			return data, nil
		}
		return StringToSliceSep.Split(data.(string), -1), nil
	}
}

// expandPlaybookLoops takes a playbook and expands any tasks that have a Loop,
// replacing them with multiple tasks, each loop item copied into task.Vars
func expandPlaybookLoops(pb *model.Playbook) {
	for playidx := range *pb {
		play := &(*pb)[playidx]
		for i := len(play.Tasks) - 1; i >= 0; i-- { // reverse range, because we are expanding the slice in place
			if loops := play.Tasks[i].Loop; loops != nil {
				newTasks := []model.Task{}
				for _, l := range loops {
					newTask := play.Tasks[i].Clone()
					newTask.Vars["item"] = l
					newTask.Loop = nil
					newTasks = append(newTasks, newTask)
				}
				// remove this task from the playbook
				// and insert len(Loop) new tasks in its place
				play.Tasks = slices.Replace(play.Tasks, i, i+1, newTasks...)
			}
		}
	}
}

// validate checks what the decoder cannot: that every task names a runner,
// that its arguments are ones the runner reads, and that every handler it
// notifies exists. All three were previously silent.
func validate(pb *model.Playbook) error {
	var problems []string
	for i := range *pb {
		play := &(*pb)[i]
		where := play.Name
		if where == "" {
			where = fmt.Sprintf("play %d", i+1)
		}

		handlers := map[string]bool{}
		for _, h := range play.Handlers {
			handlers[h.Name] = true
		}

		for _, task := range append(append([]model.Task{}, play.Tasks...), play.Handlers...) {
			name := task.Name
			if name == "" {
				name = task.Runner
			}
			if task.Runner == "" {
				problems = append(problems,
					fmt.Sprintf("%s: task %q names no runner; known runners are %s",
						where, name, strings.Join(runners.All(), ", ")))
				continue
			}
			if !slices.Contains(runners.All(), task.Runner) {
				problems = append(problems,
					fmt.Sprintf("%s: task %q uses unknown runner %q", where, name, task.Runner))
				continue
			}
			for _, arg := range runners.UnknownArgs(task.Runner, task.Args) {
				problems = append(problems,
					fmt.Sprintf("%s: task %q passes %q, which the %s runner does not read",
						where, name, arg, task.Runner))
			}
			// A notify naming a handler that does not exist silently does
			// nothing, which reads exactly like a handler that did not need
			// to fire.
			for _, n := range task.Notify {
				if !handlers[n] {
					problems = append(problems,
						fmt.Sprintf("%s: task %q notifies %q, which is not a handler in this play",
							where, name, n))
				}
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "\n"))
	}
	return nil
}

// ansibleNoOps are Ansible keywords whip accepts and ignores. Keeping them
// loadable is deliberate: whip sticks to Ansible's verbiage so a playbook can
// be moved across without a rewrite, and whip's own fixtures carry them.
var ansibleNoOps = map[string]bool{
	"gather_facts":     true, // whip gathers no facts
	"remote_user":      true, // the user is part of the host string
	"connection":       true,
	"any_errors_fatal": true,
	"serial":           true,
	"strategy":         true,
}

// ansibleRefused are Ansible keywords that would change what a playbook DOES
// if they were honoured. Ignoring them silently is the dangerous option: a
// play written with `become: true` and run without it does the wrong work as
// the wrong user and reports success.
var ansibleRefused = map[string]string{
	"become":        "whip has no privilege escalation; connect as the user you need (user@host)",
	"become_user":   "whip has no privilege escalation; connect as the user you need (user@host)",
	"become_method": "whip has no privilege escalation; connect as the user you need (user@host)",
	"sudo":          "whip has no privilege escalation; connect as the user you need (user@host)",
	"sudo_user":     "whip has no privilege escalation; connect as the user you need (user@host)",
	"vars_prompt":   "whip is non-interactive; use vars_files or -e",
	"when":          "whip has no conditionals; use `unless`, `creates` or `removes`",
}

// classifyUnused splits the decoder's unused-field list into fields whip has
// simply never heard of and fields it knows about and refuses.
func classifyUnused(unused []string) (unknown, refused []string) {
	for _, f := range unused {
		leaf := f
		if i := strings.LastIndex(leaf, "."); i >= 0 {
			leaf = leaf[i+1:]
		}
		switch {
		case ansibleNoOps[leaf]:
			// Accepted and ignored, on purpose.
		case ansibleRefused[leaf] != "":
			refused = append(refused, fmt.Sprintf("%s: %s", f, ansibleRefused[leaf]))
		default:
			unknown = append(unknown, f)
		}
	}
	sort.Strings(unknown)
	sort.Strings(refused)
	return unknown, refused
}
