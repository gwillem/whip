package runners

import (
	"os/exec"

	"github.com/gwillem/chief-whip/pkg/whip"
)

func Shell(args whip.TaskArgs) (tr whip.TaskResult) {
	cmd := []string{"/bin/bash", "-c", args.Key(defaultArg)}

	data, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
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
	registerRunner("shell", Shell, runnerMeta{})
}
