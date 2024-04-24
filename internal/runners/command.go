package runners

import (
	"os/exec"

	"github.com/google/shlex"
	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/parser"
)

func Command(t *model.Task) (tr model.TaskResult) {
	log.Debug("command args:", t.Args)
	tokens, err := shlex.Split(t.Args.String(parser.DefaultArg))
	if err != nil {
		tr.Status = Failed
		tr.Output = err.Error()
		return tr
	}
	return runCommand(tokens, t.Args.String("unless"))
}

func init() {
	registerRunner("command", runner{run: Command})
}

func runCommand(cmd []string, unlessCmd string) (tr model.TaskResult) {
	if unlessCmd != "" {
		if _, err := exec.Command("/bin/sh", "-c", unlessCmd).CombinedOutput(); err == nil {
			return model.TaskResult{Status: Success}
		}
	}

	return system(cmd)
}
