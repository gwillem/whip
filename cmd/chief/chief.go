package main

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"strings"

	_ "embed"

	"github.com/charmbracelet/log"
	"github.com/gwillem/chief-whip/pkg/ssh"
)

const (
	deputyPath = ".cache/chief-whip/deputy"
)

//go:embed deputies
var deputies embed.FS

func main() {
	/*

		1. Collect inventory
		2. Construct Job
			1. Collect tasks
			2. Collect assets
			3. Collect vars
		3. Iterate over inventory, for each:
			1. Ensure chief-whip-local present
				1. Run local bash script
				2. Upload chief-whip-local
			2. SSH to target, serialize Job on its stdin
			3. Read status reports (1 json obj per task)

	*/

	if e := rootCmd.Execute(); e != nil {
		log.Fatal(e)
	}

}

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
	if err := c.UploadBytes(myDep, deputyPath, 0755); err != nil {
		return fmt.Errorf("Could not upload deputy: %s", err)
	}

	return nil
}

// func getInventory() []string {
// 	return []string{"ubuntu@192.168.64.10"}
// }
