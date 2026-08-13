package main

import (
	"bytes"
	"embed"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/assets"
	"github.com/gwillem/whip/internal/fsutil"
	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/playbook"
	"github.com/gwillem/whip/internal/runners"
	"github.com/gwillem/whip/internal/ssh"
)

const (
	deputyPath          = ".cache/whip/deputy"
	defaultPlaybookPath = ".whip/playbook.yml"
)

//go:embed deputies
var deputies embed.FS

var buildVersion = "unknown"

func runWhip(playbookArg string, verbosity int, extraVars model.Vars) {
	whipStartTime := time.Now()
	log.Task("Starting whip", buildVersion)
	playbookPath := getPlaybookPath(playbookArg)

	// change working dir to playbook parent
	// this is where we will look for assets
	if err := os.Chdir(filepath.Dir(playbookPath)); err != nil {
		log.Fatal(err)
	}

	playbookPath = filepath.Base(playbookPath)
	pb, err := playbook.Load(playbookPath)
	if err != nil {
		log.Fatal(err)
	}

	// -e wins over vars_files and over the play's own vars, so it is merged
	// before anything reads them (prerun tasks included).
	applyExtraVars(pb, extraVars)

	// Render `hosts` with the play's variables, now that vars_files and -e
	// have been merged. Without this a playbook can be parameterised in every
	// respect except the one that decides which machine it runs against, so
	// pointing it at a different target means editing the file -- and a
	// playbook edited between runs is a playbook that eventually runs against
	// the wrong host.
	if err := renderHosts(pb); err != nil {
		log.Fatal(err)
	}

	log.Progress("Loaded playbook with", len(*pb), "plays")

	// validation... should happen at deputy, because controller doesn't have access
	// to facts and cannot parse dynamic tasks without them

	runPreRunTasks(pb)

	// Create jobbook to map plays to targets
	jobBook := createJobBook(pb)

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
		log.Fatal("SSH connection failed:", t, err)
	}
	defer conn.Close() //nolint:errcheck

	if err := ensureDeputy(conn); err != nil {
		log.Fatal("Failed to install deputy on", t, err)
	}
	results <- model.TaskResult{
		Host:     t,
		Task:     &model.Task{Runner: "connect"},
		Output:   "Loaded Deputy",
		Duration: time.Since(runStart),
	}

	// chain gob encoder and zstd compressor
	gobRd, gobWr := io.Pipe()
	go func() {
		if err := gob.NewEncoder(gobWr).Encode(job); err != nil {
			log.Fatal("gob encode err", err)
		}
		_ = gobWr.Close()
	}()

	zstdRd, zstdWr := io.Pipe()
	go func() {
		err := assets.Compress(gobRd, zstdWr)
		zstdWr.CloseWithError(err)
	}()

	cmd := "sudo $HOME/.cache/whip/deputy 2>$HOME/.cache/whip/whip.err"
	err = ssh.RunGobStreamer(conn, cmd, zstdRd, func(res model.TaskResult) {
		res.Host = t
		results <- res
	})
	if err != nil {
		// should show red ERROR but doesnot work.. concurrency issue?
		// results <- model.TaskResult{Status: runners.Failed, Host: t, Output: err.Error()}
		log.Fatal("Deputy error, see ~/.cache/whip/whip.err at", t, err)
	}
	if e := conn.Close(); e != nil {
		log.Error(e)
	}
	if e := zstdRd.Close(); e != nil {
		log.Error(e)
	}
	if e := gobRd.Close(); e != nil {
		log.Error(e)
	}
}

