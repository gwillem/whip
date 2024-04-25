package main

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"time"

	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/assets"
	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/runners"
)

func main() {
	start := time.Now()
	log.Task("Running deputy at", time.Now().UTC().Format(time.RFC3339))
	runJob(getJobFromStdin())
	log.Ok("Finished deputy (" + time.Since(start).Round(time.Millisecond).String() + ")")
}

func runJob(job *model.Job) {
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

			if tr.Status == runners.Failed {
				return
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
		for _, handler := range play.Handlers {
			// empty tr in case of unnotified handler
			tr := model.TaskResult{Status: runners.Skipped}

			if handlers[handler.Name] {
				// log.Debug("Running handler", handler)
				tr = runners.Run(&handler, play.Vars)
			}
			tr.Task = &handler
			if tr.Task.Runner != "" {
				tr.Task.Runner = "handler:" + handler.Runner // todo fixme
			}
			delete(tr.Task.Args, "_assets")
			if err := encoder.Encode(tr); err != nil {
				panic(err)
			}
			if tr.Status == runners.Failed {
				return
			}
		}
	}
}

func getJobFromStdin() *model.Job {
	stdinReader := assets.NewReadCounter(os.Stdin)
	pr, pw := io.Pipe()
	go func() {
		if e := assets.Decompress(stdinReader, pw); e != nil { // was: os.Stdin
			log.Errorf("error decompressing: %w", e)
		}
		pw.Close()
	}()

	decompressedReader := assets.NewReadCounter(pr)

	decoder := gob.NewDecoder(decompressedReader) // was: pr
	job := &model.Job{}
	if err := decoder.Decode(job); err != nil {
		log.Errorf("gob decode: %w", err)
	}

	ratio := fmt.Sprintf("%.0f%%", 100*float64(stdinReader.Count())/float64(decompressedReader.Count()))
	log.Debug("Compesssed size/ratio:", stdinReader.Count(), ratio)

	return job
}
