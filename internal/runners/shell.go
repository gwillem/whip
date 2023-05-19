package runners

func Shell(args TaskArgs) (tr TaskResult) {
	cmd := []string{"/bin/bash", "-c", args.String(defaultArg)}
	return system(cmd)
}

func init() {
	registerRunner("shell", Shell, runnerMeta{})
}
