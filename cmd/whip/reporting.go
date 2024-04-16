package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/model"
)

type (
	resultHandler interface {
		Send(model.TaskResult)
		Quit()
	}
	tuiHandler struct {
		tui *tea.Program
	}
	verboseHandler struct{}
)

func (t tuiHandler) Send(r model.TaskResult) {
	t.tui.Send(r)
}

func (t tuiHandler) Quit() {
	time.Sleep(100 * time.Millisecond) // TODO eliminate this
	t.tui.Quit()
	t.tui.Wait()
}

func (h verboseHandler) Send(r model.TaskResult) {
	statusColor := green
	status := "ok"

	switch {
	case r.Changed && r.Status == 0:
		statusColor = yellow
		status = "changed"
	case r.Status != 0:
		statusColor = red
		status = "error"
	}

	// runner := fmt.Sprintf("%-14.14s", r.Task.Runner)
	runner := r.Task.Runner
	taskSummary := fmt.Sprintf("%s %s", statusColor(runner), r.Task.Args)

	log.Progress(fmt.Sprintf("%s %s (%.2fs %s)", r.Host, taskSummary, r.Duration.Seconds(), status))
	// fmt.Printf("<%s>\n", r.Output)
	if len(r.Output) > 0 {
		for _, line := range strings.Split(strings.TrimSpace(r.Output), "\n") {
			log.Debug(dark(line))
		}
	}
	// fmt.Println(r.Output)
}
func (h verboseHandler) Quit() {}

func reportResults(results <-chan model.TaskResult, verbosity int) {
	// fmt.Println()

	var handler resultHandler

	switch verbosity {
	case 0:
		handler = tuiHandler{createTui()}
	default:
		handler = verboseHandler{}
	}

	stats := map[string]map[string]int{}
	failed := []model.TaskResult{}
	for res := range results {
		if stats[res.Host] == nil {
			stats[res.Host] = map[string]int{}
		}

		switch {
		case res.Changed && res.Status == 0:
			stats[res.Host]["changed"]++
		case res.Status != 0:
			stats[res.Host]["error"]++
		default:
			stats[res.Host]["ok"]++
		}

		handler.Send(res)
		if res.Status != 0 {
			failed = append(failed, res)
		}
	}
	handler.Quit()

	if len(failed) > 0 {
		fmt.Println()
		for _, f := range failed {
			fmt.Println(f)

			for _, line := range strings.Split(strings.TrimSpace(f.Output), "\n") {
				fmt.Println("  " + red(line))
			}
		}
	}

	if verbosity > 0 {
		for k, stats := range stats {
			log.Ok(fmt.Sprint(k, " ", stats))
		}
	}
	if len(failed) > 0 {
		os.Exit(1)
	}
}
