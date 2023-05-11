package runners

import (
	"fmt"
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
		Output:  output}
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
		items, ok := task.Items.([]any)
		if !ok {
			return whip.TaskResult{
				Status: failed,
				Output: fmt.Sprintf("with_items must be a list of any, it is %T", task.Items),
				Task:   task}
		}

		for _, rawItem := range items {
			item, ok := rawItem.(string)
			if !ok {
				return whip.TaskResult{
					Status: failed,
					Output: "with_items must be a list of strings",
					Task:   task}
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
