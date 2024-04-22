package main

import (
	"bytes"
	"embed"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gwillem/go-buildversion"
	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/playbook"
	"github.com/gwillem/whip/internal/runners"
	"github.com/gwillem/whip/internal/ssh"
	"github.com/spf13/cobra"
)

const (
	deputyPath       = ".cache/whip/deputy"
	defaultAssetPath = "files"
)

//go:embed deputies
var deputies embed.FS

func runWhip(cmd *cobra.Command, args []string) {
	whipStartTime := time.Now()

	verbosity, err := cmd.Flags().GetCount("verbose")
	if err != nil {
		log.Error(err)
	}

	// fmt.Println("verbosity level is", verbosity)
	log.SetLevel(log.LevelError)
	if verbosity > 0 {
		log.SetLevel(log.LevelTask)
	}

	if verbosity > 1 {
		log.SetLevel(log.LevelDebug)
	}
	if verbosity > 2 {
		log.SetPrefixer(&durationPrefixer{})
	}

	files, _ := deputies.ReadDir("deputies")
	log.Task("Starting whip", buildversion.String(), "with", len(files), "embedded deputies")

	pb, err := playbook.Load(args[0])
	if err != nil {
		log.Error(err)
		return
	}

	log.Progress("Loaded playbook with", len(*pb), "plays")

	// change working dir to playbook parent
	// this is where we will look for assets
	if err := os.Chdir(filepath.Dir(args[0])); err != nil {
		log.Fatal(err)
	}

	// load assets TODO move to prerun
	// assets, err := playbook.DirToAsset(defaultAssetPath)
	// if err != nil {
	// 	log.Warn(err)
	// }

	// validation... should happen at deputy, because controller doesn't have access
	// to facts and cannot parse dynamic tasks without them

	// prerun!
	log.Task("Running any pre-run tasks on controller")
	for _, play := range *pb {
		for _, task := range play.Tasks {
			tr := runners.PreRun(&task, play.Vars)
			if tr.Status == runners.Skipped {
				continue
			}
			log.Progress("pre-run", task.Runner, "with status", tr.Status, tr.Output)
		}
	}

	// load external vars?

	// Create jobbook to map plays to targets
	jobBook := map[model.TargetName]model.Job{}
	for i, play := range *pb {
		log.Progress("Processing play", i, "with", len(play.Hosts), "hosts")
		for _, target := range play.Hosts {

			if _, ok := jobBook[target]; !ok {
				jobBook[target] = model.Job{}
			}

			t := jobBook[target]
			// t.Assets = assets
			t.Playbook = append(t.Playbook, play)
			jobBook[target] = t
		}
	}

	stats := map[model.TargetName]map[string]int{}

	resultChan := make(chan model.TaskResult)
	wg := sync.WaitGroup{}

	for target, job := range jobBook {
		// need to save total tasks for progress meter later
		stats[target] = map[string]int{"total": len(job.Tasks()) + 2} // +1 for loading the deputy

		wg.Add(1)
		go func(job model.Job, h model.TargetName, r chan<- model.TaskResult) {
			defer wg.Done()
			runPlaybookAtHost(job, h, r)
		}(job, target, resultChan)
	}

	// kill result channel so reader knows when to stop
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	reportResults(resultChan, stats, verbosity)
	log.Ok(fmt.Sprintf("Finished whip in %.1fs", time.Since(whipStartTime).Seconds()))
}

func runPlaybookAtHost(job model.Job, t model.TargetName, results chan<- model.TaskResult) {
	runStart := time.Now()
	if len(job.Playbook) == 0 {
		log.Fatal("no plays to run at target", t)
	}
	log.Task("Running play at target:", t, "with", len(job.Playbook), "plays")

	// show that we are starting
	results <- model.TaskResult{
		Host:   t,
		Task:   &model.Task{Runner: ""},
		Output: "Starting",
	}

	conn, err := ssh.Connect(string(t))
	if err != nil {
		log.Error(err)
		return
	}
	defer conn.Close()

	if err := ensureDeputy(conn); err != nil {
		log.Error(err)
		return
	}
	results <- model.TaskResult{
		Host:     t,
		Task:     &model.Task{Runner: "loader"},
		Output:   "Loaded Deputy",
		Duration: time.Since(runStart),
	}

	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(job); err != nil {
		log.Fatal("gob encode err", err)
		return

	}
	cmd := "sudo $HOME/.cache/whip/deputy 2>$HOME/.cache/whip/deputy.err"
	err = ssh.RunGobStreamer(conn, cmd, &buffer, func(res model.TaskResult) {
		res.Host = t
		results <- res
	})
	if err != nil {
		log.Fatal("Deputy error, see ~/.cache/whip/deputy.err at", t, err)
		return
	}
}

type durationPrefixer struct {
	last time.Time
}

func (p *durationPrefixer) Prefix() string {
	var delta time.Duration
	if !p.last.IsZero() {
		delta = time.Since(p.last)
	}
	p.last = time.Now()
	return dark(fmt.Sprintf("%.3f", delta.Seconds()))
}
