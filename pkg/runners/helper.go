package runners

import (
	"fmt"
	"time"

	"github.com/gwillem/chief-whip/pkg/whip"
)

var (
	runners = map[string]RunnerFunc{}
)

type (
	RunnerFunc func(whip.TaskArgs) whip.TaskResult
)

func registerRunner(name string, fn RunnerFunc) {
	runners[name] = fn
}

func Task(task whip.Task) whip.TaskResult {
	fn, ok := runners[task.Type]
	if !ok {
		return whip.TaskResult{Status: 1, Output: fmt.Sprintf("No runner found for task '%s'", task.Type)}
	}
	start := time.Now()
	res := fn(task.Args)
	res.Duration = time.Since(start)
	return res
}
