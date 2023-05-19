package runners

import (
	"github.com/google/shlex"
)

func Command(args TaskArgs) (tr TaskResult) {
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
