package runners

import (
	"fmt"
	"os/exec"

	"github.com/gwillem/chief-whip/pkg/whip"
)

func Command(args whip.TaskArgs) whip.TaskResult {
	tr := whip.TaskResult{}

	cmd := args["args"]

	fmt.Println("Running command:", cmd)

	data, err := exec.Command(args["args"]).CombinedOutput()
	tr.Changed = true
	if err == nil {
		tr.Status = 0
		tr.Output = string(data)
	} else {
		tr.Status = 1
		tr.Output = err.Error()
	}
	return tr
}

func init() {
	registerRunner("command", Command)
}
