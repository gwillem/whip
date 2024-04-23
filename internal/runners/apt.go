/*

https://manpages.ubuntu.com/manpages/xenial/man8/apt.8.html

    Performs the requested action on one or more packages specified via regex(7),
    glob(7) or exact match. The requested action can be overridden for
    specific packages by append a plus (+) to the package name to install
    this package or a minus (-) to remove it.

*/

package runners

import (
	"os/exec"
	"strings"

	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/parser"
	"golang.org/x/exp/maps"
)

const (
	aptBin    = "/usr/bin/apt-get"
	installed = "install"
	removed   = "remove"
	purged    = "purge"
)

var aptStateMap = map[string]string{
	"present": installed,
	"absent":  removed,
	"purged":  purged,
}

type aptPkgState map[string]map[string]bool

func (aps aptPkgState) add(pkg, state string) {
	if aps[state] == nil {
		aps[state] = map[string]bool{}
	}
	aps[state][pkg] = true
}

func (aps aptPkgState) has(pkg, state string) bool {
	return aps[state][pkg]
}

func buildCurrent() (aptPkgState, error) {
	pkglist := aptPkgState{}

	data, err := exec.Command("apt", "list", "--installed").CombinedOutput()
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(string(data), "\n") {
		if len(line) == 0 {
			continue
		}
		pkg := strings.Split(line, "/")[0]
		pkglist.add(pkg, installed)
	}
	return pkglist, nil
}

func getState(s string) string {
	if val := aptStateMap[s]; val != "" {
		return val
	}
	return installed
}

func buildWanted(args model.TaskArgs) (aptPkgState, error) {
	pkglist := aptPkgState{}
	defaultState := getState(args.String("state"))
	for _, p := range args.StringSlice("name") {
		state := defaultState
		args := parser.ParseArgString(p)
		if s := args.String("state"); s != "" {
			state = getState(s)
		}
		pkglist.add(args.String(parser.DefaultArg), state)
	}
	return pkglist, nil
}

func apt(t *model.Task) (tr model.TaskResult) {
	if !isExecutable(aptBin) {
		return failure("cannot run", aptBin)
	}

	current, err := buildCurrent()
	if err != nil {
		return failure("cannot get current apt state", err)
	}
	wanted, err := buildWanted(t.Args)
	if err != nil {
		return failure("cannot get wanted apt state", err)
	}

	worklist := aptPkgState{}
	for state, pkgs := range wanted {
		for p := range pkgs {
			if state == installed && !current.has(p, state) {
				worklist.add(p, state)
			} else if state != installed && current.has(p, installed) {
				worklist.add(p, state)
			}
		}
	}

	total := 0

	for state, pkgs := range worklist {
		if len(pkgs) == 0 {
			log.Debug("nothing in state (should not happen!)", state)
			continue
		}
		total += len(pkgs)
		args := append([]string{state}, maps.Keys(pkgs)...)
		log.Debug("running apt-mark with", args)
		data, err := exec.Command("apt-mark", args...).CombinedOutput()
		if err != nil {
			return failure("cannot mark packages\n" + string(data))
		}
		log.Debug("apt-mark", string(data))
	}

	if total == 0 {
		return model.TaskResult{Status: Success}
	}

	return runShell("DEBIAN_FRONTEND=noninteractive apt-get dselect-upgrade -y -q")
}

func init() {
	registerRunner("apt", runner{run: apt})
}
