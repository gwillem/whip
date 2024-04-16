package runners

import (
	"github.com/gwillem/whip/internal/model"
	m "github.com/gwillem/whip/internal/model"
)

func Shell(t *model.Task) (tr m.TaskResult) {
	cmd := []string{"/bin/bash", "-c", t.Args.String(defaultArg)}
	return system(cmd)
}

func init() {
	registerRunner("shell", runner{run: Shell})
}
