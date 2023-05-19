/*

https://manpages.ubuntu.com/manpages/xenial/man8/apt.8.html

    Performs the requested action on one or more packages specified via regex(7),
    glob(7) or exact match. The requested action can be overridden for
    specific packages by append a plus (+) to the package name to install
    this package or a minus (-) to remove it.

*/

package runners

import (
	"fmt"
	"os"

	"github.com/gwillem/whip/internal/model"
	"github.com/k0kubun/pp"
)

const (
	aptBin = "/usr/bin/apt-get"

	aptInstall aptState = iota
	aptRemove
	aptPurge
)

type (
	aptState int
)

func buildAptCmd(args model.TaskArgs) ([]string, error) {
	pkglist := map[aptState][]string{}

	var state aptState
	switch args.String("state") {
	case "latest", "install":
		state = aptInstall
	case "absent":
		state = aptRemove
	case "purge":
		state = aptPurge
	default:
		return nil, fmt.Errorf("invalid state: %s", args.String("state"))
	}

	pkglist[state] = append(pkglist[state], args.StringSlice("name")...)

	modifiers := map[aptState]string{
		aptInstall: "+",
		aptRemove:  "-",
		aptPurge:   "",
	}

	pkgs := []string{}
	for state, names := range pkglist {
		for _, pkg := range names {
			pkg += modifiers[state]
			pkgs = append(pkgs, pkg)
		}
	}

	cmd := []string{"sudo", aptBin, "-y", "purge"}
	cmd = append(cmd, pkgs...)
	return cmd, nil
}

func Apt(args model.TaskArgs) (tr model.TaskResult) {
	fmt.Fprintln(os.Stderr, "starting apt task", args)

	if !isExecutable(aptBin) {
		return failure(aptBin + " not found")
	}
	cmd, err := buildAptCmd(args)
	if err != nil {
		return failure(err)
	}
	fmt.Fprintln(os.Stderr, "got cmd", cmd)
	tr = system(cmd)
	pp.Fprintln(os.Stderr, tr)
	return tr
}

func init() {
	registerRunner("apt", Apt, runnerMeta{})
}
