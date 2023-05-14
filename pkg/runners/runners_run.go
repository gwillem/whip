package runners

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/barkimedes/go-deepcopy"
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
		TaskID    int           `json:"task_id,omitempty"`
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

func (ta TaskArgs) Key(s string) string {
	return ta[s].(string)
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
func Run(task Task) (tr TaskResult) {
	if fs == nil {
		// fmt.Println("creating layover FS")
		fs = afero.NewOsFs()
		fsutil = &afero.Afero{Fs: fs}
	}

	// fmt.Println("Running", task.Type)
	runner, ok := runners[task.Runner]
	if !ok {
		return TaskResult{
			Status: failed,
			Output: fmt.Sprintf("No runner found for task '%s'", task.Runner),
			Task:   task,
		}
	}
	start := time.Now()

	// loop item? should replace this with generic vars substitution
	if task.Vars["item"] != nil {
		item, ok := task.Vars["item"].(string)
		if !ok {
			return TaskResult{
				Status: failed,
				Output: "loop only supports strings for now",
				Task:   task,
			}
		}

		// subTask := task.Clone()
		for k, v := range task.Args {
			if val, ok := v.(string); ok {
				new := strings.ReplaceAll(val, "{{item}}", item)
				task.Args[k] = new
				// fmt.Println("substituting", val, "with", new)
			}
		}
		// // run cloned task
		// subtr := runner.fn(subTask.Args)

		// if subtr.Changed {
		// 	tr.Changed = true
		// }
		// 	// merge tr with parent tr
		// 	tr.Output += strings.TrimSpace(subtr.Output) + "\n"
		// 	tr.Status = subtr.Status
		// 	if subtr.Status == failed {
		// 		// fmt.Println("failure, stopping")
		// 		break
		// 	}
		// }
	}
	tr = runner.fn(task.Args)
	tr.Duration = time.Since(start)
	tr.Task = task
	return tr
}
