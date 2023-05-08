package main

import (
	"fmt"

	"github.com/gwillem/chief-whip/pkg/whip"
)

func main() {
	/*

		1. Collect inventory
		2. Construct Job
			1. Collect tasks
			2. Collect assets
			3. Collect vars
		3. Iterate over inventory, for each:
			1. Ensure chief-whip-local present
				1. Run local bash script
				2. Upload chief-whip-local
			2. SSH to target, serialize Job on its stdin
			3. Read status reports (1 json obj per task)

	*/

	myInv := []string{"localhost"}

	// job := internal.Job{
	// 	Tasks: []internal.Task{
	// 		{
	// 			Name: "echo",
	// 			Args: []string{"hello world"},
	// 		},
	// 	},
	// }

	for _, target := range myInv {
		// Ensure chief-whip-local present
		// SSH to target, serialize Job on its stdin
		// Read status reports (1 json obj per task)
		client, err := whip.DialSSH(target)
		if err != nil {
			panic(err)
		}
		defer client.Close()

		session, err := client.NewSession()
		if err != nil {
			panic(err)
		}
		defer session.Close()

		session.StdinPipe()

		cmd := "hostname && date && whoami"
		output, err := session.CombinedOutput(cmd)
		fmt.Println(target, string(output), err)
		output, err = session.CombinedOutput("date")
		fmt.Println(string(output), err)
	}

}
