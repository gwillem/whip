package runners

import (
	"fmt"
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
	runnerFunc    func(*model.Task) model.TaskResult
	validatorFunc func(model.TaskArgs) error
	preRunnerFunc func(*model.Task) model.TaskResult

	runnerMeta struct {
		requiredArgs []string
		optionalArgs []string
	}

	runner struct {
		run      runnerFunc
		meta     runnerMeta
		prerun   runnerFunc
		validate validatorFunc
	}
)

var (
	fs      afero.Fs
	fsutil  *afero.Afero
	runners = map[string]runner{}
	facts   = gatherFacts()
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
		return
	}
	task.Vars = mergedVars.(map[string]any)

	// todo: merge vars
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

	// arg substitution, notably for loop {{item}}
	for k, v := range task.Args {
		if val, ok := v.(string); ok {
			parsed, err := tplParseString(val, task.Vars)
			if err != nil {
				return fail(err.Error())
			}
			task.Args[k] = parsed
		}
	}

	tr = runner.run(task)
	tr.Duration = time.Since(start)
	tr.Task = task
	return tr
}
