package runners

import (
	"github.com/gwillem/whip/internal/model"
)

func shell(t *model.Task) (tr model.TaskResult) {
	cmd := []string{"/bin/bash", "-c", t.Args.String(defaultArg)}
	return system(cmd)
}

func init() {
	registerRunner("shell", runner{run: shell})
}
