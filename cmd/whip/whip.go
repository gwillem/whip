package main

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/gwillem/go-buildversion"
	"github.com/gwillem/whip/internal/loader"
	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/runners"
	"github.com/gwillem/whip/internal/ssh"
	"github.com/spf13/cobra"
)

const (
	deputyPath = ".cache/whip/deputy"
)

//go:embed deputies
var deputies embed.FS

func ensureDeputy(c *ssh.Client) error {
	uname, err := c.Run(`
			uname -sm; 
			mkdir -p ~/.cache/whip 2>/dev/null
			touch ~/.cache/whip/deputy 2>/dev/null;
			sha256sum ~/.cache/whip/deputy 2>/dev/null | awk '{print $1}';
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

func runPlaybookAtHost(pb model.Playbook, h model.Host, results chan<- runners.TaskResult) {
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

	// TODO add vars and assets
	job := model.Job{Playbook: pb}

	blob, err := job.ToJSON()
	if err != nil {
		log.Error(err)
		return
	}
	cmd := "PATH=~/.cache/whip:$PATH deputy 2>>~/.cache/whip/deputy.err"
	err = conn.RunLineStreamer(cmd, blob, func(b []byte) {
		// fmt.Println("got res frm deputy... ", string(b))
		var res runners.TaskResult
		if err := json.Unmarshal(b, &res); err != nil {
			log.Error(err)
			return
		}
		res.Host = string(h)
		results <- res
		// fmt.Println(res)
	})
	if err != nil {
		log.Error(fmt.Errorf("could not run deputy: %s", err))
		return
	}
}

func runWhip(cmd *cobra.Command, args []string) {

	verbosity, err := cmd.Flags().GetCount("verbose")
	if err != nil {
		log.Error(err)
	}

	fmt.Println("verbosity level is", verbosity)

	log.SetLevel(log.DebugLevel)

	files, _ := deputies.ReadDir("deputies")
	log.Infof("Starting whip %s with %d embedded deputies", buildversion.String(), len(files))

	playbook, err := loader.LoadPlaybook(args[0])
	if err != nil {
		log.Error(err)
		return
	}

	// TODO merge inventory with playbook if any

	// Create jobbook to map plays to hosts
	jobBook := map[model.Host]model.Playbook{}
	for _, play := range *playbook {
		for _, target := range play.Hosts {
			jobBook[target] = append(jobBook[target], play)
		}
	}

	resultChan := make(chan runners.TaskResult)
	wg := sync.WaitGroup{}

	for target, pb := range jobBook {
		wg.Add(1)
		go func(pb model.Playbook, h model.Host, r chan<- runners.TaskResult) {
			defer wg.Done()
			runPlaybookAtHost(pb, h, r)
		}(pb, target, resultChan)
	}

	// kill result channel so reader knows when to stop
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	reportResults(resultChan, verbosity)
}
