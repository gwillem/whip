package main

import (
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

	osarch := strings.ToLower(lines[0])
	osarch = strings.ReplaceAll(osarch, " ", "-")
	osarch = strings.ReplaceAll(osarch, "aarch64", "arm64")
	osarch = strings.ReplaceAll(osarch, "x86_64", "amd64")

	remoteSha := strings.TrimSpace(lines[1])

	myDep, err := deputies.ReadFile("deputies/" + osarch)
	if err != nil {
		return fmt.Errorf("could not read deputy for %s: %s", osarch, err)
	}

	localShaBytes, err := deputies.ReadFile("deputies/" + osarch + ".sha256")
	if err != nil {
		return fmt.Errorf("could not read deputy SHA256 for %s: %s", osarch, err)
	}
	localSha := strings.TrimSpace(string(localShaBytes))

	// log.Debugf("local/remote sha:\n\t%s\n\t%s", localSha, remoteSha)

	if localSha == remoteSha {
		// log.Debug("remote deputy seems to be fine")
		return nil
	}

	// log.Debug("uploading deputy for ", osarg)
	if err := c.UploadBytesXZ(myDep, deputyPath, 0o755); err != nil {
		return fmt.Errorf("Could not upload deputy: %s", err)
	}

	return nil
}
