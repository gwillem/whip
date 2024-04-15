package runners

import (
	"fmt"

	"github.com/gwillem/whip/internal/model"
	m "github.com/gwillem/whip/internal/model"
)

// var allowedServiceStates = []string{"started", "stopped", "restarted", "reloaded"}

var serviceStateMap = map[string]string{
	"started":   "start",
	"stopped":   "stop",
	"restarted": "restart",
	"reloaded":  "reload",
}

func Service(args m.TaskArgs, _ model.TaskVars) (tr m.TaskResult) {
	state := serviceStateMap[args.String("state")]
	if state == "" {
		return failure("unknown state, try started|stopped|restarted|reloaded")
	}
	cmd := []string{"/bin/bash", "-c", fmt.Sprintf("systemctl %s %s", state, args.String("name"))}
	return system(cmd)
}

func init() {
	registerRunner("service", Service, runnerMeta{
		requiredArgs: []string{"name", "state"},
		optionalArgs: []string{},
	})
}
