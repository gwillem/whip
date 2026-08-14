package runners

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"dario.cat/mergo"
	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/model"
	"github.com/ieee0824/go-deepmerge"
	"github.com/spf13/afero"
)

const (
	Unknown int = iota
	Success
	Failed
	Skipped
)

type (
	runnerFunc func(*model.Task) model.TaskResult

	runnerMeta struct {
		requiredArgs []string
		optionalArgs []string

		// stringArg names the argument a bare string value is assigned to,
		// verbatim. A runner that leaves it empty keeps the historic
		// behaviour: the string is split on spaces and every token holding an
		// "=" becomes a named argument.
		//
		// That dialect is right for apt package names and tree prefix
		// metadata, and catastrophic for a runner whose argument is a
		// command:
		//
		//	shell: mysql -e "SET @a=1"
		//	  -> map[@a:1" _args:mysql -e "SET]
		//
		// which runs, exits zero, and does something else entirely.
		stringArg string
	}

	runner struct {
		run    runnerFunc
		meta   runnerMeta
		prerun runnerFunc
	}
)

var (
	fs      afero.Fs
	fsutil  *afero.Afero
	runners = map[string]runner{}
)

func init() {
	if fs == nil {
		// fmt.Println("creating layover FS")
		fs = afero.NewOsFs()
		fsutil = &afero.Afero{Fs: fs}
	}
}

// StringArg reports which argument a bare string value belongs in for the
// named runner, and whether that runner wants the value left unparsed.
func StringArg(runner string) (string, bool) {
	r, ok := runners[runner]
	if !ok || r.meta.stringArg == "" {
		return "", false
	}
	return r.meta.stringArg, true
}

func All() []string {
	keys := []string{}
	for k := range runners {
		keys = append(keys, k)
	}
	sort.StringSlice(keys).Sort()
	return keys
}

func failure(msg ...any) model.TaskResult {
	output := ""

	_, file, line, ok := runtime.Caller(1)
	if ok {
		output += fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}

	for _, m := range msg {
		if _, ok := m.(error); ok {
			output += " ERR"
		}
		output += fmt.Sprintf(" %v", m)
	}
	output = strings.TrimSpace(output)

	return model.TaskResult{
		Status:  Failed,
		Changed: false,
		Output:  output,
	}
}

func registerRunner(name string, r runner) {
	runners[name] = r
}

// PreRun runs the controller-side half of a task, for the runners that have
// one. Today that is `tree`, which reads the source directory here so the
// files travel with the job rather than being fetched by the target.
//
// Arguments are rendered before the pre-run sees them, for the same reason Run
// renders them: a source path is as much a candidate for a variable as
// anything else.
//
//   - name: restore this instance's stored catalogue
//     tree:
//     src: ../../seed/{{ instance }}
//     dst: /var/tmp/seed
//
// used to glob the literal string, find nothing, and fail on the target with
// "no assets found" - a message about the destination, for a mistake made on
// the controller. The workaround was a prerun shell that copied the right
// directory to a fixed path first.
//
// Rendering twice is harmless: Run renders the same arguments again on the
// deputy, and a value with no {{ }} left in it is returned unchanged. The
// assets the pre-run attaches are not strings and pass through untouched.
func PreRun(task *model.Task, playVars model.TaskVars) (tr model.TaskResult) {
	runner, ok := runners[task.Runner]
	if !ok {
		log.Fatal("Runner not found, should have been validated", task.Runner)
	}

	if runner.prerun == nil {
		tr.Status = Skipped
		return tr
	}

	// todo: isolate this
	// merge global and task vars
	mergedVars, err := deepmerge.Merge(map[string]any(playVars), map[string]any(task.Vars))
	if err != nil {
		tr.Status = Failed
		tr.Output = err.Error()
		return tr
	}
	task.Vars = mergedVars.(map[string]any)

	// A loop item first, so that a {{ }} written inside one is substituted
	// before the argument that carries it. Same two passes as Run.
	if item, ok := task.Vars["item"]; ok {
		rendered, err := renderValue(item, task.Vars)
		if err != nil {
			tr.Status = Failed
			tr.Output = err.Error()
			return tr
		}
		task.Vars["item"] = rendered
	}

	// In place: task is a copy of the playbook's entry, but Args is a map and
	// therefore shared with it, which is also how the pre-run's assets reach
	// the job that is sent to the target.
	for k, v := range task.Args {
		rendered, err := renderValue(v, task.Vars)
		if err != nil {
			tr.Status = Failed
			tr.Output = fmt.Sprintf("%s: %v", k, err)
			return tr
		}
		task.Args[k] = rendered
	}

	tr = runner.prerun(task)
	tr.Task = task
	return tr
}

