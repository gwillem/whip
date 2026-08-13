package runners

import (
	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/parser"
)

func shell(t *model.Task) (tr model.TaskResult) {
	cmd := []string{"/bin/sh", "-c", t.Args.String(parser.DefaultArg)}
	// systemTask, not system: it applies the task's changed_when, without
	// which every shell task reports "changed" on every deploy and the run
	// summary says nothing.
	return systemTask(t, cmd)
}

func init() {
	// stringArg: a shell command is a command, not a bag of key=value pairs.
	registerRunner("shell", runner{
		run:  shell,
		meta: runnerMeta{stringArg: parser.DefaultArg},
	})
}

func runShell(cmd string) (tr model.TaskResult) {
	return system([]string{"/bin/bash", "-c", cmd})
}