func runPreRunTasks(pb *model.Playbook) {
	log.Task("Running pre-run tasks on controller")
	for _, play := range *pb {
		for _, cmd := range play.PreRun {
			// Stream rather than capture: a prerun is a controller-side build
			// or a nested whip run that can take minutes, and capturing it made
			// whip look hung for its whole duration. One writer for both
			// streams keeps the interleaving that the shell would have shown
			// (os/exec shares the pipe when Stdout and Stderr are equal).
			out := &prefixWriter{w: os.Stdout, prefix: dark("prerun|") + " "}
			c := exec.Command("/bin/sh", "-c", cmd)
			c.Stdout = out
			c.Stderr = out
			err := c.Run()
			out.Flush()
			if err != nil {
				// The output is already on the terminal above, so the fatal
				// line only has to say which command produced it.
				log.Fatal(cmd+":", err)
			}
			log.Ok(cmd)
		}

		for _, task := range play.Tasks {
			tr := runners.PreRun(&task, play.Vars)
			if tr.Status == runners.Skipped {
				continue
			}
			log.Debug("Pre-run", task.Runner, "with status", tr.Status, tr.Output)
		}
	}
}

// prefixWriter prefixes every forwarded line, so controller-side output cannot
// be mistaken for output from a target.
type prefixWriter struct {
	w      io.Writer
	prefix string
	buf    []byte
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	p.buf = append(p.buf, b...)
	for {
		i := bytes.IndexByte(p.buf, '\n')
		if i < 0 {
			break
		}
		if _, err := fmt.Fprintf(p.w, "%s%s\n", p.prefix, p.buf[:i]); err != nil {
			return 0, err
		}
		p.buf = p.buf[i+1:]
	}
	return len(b), nil
}

// Flush emits a final line that was not newline terminated.
func (p *prefixWriter) Flush() {
	if len(p.buf) == 0 {
		return
	}
	fmt.Fprintf(p.w, "%s%s\n", p.prefix, p.buf)
	p.buf = nil
}

// parseExtraVars turns repeated -e key=value flags into a variable set.
func parseExtraVars(args []string) (model.Vars, error) {
	if len(args) == 0 {
		return nil, nil
	}
	vars := model.Vars{}
	for _, arg := range args {
		k, v, ok := strings.Cut(arg, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" {
			return nil, fmt.Errorf("extra var %q is not key=value", arg)
		}
		vars[k] = v
	}
	return vars, nil
}

// applyExtraVars merges command line variables into every play. vars_files have
// already been merged into Play.Vars by playbook.Load, and the play's own vars
// live in the same map, so overwriting it is the whole precedence rule: extra
// vars are the operator's last word, as in Ansible.
func applyExtraVars(pb *model.Playbook, extra model.Vars) {
	if len(extra) == 0 {
		return
	}
	for i := range *pb {
		play := &(*pb)[i]
		if play.Vars == nil {
			play.Vars = map[string]any{}
		}
		for k, v := range extra {
			play.Vars[k] = v
		}
	}
}

// Function to create jobBook from playbook
func createJobBook(pb *model.Playbook) map[model.TargetName]model.Job {
	jobBook := map[model.TargetName]model.Job{}
	for i, play := range *pb {
		log.Debug("Processing play", i, "with", len(play.Hosts), "hosts")
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
	return jobBook
}

func getPlaybookPath(arg string) string {
	playbookPath := arg
	if playbookPath == "" {
		playbookPath = fsutil.FindAncestorPath(defaultPlaybookPath)
	}

	if playbookPath == "" {
		log.Fatal("No playbook supplied and no playbook.yml found in current or parent directories")
	}
	return playbookPath
}

// renderHosts templates each play's target list against that play's variables.
func renderHosts(pb *model.Playbook) error {
	for i := range *pb {
		play := &(*pb)[i]
		for j, h := range play.Hosts {
			rendered, err := runners.RenderString(string(h), play.Vars)
			if err != nil {
				return fmt.Errorf("play %q: hosts: %w", play.Name, err)
			}
			if rendered == "" {
				return fmt.Errorf("play %q: hosts entry %q rendered empty", play.Name, h)
			}
			play.Hosts[j] = model.TargetName(rendered)
		}
	}
	return nil
}
