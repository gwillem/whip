package main

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/gwillem/whip/internal/ssh"
)

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
	osarg = strings.ReplaceAll(osarg, "x86_64", "amd64")

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
