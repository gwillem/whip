package runners

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gwillem/whip/internal/model"
	"github.com/ieee0824/go-deepmerge"
	"github.com/spf13/afero"
)

const (
	success int = iota
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
	runnerFunc func(model.TaskArgs) model.TaskResult
	runnerMeta struct {
		requiredArgs []string
		optionalArgs []string
	}
)

func failure(msg ...any) model.TaskResult {
	output := ""
	for _, m := range msg {
		if _, ok := m.(error); ok {
			output += ":"
		}
		output += " "
		output += fmt.Sprintf("%v", m)
	}
	output = strings.TrimSpace(output)

	return model.TaskResult{
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
func Run(task model.Task, vars map[string]any, afs afero.Fs) (tr model.TaskResult) {
	start := time.Now()
	fail := func(msg string) model.TaskResult {
		return model.TaskResult{
			Status: failed,
			Output: msg,
			Task:   task,
		}
	}

	// defer func() {
	// 	if r := recover(); r != nil {
	// 		fail("skdjfjksdjf")
	// 	}
	// }()

	// fmt.Println("Running", task.Type)
	runner, ok := runners[task.Runner]
	if !ok {
		return fail("No runner found for task '" + task.Runner + "'") // todo, is empty for unknown runners
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

	if afs != nil {
		task.Args["_assets"] = afs
	}

	tr = runner.fn(task.Args)
	tr.Duration = time.Since(start)
	tr.Task = task
	return tr
}
