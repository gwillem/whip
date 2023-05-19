package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/gwillem/whip/internal/runners"
	"github.com/gwillem/whip/internal/whip"
)

func main() {
	job := getJobFromStdin()
	taskTotal := len(job.Tasks())
	taskIdx := 0
	for playIdx, play := range job.Playbook {
		for _, task := range play.Tasks {
			taskIdx++
			res := runners.Run(task, play.Vars)
			res.PlayIdx = playIdx
			res.TaskIdx = taskIdx
			res.TaskTotal = taskTotal
			blob, err := json.Marshal(res)
			if err != nil {
				panic(err)
			}
			fmt.Println(string(blob))

			if res.Status != 0 {
				break
			}
		}
	}
}

func getJobFromStdin() *whip.Job {
	reader := bufio.NewReader(os.Stdin)
	blob, err := io.ReadAll(reader)
	if err != nil {
		panic(err)
	}
	// println("got blob with length", len(blob))
	job := &whip.Job{}
	if e := json.Unmarshal(blob, job); e != nil {
		panic(e)
	}
	return job
}
