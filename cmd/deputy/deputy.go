package main

import (
	"encoding/gob"
	"os"

	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/playbook"
	"github.com/gwillem/whip/internal/runners"
)

func main() {
	job := getJobFromStdin()
	taskTotal := len(job.Tasks())
	taskIdx := 0

	assetFs, err := playbook.AssetToFS(job.Assets)
	if err != nil {
		panic(err)
	}

	encoder := gob.NewEncoder(os.Stdout)

	for playIdx, play := range job.Playbook {
		for _, task := range play.Tasks {
			taskIdx++
			res := runners.Run(task, play.Vars, assetFs)
			res.PlayIdx = playIdx
			res.TaskIdx = taskIdx
			res.TaskTotal = taskTotal

			// log.Debug("task result here", res.Task.Args)

			// don't echo back all the files..
			delete(res.Task.Args, "_assets")

			if err := encoder.Encode(res); err != nil {
				panic(err)
			}

			if res.Status != 0 {
				break
			}
		}
	}
}

func getJobFromStdin() *model.Job {
	decoder := gob.NewDecoder(os.Stdin)
	job := &model.Job{}
	if err := decoder.Decode(job); err != nil {
		panic(err)
	}
	return job
}