// Run is called by the deputy to run a task on localhost.
func Run(task *model.Task, playVars model.TaskVars) (tr model.TaskResult) {
	start := time.Now()
	fail := func(msg string) model.TaskResult {
		return model.TaskResult{
			Status: Failed,
			Output: msg,
			Task:   task,
		}
	}

	defer func() {
		if r := recover(); r != nil {
			trace := string(debug.Stack())
			log.Debug("Panic in runner", r, trace)
			// get rid of first 5 lines
			// trace = strings.Join(strings.Split(trace, "\n")[5:], "\n")
			tr = fail(trace) // will return from parent func
		}
	}()

	runner, ok := runners[task.Runner]
	if !ok {
		return fail("No runner found for task '" + task.Runner + "'") // todo, is empty for unknown runners
	}

	if runner.run == nil {
		// local_action perhaps?
		return model.TaskResult{
			Output: "skipped, no runner",
			Task:   task,
		}
	}

	if e := mergo.Merge(&task.Vars, playVars); e != nil {
		return fail(e.Error())
	}

	// A loop item is substituted into the argument in one pass, so a `{{ }}`
	// written INSIDE a loop item used to arrive at the target as literal
	// text. Rendering the item first makes the two passes explicit:
	//
	//	loop: ["innodb_buffer_pool_size = {{ pool }}M"]
	//
	// wrote that string into a live my.cnf verbatim, and the database then
	// refused to start.
	if item, ok := task.Vars["item"]; ok {
		rendered, err := renderValue(item, task.Vars)
		if err != nil {
			return fail(err.Error())
		}
		task.Vars["item"] = rendered
	}

	// Argument substitution, recursively. Only top-level strings used to be
	// rendered, so a variable inside a list or a map -- an apt package list,
	// a tree prefix table -- reached the runner unrendered.
	for k, v := range task.Args {
		rendered, err := renderValue(v, task.Vars)
		if err != nil {
			return fail(err.Error())
		}
		task.Args[k] = rendered
	}

	// Guards, after templating so they can use the task's own variables, and
	// before the runner so a skip costs nothing.
	if skip, reason, err := shouldSkip(task); err != nil {
		return fail(err.Error())
	} else if skip {
		return model.TaskResult{Status: Success, Output: reason, Task: task}
	}

	tr = runner.run(task)
	tr.Duration = time.Since(start)
	tr.Task = task
	return tr
}

// renderValue applies the template engine to every string inside v, however
// deeply nested. Whip templated only top-level string arguments, which meant a
// variable inside a list or a map silently survived as literal text.
func renderValue(v any, vars model.TaskVars) (any, error) {
	switch t := v.(type) {
	case string:
		return tplParseString(t, vars)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			r, err := renderValue(e, vars)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	case []string:
		out := make([]string, len(t))
		for i, e := range t {
			r, err := tplParseString(e, vars)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			r, err := renderValue(e, vars)
			if err != nil {
				return nil, err
			}
			out[k] = r
		}
		return out, nil
	default:
		return v, nil
	}
}

// shouldSkip evaluates the three guards a task may carry, cheapest first.
//
// creates and removes are paths, checked with a stat and no shell: they say
// "this has already happened" directly, which is what the overwhelming
// majority of `unless` commands were written to say.
//
// All three are templated. A guard is the one place where getting a variable
// wrong is invisible: an unrendered `{{ x }}/lock` names a path that cannot
// exist, so the guard never fires and the task silently runs every time.
func shouldSkip(task *model.Task) (bool, string, error) {
	if task.Creates != "" {
		p, err := tplParseString(task.Creates, task.Vars)
		if err != nil {
			return false, "", err
		}
		exists, err := fsutil.Exists(p)
		if err != nil {
			return false, "", err
		}
		if exists {
			return true, fmt.Sprintf("skipped, %s already exists", p), nil
		}
	}
	if task.Removes != "" {
		p, err := tplParseString(task.Removes, task.Vars)
		if err != nil {
			return false, "", err
		}
		exists, err := fsutil.Exists(p)
		if err != nil {
			return false, "", err
		}
		if !exists {
			return true, fmt.Sprintf("skipped, %s is already absent", p), nil
		}
	}
	if task.Unless != "" {
		guard, err := tplParseString(task.Unless, task.Vars)
		if err != nil {
			return false, "", err
		}
		if _, err := exec.Command("/bin/sh", "-c", guard).CombinedOutput(); err == nil {
			return true, fmt.Sprintf("skipped, 'unless' clause succeeded (%v)", guard), nil
		}
	}
	return false, "", nil
}

// universalArgs are read by the framework rather than by any one runner, so
// they are never "unknown".
var universalArgs = []string{
	"_args",        // the bare string value
	"_assets",      // files shipped alongside a task
	"changed_when", // post-hoc changed detection
	"oldArgs",      // legacy string form of `args:`
}

// UnknownArgs reports arguments the named runner does not read.
//
// Only runners that actually declare their argument list get an opinion: most
// declare nothing, and refusing everything they were not told about would
// reject working playbooks. requiredArgs and optionalArgs were declared and
// read by nothing at all until now, which is why a typo in an argument name
// was silently ignored and the task did something other than what was written.
func UnknownArgs(name string, args model.TaskArgs) []string {
	r, ok := runners[name]
	if !ok {
		return nil
	}
	if len(r.meta.requiredArgs) == 0 && len(r.meta.optionalArgs) == 0 {
		return nil
	}
	allowed := map[string]bool{}
	for _, a := range r.meta.requiredArgs {
		allowed[a] = true
	}
	for _, a := range r.meta.optionalArgs {
		allowed[a] = true
	}
	for _, a := range universalArgs {
		allowed[a] = true
	}
	if r.meta.stringArg != "" {
		allowed[r.meta.stringArg] = true
	}
	var out []string
	for k := range args {
		// The tree runner's prefix metadata is written as absolute paths used
		// as keys, which are data rather than argument names.
		if strings.HasPrefix(k, "/") {
			continue
		}
		if !allowed[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// MissingArgs reports required arguments the task does not supply.
func MissingArgs(name string, args model.TaskArgs) []string {
	r, ok := runners[name]
	if !ok {
		return nil
	}
	var out []string
	for _, a := range r.meta.requiredArgs {
		if _, ok := args[a]; !ok {
			out = append(out, a)
		}
	}
	sort.Strings(out)
	return out
}
