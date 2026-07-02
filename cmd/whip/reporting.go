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
	"golang.org/x/term"
)

// useTUI reports whether the interactive bubbletea progress bar should be used.
// The TUI needs a controlling terminal; without one (CI, cron, piped output)
// bubbletea fails to open /dev/tty, so we fall back to the plain verbose handler.
func useTUI(verbosity int, isTTY bool) bool {
	return verbosity == 0 && isTTY
}

// isInteractive reports whether stdout is attached to a terminal.
func isInteractive() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

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
		for line := range strings.SplitSeq(strings.TrimSpace(tr.Output), "\n") {
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
	if useTUI(verbosity, isInteractive()) {
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
		fmt.Fprintln(os.Stderr, red("Failed tasks:"))
		for _, f := range failed {
			for line := range strings.SplitSeq(strings.TrimSpace(f.Output), "\n") {
				fmt.Fprintf(os.Stderr, "  %s %s\n", f.Host, red(line))
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
