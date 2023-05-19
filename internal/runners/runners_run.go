package runners

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/barkimedes/go-deepcopy"
	"github.com/ieee0824/go-deepmerge"
	"github.com/spf13/afero"
)

const (
	ok int = iota
	failed

	defaultArg = "_args"
)

type (
	Task struct {
		Runner string         `json:"runner,omitempty"`
		Name   string         `json:"name,omitempty"`
		Args   TaskArgs       `json:"args,omitempty"`
		Loop   []any          `json:"loop,omitempty"`
		Vars   map[string]any `json:"vars,omitempty"`
	}

	TaskArgs map[string]any

	TaskResult struct {
		PlayIdx   int           `json:"play_idx,omitempty"`
		TaskIdx   int           `json:"task_idx,omitempty"`
		TaskTotal int           `json:"task_total,omitempty"`
		Host      string        `json:"target,omitempty"`
		Changed   bool          `json:"changed,omitempty"`
		Output    string        `json:"output,omitempty"`
		Status    int           `json:"status_code,omitempty"`
		Duration  time.Duration `json:"duration,omitempty"`
		Task      Task          `json:"task,omitempty"`
	}
)

var (
	fs     afero.Fs
	fsutil *afero.Afero

	runners = map[string]struct {
		fn   runnerFunc
		meta runnerMeta
	}{}

	facts = gatherFacts()
)

func init() {
	if fs == nil {
		// fmt.Println("creating layover FS")
		fs = afero.NewOsFs()
		fsutil = &afero.Afero{Fs: fs}
	}
}

func All() []string {
	keys := []string{}
	for k := range runners {
		keys = append(keys, k)
	}
	sort.StringSlice(keys).Sort()
	return keys
}

type (
	runnerFunc func(TaskArgs) TaskResult
	runnerMeta struct {
		requiredArgs []string
		optionalArgs []string
	}
)

func (tr TaskResult) String() string {
	return fmt.Sprintf("TaskResult %s from %s (%.2f sec)", tr.Task.Runner, tr.Host, tr.Duration.Seconds())
}

func (ta TaskArgs) String(s string) string {
	return ta[s].(string)
}

func (ta TaskArgs) StringSlice(s string) []string {
	switch val := ta[s].(type) {
	case []string:
		return val
	default:
		return []string{fmt.Sprintf("%v", val)}
	}
}

func (t Task) Clone() Task {
	return deepcopy.MustAnything(t).(Task)
}

func failure(msg ...any) TaskResult {
	output := ""
	for _, m := range msg {
		if _, ok := m.(error); ok {
			output += ":"
		}
		output += " "
		output += fmt.Sprintf("%v", m)
	}
	output = strings.TrimSpace(output)

	return TaskResult{
		Status:  failed,
		Changed: false,
		Output:  output,
	}
}

func registerRunner(name string, fn runnerFunc, meta runnerMeta) {
	runners[name] = struct {
		fn   runnerFunc
		meta runnerMeta
	}{fn, meta}
}

// Run is called by the deputy to run a task on localhost.
func Run(task Task, vars map[string]any) (tr TaskResult) {

	start := time.Now()
	fail := func(msg string) TaskResult {
		return TaskResult{
			Status: failed,
			Output: msg,
			Task:   task,
		}
	}

	// fmt.Println("Running", task.Type)
	runner, ok := runners[task.Runner]
	if !ok {
		return fail("No runner found for task '" + task.Runner + "'")
	}

	// merge global and task vars

	mergedVars, err := deepmerge.Merge(vars, task.Vars)
	if err != nil {
		return fail(err.Error())
	}
	task.Vars = mergedVars.(map[string]any)

	// arg substitution, notably for loop {{item}}
	for k, v := range task.Args {
		if val, ok := v.(string); ok {
			new, err := tplParse(val, task.Vars)
			if err != nil {
				return fail(err.Error())
			}
			task.Args[k] = new
		}
	}

	tr = runner.fn(task.Args)
	tr.Duration = time.Since(start)
	tr.Task = task
	return tr
}
