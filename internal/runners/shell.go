package runners

import (
	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/parser"
)

func shell(t *model.Task) (tr model.TaskResult) {
	cmd := []string{"/bin/sh", "-c", t.Args.String(parser.DefaultArg)}
	return system(cmd)
}

func init() {
	registerRunner("shell", runner{run: shell})
}

func runShell(cmd string) (tr model.TaskResult) {
	return system([]string{"/bin/bash", "-c", cmd})
}
