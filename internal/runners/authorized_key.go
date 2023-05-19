package runners

import (
	osuser "os/user"
)

func AuthorizedKey(args TaskArgs) (tr TaskResult) {
	tr.Status = ok

	key := args.String("key")
	if key == "" {
		return failure("no key provided")
	}

	user := args.String("user")
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
	if e := fs.MkdirAll(sshdir, 0o700); e != nil {
		return failure("failed to create .ssh dir for user", user, e)
	}
	authfile := sshdir + "/authorized_keys"

	if changed, e := ensureLineInFile(authfile, key); e != nil {
		return failure("failed to add key to authorized_keys file", e)
	} else {
		tr.Changed = changed
	}
	tr.Output = "Installed authorized_key for " + args.String("user") + " with key " + args.String("key") + "\n"
	return tr
}

func init() {
	registerRunner("authorized_key", AuthorizedKey, runnerMeta{})
}