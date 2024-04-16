package main

import (
	"encoding/gob"
	"os"

	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/runners"
)

func main() {
	job := getJobFromStdin()
	taskTotal := len(job.Tasks())
	taskIdx := 0

	// assetFs, err := playbook.AssetToFS(job.Assets)
	// if err != nil {
	// 	panic(err)
	// }

	encoder := gob.NewEncoder(os.Stdout)

	for playIdx, play := range job.Playbook {

		handlers := map[string]bool{}

		for _, task := range play.Tasks {
			taskIdx++
			tr := runners.Run(&task, play.Vars)
			tr.Task = &task
			tr.PlayIdx = playIdx
			tr.TaskIdx = taskIdx
			tr.TaskTotal = taskTotal

			// log.Debug("task result here", res.Task.Args)

			// don't echo back all the files..
			delete(tr.Task.Args, "_assets")

			if err := encoder.Encode(tr); err != nil {
				panic(err)
			}

			if tr.Status != 0 {
				break
			}

			if tr.Changed {
				for _, handler := range task.Notify {
					handlers[handler] = true
				}
			}

		}
		for _, handler := range play.Handlers {
			taskIdx++

			// empty tr in case of unnotified handler
			tr := model.TaskResult{}

			if handlers[handler.Name] {
				// log.Debug("Running handler", handler)
				tr = runners.Run(&handler, play.Vars)
			}
			tr.Task = &handler
			tr.PlayIdx = playIdx
			tr.TaskIdx = taskIdx
			tr.TaskTotal = taskTotal
			tr.Task.Runner = "handler:" + handler.Runner // todo fixme
			delete(tr.Task.Args, "_assets")
			if err := encoder.Encode(tr); err != nil {
				panic(err)
			}
			if tr.Status != 0 {
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
