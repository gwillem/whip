package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gwillem/whip/internal/runners"
)

type (
	resultHandler interface {
		Send(runners.TaskResult)
		Quit()
	}
	tuiHandler struct {
		tui *tea.Program
	}
	verboseHandler struct{}
)

func (t tuiHandler) Send(r runners.TaskResult) {
	t.tui.Send(r)
}
func (t tuiHandler) Quit() {
	time.Sleep(100 * time.Millisecond) // TODO eliminate this
	t.tui.Quit()
	t.tui.Wait()
}

func (h verboseHandler) Send(r runners.TaskResult) {

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

	fmt.Printf("%s %s (%.2fs %s)\n", r.Host, taskSummary, r.Duration.Seconds(), status)
	// fmt.Printf("<%s>\n", r.Output)
	if len(r.Output) > 0 {
		for _, line := range strings.Split(strings.TrimSpace(r.Output), "\n") {
			fmt.Printf("  %s\n", dark(line))
		}
	}
	// fmt.Println(r.Output)

}
func (h verboseHandler) Quit() {}

func reportResults(results <-chan runners.TaskResult, verbosity int) {
	fmt.Println()

	var handler resultHandler = verboseHandler{}

	switch verbosity {
	case 0:
		handler = tuiHandler{createTui()}
	default:
		handler = verboseHandler{}
	}

	stats := map[string]map[string]int{}

	failed := []runners.TaskResult{}
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

	fmt.Println()
	for k, stats := range stats {
		fmt.Println(k, stats)
	}

	fmt.Println()
}
