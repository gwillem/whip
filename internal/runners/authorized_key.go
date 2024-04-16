package runners

/* deprecated this module, as it is easier to do with the tree module

func AuthorizedKey(args model.TaskArgs, _ model.TaskVars) (tr model.TaskResult) {
	var err error
	tr.Status = success

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

	if tr.Changed, err = ensureLineInFile(authfile, key); err != nil {
		return failure("failed to add key to authorized_keys file", e)
	}
	tr.Output = "Installed authorized_key for " + args.String("user") + " with key " + args.String("key") + "\n"
	return tr
}

func init() {
	registerRunner("authorized_key", runner{run: AuthorizedKey})
}
*/
