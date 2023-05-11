package runners

import (
	"fmt"
	"strings"
	"time"

	"github.com/gwillem/chief-whip/pkg/whip"
)

const (
	ok int = iota
	failed

	defaultArg = "_args"
)

var (
	runners = map[string]struct {
		fn   runnerFunc
		meta runnerMeta
	}{}
)

type (
	runnerFunc func(whip.TaskArgs) whip.TaskResult
	runnerMeta struct {
		requiredArgs []string
		optionalArgs []string
		wantItems    bool
	}
)

func registerRunner(name string, fn runnerFunc, meta runnerMeta) {
	runners[name] = struct {
		fn   runnerFunc
		meta runnerMeta
	}{fn, meta}
}

func Run(task whip.Task) (tr whip.TaskResult) {
	// fmt.Println("Running", task.Type)
	runner, ok := runners[task.Type]
	if !ok {
		return whip.TaskResult{
			Status: failed,
			Output: fmt.Sprintf("No runner found for task '%s'", task.Type),
			Task:   task}
	}
	start := time.Now()

	// with_items?
	if task.Items != nil && !runner.meta.wantItems {
		for _, rawItem := range task.Items.([]any) {
			// fmt.Println("Running with item", item)
			// clone task, interpolate all args
			// todo should use tpl engine

			item, ok := rawItem.(string)
			if !ok {
				continue
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
