package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/gwillem/chief-whip/internal"
)

// apply Job to localhost

func main() {

	job := getJobFromStdin()
	for i, task := range job.Tasks {
		res := internal.JobResult{
			Changed: true,
			Output:  fmt.Sprintf("Task %d. %s completed", i, task.Name),
			Status:  0}
		blob, err := json.Marshal(res)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(blob))
		// fmt.Println("task", i, task.Name)
	}

	os.Stderr.WriteString(fmt.Sprintln(job))
	/*

		Pretty simple?!

		1. Take Job from stdin
		2. Iterate over tasks
			- Test if change is required
			- Apply change
			- Report status on stdout

		For each task:
		- process templates
		- substitute vars in task arguments

	*/

}

func getJobFromStdin() *internal.Job {
	reader := bufio.NewReader(os.Stdin)
	blob, err := io.ReadAll(reader)
	if err != nil {
		panic(err)
	}
	// println("got blob with length", len(blob))
	job := &internal.Job{}
	if e := json.Unmarshal(blob, job); e != nil {
		panic(e)
	}
	return job
}
