package runners

import (
	osuser "os/user"

	"github.com/gwillem/chief-whip/pkg/whip"
)

func AuthorizedKey(args whip.TaskArgs) (tr whip.TaskResult) {
	tr.Status = ok

	key := args.Key("key")
	if key == "" {
		return failure("no key provided")
	}

	user := args.Key("user")
	if user == "" {
		user = facts["user"]
	}

	if user == "" {
		return failure("no user found to set SSH key for")
	}

	u, e := osuser.Lookup(user)
	if e != nil {
		return failure("no homedir for user " + user)
	}
	homedir := u.HomeDir
	sshdir := homedir + "/.ssh"
	if e := fs.MkdirAll(sshdir, 0700); e != nil {
		return failure("failed to create .ssh dir for user", user, e)
	}
	authfile := sshdir + "/authorized_keys"

	if changed, e := ensureLineInFile(authfile, key); e != nil {
		return failure("failed to add key to authorized_keys file", e)
	} else {
		tr.Changed = changed
	}
	tr.Output = "Installed authorized_key for " + args.Key("user") + " with key " + args.Key("key") + "\n"
	return tr
}

func init() {
	registerRunner("authorized_key", AuthorizedKey, runnerMeta{})
}
