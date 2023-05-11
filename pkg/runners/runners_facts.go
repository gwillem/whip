package runners

import (
	"fmt"
	"os"
	"runtime"
)

func gatherFacts() map[string]string {
	facts := map[string]string{}
	facts["hostname"], _ = os.Hostname()
	facts["user"] = os.Getenv("USER")
	facts["num_cpu"] = fmt.Sprint(runtime.NumCPU())
	return facts
}
