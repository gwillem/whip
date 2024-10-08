package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/runners"
)

type (
	resultHandler interface {
		Send(model.ReportMsg)
		Quit()
	}
	tuiHandler struct {
		tui *tea.Program
	}
	verboseHandler struct{}
)

func (t tuiHandler) Send(r model.ReportMsg) {
	t.tui.Send(r)
}

func (t tuiHandler) Quit() {
	time.Sleep(100 * time.Millisecond) // TODO eliminate this
	t.tui.Quit()
	t.tui.Wait()
}

func (h verboseHandler) Send(m model.ReportMsg) {
	statusColor := green
	status := "ok"

	tr := m.TaskResult

	switch {
	case tr.Changed && tr.Status == runners.Success:
		statusColor = yellow
		status = "changed"
	case tr.Status == runners.Failed:
		statusColor = red
		status = "error"
	case tr.Status == runners.Skipped:
		statusColor = dark
		status = "skipped"

	}

	// runner := fmt.Sprintf("%-14.14s", r.Task.Runner)
	runner := tr.Task.Runner
	if runner == "" {
		return
	}
	args := tr.Task.Args.ToString()
	trimmedArgs := trimDotDot(args, 60-len(runner))
	taskSummary := fmt.Sprintf("%s %s", statusColor(runner), trimmedArgs)

	log.Progress(fmt.Sprintf("%s %s (%.1fs %s)", tr.Host, taskSummary, tr.Duration.Seconds(), status))
	// fmt.Printf("<%s>\n", r.Output)
	if len(tr.Output) > 0 {
		for _, line := range strings.Split(strings.TrimSpace(tr.Output), "\n") {
			log.Debug(dark(line))
		}
	}
	// fmt.Println(r.Output)
}
func (h verboseHandler) Quit() {}

func trimDotDot(s string, lim int) string {
	if len(s) > lim {
		return s[:lim-3] + "..."
	}
	return s + strings.Repeat(" ", lim-len(s))
}

func reportResults(results <-chan model.TaskResult, stats map[model.TargetName]map[string]int, verbosity int) {
	var handler resultHandler = verboseHandler{}
	if verbosity == 0 {
		handler = tuiHandler{createTui()}
	}

	failed := []model.TaskResult{}
	for res := range results {
		if stats[res.Host] == nil {
			panic(fmt.Sprintf("no stats for %s, should not happen", res.Host))
		}

		stats[res.Host]["idx"]++

		switch {
		case res.Changed && res.Status == runners.Success:
			stats[res.Host]["changed"]++
		case res.Status == runners.Failed:
			stats[res.Host]["error"]++
		case res.Status == runners.Skipped:
			stats[res.Host]["skipped"]++
		default:
			stats[res.Host]["ok"]++
		}

		handler.Send(model.ReportMsg{
			TaskIdx:    stats[res.Host]["idx"],
			TaskTotal:  stats[res.Host]["total"],
			TaskResult: res,
		})
		if res.Status == runners.Failed {
			failed = append(failed, res)
		}
	}
	handler.Quit()

	if len(failed) > 0 {
		log.Task("Failed tasks")
		for _, f := range failed {
			for _, line := range strings.Split(strings.TrimSpace(f.Output), "\n") {
				log.Progress(fmt.Sprintf("%s %s", f.Host, red(line)))
			}
		}
	}

	if verbosity > 0 {
		log.Task("Summary")
		for k, stats := range stats {
			log.Ok(fmt.Sprint(k, " ", stats))
		}
	}
	if len(failed) > 0 {
		os.Exit(1)
	}
}
