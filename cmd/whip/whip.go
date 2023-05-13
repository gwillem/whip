package main

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gwillem/chief-whip/pkg/ssh"
	"github.com/gwillem/chief-whip/pkg/whip"
	"github.com/gwillem/go-buildversion"
	"github.com/spf13/cobra"
)

const (
	deputyPath = ".cache/chief-whip/deputy"
)

//go:embed deputies
var deputies embed.FS

func ensureDeputy(c *ssh.Client) error {
	uname, err := c.Run(`
			uname -sm; 
			mkdir -p ~/.cache/chief-whip 2>/dev/null
			touch ~/.cache/chief-whip/deputy 2>/dev/null;
			sha256sum ~/.cache/chief-whip/deputy 2>/dev/null | awk '{print $1}';
			`)
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSpace(uname), "\n")
	if len(lines) != 2 {
		return fmt.Errorf("unexpected output from uname: %s", uname)
	}

	osarg := strings.ToLower(lines[0])
	osarg = strings.ReplaceAll(osarg, " ", "-")
	osarg = strings.ReplaceAll(osarg, "aarch64", "arm64")

	remoteSha := strings.TrimSpace(lines[1])

	myDep, err := deputies.ReadFile("deputies/" + osarg)
	if err != nil {
		return fmt.Errorf("could not read deputy for %s: %s", osarg, err)
	}

	localSha := fmt.Sprintf("%x", sha256.Sum256(myDep))

	// log.Debugf("local/remote sha:\n\t%s\n\t%s", localSha, remoteSha)

	if localSha == remoteSha {
		// log.Debug("remote deputy seems to be fine")
		return nil
	}

	// log.Debug("uploading deputy for ", osarg)
	if err := c.UploadBytes(myDep, deputyPath, 0o755); err != nil {
		return fmt.Errorf("Could not upload deputy: %s", err)
	}

	return nil
}

func runPlayAtHost(p whip.Play, h whip.Host, results chan<- whip.TaskResult) {
	// log.Infof("Running play at target: %s", h)
	conn, err := ssh.Connect(string(h))
	if err != nil {
		log.Error(err)
		return
	}
	defer conn.Close()

	if err := ensureDeputy(conn); err != nil {
		log.Error(err)
		return
	}
	// log.Info("Sending job to target deputy...")

	job := whip.Job{
		Tasks: p.Tasks,
	}

	blob, err := job.ToJSON()
	if err != nil {
		log.Error(err)
		return
	}
	cmd := "PATH=~/.cache/chief-whip:$PATH deputy 2>>~/.cache/chief-whip/deputy.err"
	err = conn.RunLineStreamer(cmd, blob, func(b []byte) {
		// fmt.Println("got res frm deputy... ", string(b))
		var res whip.TaskResult
		if err := json.Unmarshal(b, &res); err != nil {
			log.Error(err)
			return
		}
		res.Host = h
		results <- res
		// fmt.Println(res)
	})
	if err != nil {
		log.Error(fmt.Errorf("could not run deputy: %s", err))
		return
	}
}

func runWhip(cmd *cobra.Command, args []string) {
	log.SetLevel(log.DebugLevel)

	files, _ := deputies.ReadDir("deputies")
	log.Infof("Starting chief-whip %s with %d embedded deputies", buildversion.String(), len(files))

	playbook, err := whip.LoadPlaybook(args[0])
	if err != nil {
		log.Error(err)
		return
	}

	// TODO merge inventory with playbook if any
	// TODO convert playbook to map of targets -> jobs, possibly combining plays (vars?)

	resultChan := make(chan whip.TaskResult)
	wg := sync.WaitGroup{}

	totalTasks := 0

	for i1, play := range playbook {
		log.Infof("Running play %d with %d tasks", i1, len(play.Tasks))
		// fmt.Println(play.Hosts)
		totalTasks += len(play.Tasks) * len(play.Hosts)
		for _, target := range play.Hosts {
			wg.Add(1)
			go func(p whip.Play, h whip.Host, r chan<- whip.TaskResult) {
				defer wg.Done()
				// fmt.Println("sleeping")
				// time.Sleep(5 * time.Second)
				runPlayAtHost(p, h, r)
			}(play, target, resultChan)
		}
	}

	// kill result channel so reader knows when to stop
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	parseResults(resultChan)
	// now unblock resultchan
}

func parseResults(results <-chan whip.TaskResult) {
	fmt.Println()
	tui := createTui()
	failed := []whip.TaskResult{}
	for res := range results {
		tui.Send(res)
		if res.Status != 0 {
			failed = append(failed, res)
		}
	}
	// _ = tui.ReleaseTerminal()
	time.Sleep(100 * time.Millisecond)
	tui.Quit()
	tui.Wait()

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
}