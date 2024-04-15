package runners

import (
	"github.com/gwillem/whip/internal/model"
	m "github.com/gwillem/whip/internal/model"
)

func Shell(args m.TaskArgs, _ model.TaskVars) (tr m.TaskResult) {
	cmd := []string{"/bin/bash", "-c", args.String(defaultArg)}
	return system(cmd)
}

func init() {
	registerRunner("shell", Shell, runnerMeta{})
}
