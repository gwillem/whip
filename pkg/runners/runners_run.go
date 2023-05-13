package runners

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gwillem/chief-whip/pkg/whip"
	"github.com/spf13/afero"
)

const (
	ok int = iota
	failed

	defaultArg = "_args"
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
	runnerFunc func(whip.TaskArgs) whip.TaskResult
	runnerMeta struct {
		requiredArgs []string
		optionalArgs []string
		wantItems    bool
	}
)

func failure(msg ...any) whip.TaskResult {
	output := ""
	for _, m := range msg {
		if _, ok := m.(error); ok {
			output += ":"
		}
		output += " "
		output += fmt.Sprintf("%v", m)
	}
	output = strings.TrimSpace(output)

	return whip.TaskResult{
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
func Run(task whip.Task) (tr whip.TaskResult) {
	if fs == nil {
		// fmt.Println("creating layover FS")
		fs = afero.NewOsFs()
		fsutil = &afero.Afero{Fs: fs}
	}

	// fmt.Println("Running", task.Type)
	runner, ok := runners[task.Runner]
	if !ok {
		return whip.TaskResult{
			Status: failed,
			Output: fmt.Sprintf("No runner found for task '%s'", task.Runner),
			Task:   task,
		}
	}
	start := time.Now()

	// with_items?
	if task.Loop != nil && !runner.meta.wantItems {
		for _, rawItem := range task.Loop {
			item, ok := rawItem.(string)
			if !ok {
				return whip.TaskResult{
					Status: failed,
					Output: "loop only supports strings for now",
					Task:   task,
				}
			}
			subTask := task.Clone()
			for k, v := range subTask.Args {
				if val, ok := v.(string); ok {
					new := strings.ReplaceAll(val, "{{item}}", item)
					subTask.Args[k] = new
					// fmt.Println("substituting", val, "with", new)
				}
			}
			// run cloned task
			subtr := runner.fn(subTask.Args)

			if subtr.Changed {
				tr.Changed = true
			}
			// merge tr with parent tr
			tr.Output += strings.TrimSpace(subtr.Output) + "\n"
			tr.Status = subtr.Status
			if subtr.Status == failed {
				// fmt.Println("failure, stopping")
				break
			}
		}
	} else {
		tr = runner.fn(task.Args)
	}

	tr.Duration = time.Since(start)
	tr.Task = task
	return tr
}
