package main

import (
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
	cmd := "PATH=~/.cache/chief-whip:$PATH deputy 2>/tmp/deputy.err"
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

	playbook := whip.LoadPlaybook(args[0])

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
