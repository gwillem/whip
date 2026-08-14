package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/parser"
	"github.com/gwillem/whip/internal/runners"
)

func TestUseTUI(t *testing.T) {
	cases := []struct {
		name      string
		verbosity int
		isTTY     bool
		want      bool
	}{
		{"tty, no verbose -> tui", 0, true, true},
		{"no tty, no verbose -> fallback", 0, false, false},
		{"tty, verbose -> plain", 1, true, false},
		{"no tty, verbose -> plain", 1, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := useTUI(c.verbosity, c.isTTY); got != c.want {
				t.Errorf("useTUI(%d, %v) = %v, want %v", c.verbosity, c.isTTY, got, c.want)
			}
		})
	}
}

func TestFailureReport(t *testing.T) {
	cases := []struct {
		name string
		res  model.TaskResult
		want []string // substrings that must survive into the report
	}{
		{
			name: "named task shows name, command and output",
			res: model.TaskResult{
				Host:   "root@10.66.0.11",
				Task:   &model.Task{Name: "install magento", Runner: "shell", Args: model.TaskArgs{"_": "bin/magento setup:install"}},
				Output: "/bin/sh -c bin/magento setup:install\nexit status 1:\nAccess denied for user 'magento'",
			},
			want: []string{
				"root@10.66.0.11",
				"install magento",
				"shell",
				"bin/magento setup:install",
				"Access denied for user 'magento'",
			},
		},
		{
			// Most tasks are unnamed, so the arguments have to identify them.
			name: "unnamed task falls back to its arguments",
			res: model.TaskResult{
				Host:   "web1",
				Task:   &model.Task{Runner: "apt", Args: model.TaskArgs{"_": "nginx"}},
				Output: "E: Unable to locate package nginx",
			},
			want: []string{"web1", "unnamed", "apt", "nginx", "Unable to locate package"},
		},
		{
			// A runner may fail silently; the report must not be a blank line.
			name: "no output still reports the task",
			res: model.TaskResult{
				Host: "web1",
				Task: &model.Task{Name: "copy config", Runner: "copy"},
			},
			want: []string{"web1", "copy config", "no output"},
		},
		{
			// A deputy-level failure arrives without a task attached.
			name: "missing task does not panic",
			res:  model.TaskResult{Host: "web1", Output: "deputy died"},
			want: []string{"web1", "deputy died"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := failureReport(c.res)
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Errorf("report %q does not contain %q", got, w)
				}
			}
			if !strings.HasSuffix(got, "\n") {
				t.Errorf("report %q should end in a newline", got)
			}
		})
	}
}

// childEnv marks the re-executed copy of this test binary that plays the part
// of a whip run whose second task fails.
const childEnv = "WHIP_TEST_FAILING_RUN"

// TestFailureVisibleWithoutTerminal covers the two gaps that together made a
// failed deploy unreadable: the progress bar needed a terminal, and the failure
// was only reported at -v, so a redirected run produced a log with nothing in
// it. The child is a real reportResults over real (locally executed) task
// results, with stdout and stderr on a file, exactly as cron would run it.
func TestFailureVisibleWithoutTerminal(t *testing.T) {
	if os.Getenv(childEnv) == "1" {
		reportFailingRun()
		return
	}

	logPath := filepath.Join(t.TempDir(), "whip.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close() //nolint:errcheck

	cmd := exec.Command(os.Args[0], "-test.run=TestFailureVisibleWithoutTerminal")
	cmd.Env = append(os.Environ(), childEnv+"=1")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	runErr := cmd.Run()

	out, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	if runErr == nil {
		t.Errorf("a failed task must exit non-zero, got success; output:\n%s", got)
	}
	for _, want := range []string{"second task", "shell", "it broke", "exit status 3"} {
		if !strings.Contains(got, want) {
			t.Errorf("redirected output does not mention %q; output:\n%s", want, got)
		}
	}
}

// reportFailingRun feeds two genuinely executed tasks through the reporting
// path, the second of them failing, as the deputy's gob stream would.
func reportFailingRun() {
	// Same log level a default run has, or the assertions would pass on output
	// that production suppresses.
	setVerbosityLevel(0)

	tasks := []model.Task{
		{Name: "first task", Runner: "shell", Args: model.TaskArgs{parser.DefaultArg: "echo fine"}},
		{Name: "second task", Runner: "shell", Args: model.TaskArgs{parser.DefaultArg: "echo it broke >&2; exit 3"}},
	}

	results := make(chan model.TaskResult, len(tasks))
	for i := range tasks {
		tr := runners.Run(&tasks[i], nil)
		if tr.Task == nil {
			tr.Task = &tasks[i]
		}
		tr.Host = "stub"
		results <- tr
	}
	close(results)

	stats := map[model.TargetName]map[string]int{"stub": {"total": len(tasks)}}
	reportResults(results, stats, 0) // 0 = default verbosity, the broken case
}
