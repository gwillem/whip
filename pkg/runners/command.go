package runners

import (
	"os/exec"

	"github.com/google/shlex"
)

func Command(args TaskArgs) (tr TaskResult) {
	tokens, err := shlex.Split(args.Key(defaultArg))
	if err != nil {
		tr.Status = failed
		tr.Output = err.Error()
		return tr
	}

	data, err := exec.Command(tokens[0], tokens[1:]...).CombinedOutput()
	tr.Changed = true
	if err == nil {
		tr.Status = ok
		tr.Output = string(data)
	} else {
		tr.Status = failed
		tr.Output = err.Error()
	}
	return tr
}

func init() {
	registerRunner("command", Command, runnerMeta{})
}
