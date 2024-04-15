package runners

import (
	"github.com/google/shlex"
	"github.com/gwillem/whip/internal/model"
)

func Command(args model.TaskArgs, _ model.TaskVars) (tr model.TaskResult) {
	tokens, err := shlex.Split(args.String(defaultArg))
	if err != nil {
		tr.Status = failed
		tr.Output = err.Error()
		return tr
	}
	return system(tokens)
}

func init() {
	registerRunner("command", Command, runnerMeta{})
}
