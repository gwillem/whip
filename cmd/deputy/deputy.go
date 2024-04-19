package main

import (
	"encoding/gob"
	"os"

	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/runners"
)

func main() {
	job := getJobFromStdin()

	// send task results back to whip
	encoder := gob.NewEncoder(os.Stdout)

	for _, play := range job.Playbook {
		handlers := map[string]bool{}
		for _, task := range play.Tasks {
			tr := runners.Run(&task, play.Vars)
			tr.Task = &task

			// don't echo back all the files..
			delete(tr.Task.Args, "_assets")

			if err := encoder.Encode(tr); err != nil {
				panic(err)
			}

			if tr.Status != 0 {
				break
			}

			if tr.Changed {
				for _, h := range task.Notify {
					handlers[h] = true
				}
				// individual notifies, for example for the tree runner
				for h := range tr.Notify {
					handlers[h] = true
				}
			}

		}
		// if len(handlers) == 0 {
		// 	log.Debug("No handlers were notified", handlers)
		// }
		for _, handler := range play.Handlers {
			// empty tr in case of unnotified handler
			tr := model.TaskResult{Status: runners.Skipped}

			if handlers[handler.Name] {
				// log.Debug("Running handler", handler)
				tr = runners.Run(&handler, play.Vars)
			}
			tr.Task = &handler
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
