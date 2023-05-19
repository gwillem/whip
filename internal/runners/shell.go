package runners

import m "github.com/gwillem/whip/internal/model"

func Shell(args m.TaskArgs) (tr m.TaskResult) {
	cmd := []string{"/bin/bash", "-c", args.String(defaultArg)}
	return system(cmd)
}

func init() {
	registerRunner("shell", Shell, runnerMeta{})
}
