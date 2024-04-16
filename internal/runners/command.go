package runners

import (
	"github.com/google/shlex"
	"github.com/gwillem/whip/internal/model"
)

func Command(t *model.Task) (tr model.TaskResult) {
	tokens, err := shlex.Split(t.Args.String(defaultArg))
	if err != nil {
		tr.Status = Failed
		tr.Output = err.Error()
		return tr
	}
	return system(tokens)
}

func init() {
	registerRunner("command", runner{run: Command})
}
