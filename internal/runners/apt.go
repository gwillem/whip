/*

https://manpages.ubuntu.com/manpages/xenial/man8/apt.8.html

    Performs the requested action on one or more packages specified via regex(7),
    glob(7) or exact match. The requested action can be overridden for
    specific packages by append a plus (+) to the package name to install
    this package or a minus (-) to remove it.

*/

package runners

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/model"
)

const (
	aptBin     = "/usr/bin/apt-get"
	aptListBin = "apt"

	aptInstall = "install"
	aptLatest  = "latest"
	aptRemove  = "remove"
	aptPurge   = "purge"

	// Where apt records its last index refresh. The stamp file only exists
	// when APT::Periodic::Update-Package-Lists is enabled, so the lists
	// directory is the fallback: `apt-get update` always writes into it.
	aptStampFile = "/var/lib/apt/periodic/update-success-stamp"
	aptListsDir  = "/var/lib/apt/lists"
)

// dpkgOpts stop an unattended run from halting on a locally modified config
// file. These used to be one shell string; apt is now executed without a
// shell, so the quotes that only protected them from bash must not be passed
// on -- dpkg would receive them literally.
var dpkgOpts = []string{
	"-o", "Dpkg::Options::=--force-confdef",
	"-o", "Dpkg::Options::=--force-confold",
}

var aptStateMap = map[string]string{
	"present": aptInstall,
	"latest":  aptLatest,
	"absent":  aptRemove,
	"purged":  aptPurge,
}

type aptPkgState map[string]map[string]bool

func (aps aptPkgState) add(pkg, state string) {
	if aps[state] == nil {
		aps[state] = map[string]bool{}
	}
	aps[state][pkg] = true
}

// aptInventory is what the host says about itself: the installed version of
// every package, plus the ones with a newer candidate. Versions matter because
// a wanted name may carry a pin, and "upgradable" is the only way to tell
// whether a `latest` package has work outstanding.
type aptInventory struct {
	installed  map[string]string
	upgradable map[string]bool
}

// aptRun executes one apt invocation and returns its two streams separately,
// so a failure can quote what apt actually said instead of an exit code that
// is 100 for everything from a typo to a resolver dead end. It is a variable
// so the tests can drive the runner on a host without apt.
var aptRun = func(cmd []string) (stdout, stderr string, err error) {
	c := exec.Command(cmd[0], cmd[1:]...)
	// Apt asks questions -- config file conflicts, service restarts, tzdata --
	// unless it is told there is nobody there to answer them.
	c.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	var out, errOut bytes.Buffer
	c.Stdout, c.Stderr = &out, &errOut
	err = c.Run()
	return out.String(), errOut.String(), err
}

// aptStateOf translates a playbook state into an apt subcommand. An unknown
// state used to mean "present", which turned Ansible's `state: latest` into a
// plain install without a word; it is an error now.
func aptStateOf(s string) (string, error) {
	if s == "" {
		return aptInstall, nil
	}
	if v, ok := aptStateMap[s]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown apt state %q, try present|latest|absent|purged", s)
}

// splitPkgSpec separates a version pin from the package name, at the last "="
// because a version may not contain one but an epoch-laden name will not
// either. Apt's own syntax is name=version and must reach it intact; whip used
// to hand every name to the generic key=value parser, which turned
// "nginx=1.24.0-1" into an argument called nginx plus a package with no name.
func splitPkgSpec(spec string) (name, version string) {
	if i := strings.LastIndex(spec, "="); i > 0 {
		return spec[:i], spec[i+1:]
	}
	return spec, ""
}

func buildWanted(args model.TaskArgs) (aptPkgState, error) {
	pkglist := aptPkgState{}
	defaultState, err := aptStateOf(args.String("state"))
	if err != nil {
		return nil, err
	}

	for _, entry := range args.StringSlice("name") {
		state := defaultState
		pkgs := []string{}

		// `state` is the only argument this runner has ever accepted inside a
		// name, so every other token holding an "=" is a version pin and is
		// passed to apt as it stands.
		for tok := range strings.SplitSeq(entry, " ") {
			switch tok = strings.TrimSpace(tok); {
			case tok == "":
			case strings.HasPrefix(tok, "state="):
				if state, err = aptStateOf(strings.TrimPrefix(tok, "state=")); err != nil {
					return nil, err
				}
			default:
				pkgs = append(pkgs, tok)
			}
		}

		for _, p := range pkgs {
			pkglist.add(p, state)
		}
	}
	return pkglist, nil
}

// parseAptList reads `apt list` output, whose lines are
// "name/suite version arch [status]". Lines without a slash in the first field
// are apt's own chatter, such as the leading "Listing...".
func parseAptList(out string) map[string]string {
	pkgs := map[string]string{}
	for line := range strings.SplitSeq(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || !strings.Contains(f[0], "/") {
			continue
		}
		pkgs[strings.SplitN(f[0], "/", 2)[0]] = f[1]
	}
	return pkgs
}

func buildInventory() (aptInventory, error) {
	inv := aptInventory{installed: map[string]string{}, upgradable: map[string]bool{}}

	stdout, stderr, err := aptRun([]string{aptListBin, "list", "--installed"})
	if err != nil {
		return inv, fmt.Errorf("apt list --installed: %s", aptError(stdout, stderr, err))
	}
	inv.installed = parseAptList(stdout)

	stdout, stderr, err = aptRun([]string{aptListBin, "list", "--upgradable"})
	if err != nil {
		return inv, fmt.Errorf("apt list --upgradable: %s", aptError(stdout, stderr, err))
	}
	for name := range parseAptList(stdout) {
		inv.upgradable[name] = true
	}

	return inv, nil
}

// aptWorklist diffs the wanted state against the host and keeps only what is
// outstanding, so a converged host runs no apt at all and reports unchanged.
func aptWorklist(wanted aptPkgState, inv aptInventory) aptPkgState {
	work := aptPkgState{}
	for state, pkgs := range wanted {
		for spec := range pkgs {
			name, version := splitPkgSpec(spec)
			have, installed := inv.installed[name]

			switch state {
			case aptInstall:
				if !installed || (version != "" && have != version) {
					work.add(spec, state)
				}
			case aptLatest:
				// The name alone cannot say whether a newer version exists, so
				// this leans on the candidate list apt already computed.
				if !installed || inv.upgradable[name] || (version != "" && have != version) {
					work.add(spec, state)
				}
			default:
				// A pin means nothing when removing, and apt would have to
				// resolve it before agreeing to drop the package.
				if installed {
					work.add(name, state)
				}
			}
		}
	}
	return work
}

// aptCommands renders the worklist as apt invocations.
//
// One `apt-get dselect-upgrade` used to do all of it. That resolver refuses
// the entire transaction when a member breaks somebody's Recommends -- purging
// snapd while keeping apparmor, which Recommends it -- and names no package
// when it gives up:
//
//	E: Unable to satisfy dependencies. Reached two conflicting assignments
//	E: Internal error, problem resolver broke stuff
//
// install/remove/purge with explicit lists resolve one transaction at a time
// and say which one failed.
func aptCommands(work aptPkgState, defaultRelease string) [][]string {
	cmds := [][]string{}

	// present and latest are the same invocation: `apt-get install` upgrades a
	// package that is already there. They differ only in the diff above.
	if pkgs := sortedPkgs(work[aptInstall], work[aptLatest]); len(pkgs) > 0 {
		argv := []string{aptBin, aptInstall, "-y", "-q"}
		argv = append(argv, dpkgOpts...)
		if defaultRelease != "" {
			argv = append(argv, "-t", defaultRelease)
		}
		cmds = append(cmds, append(argv, pkgs...))
	}

	for _, state := range []string{aptRemove, aptPurge} {
		pkgs := sortedPkgs(work[state])
		if len(pkgs) == 0 {
			continue
		}
		argv := []string{aptBin, state, "-y", "-q"}
		argv = append(argv, dpkgOpts...)
		cmds = append(cmds, append(argv, pkgs...))
	}

	return cmds
}

// sortedPkgs flattens package sets into a stable order, so the command line is
// reproducible and readable in a log.
func sortedPkgs(sets ...map[string]bool) []string {
	pkgs := []string{}
	for _, s := range sets {
		for p := range s {
			pkgs = append(pkgs, p)
		}
	}
	sort.Strings(pkgs)
	return pkgs
}

// aptCacheAge reports how long ago the package index was refreshed, and
// whether that could be established at all.
func aptCacheAge(now time.Time) (time.Duration, bool) {
	for _, p := range []string{aptStampFile, aptListsDir} {
		if fi, err := fs.Stat(p); err == nil {
			return now.Sub(fi.ModTime()), true
		}
	}
	return 0, false
}

// shouldUpdateCache decides whether to refresh the index before comparing
// state. cache_valid_time exists because a playbook that installs from a
// freshly shipped source list must refresh, while the other twenty apt tasks
// in the same run must not each pay for it.
func shouldUpdateCache(args model.TaskArgs, now time.Time) (bool, error) {
	want, err := aptBool(args, "update_cache")
	if err != nil || !want {
		return false, err
	}

	validFor, err := aptSeconds(args, "cache_valid_time")
	if err != nil {
		return false, err
	}
	if validFor <= 0 {
		return true, nil
	}

	age, ok := aptCacheAge(now)
	if !ok {
		// No index to date: refreshing is the safe reading.
		return true, nil
	}
	return age > validFor, nil
}

// aptBool reads a boolean argument, accepting both a YAML boolean and the
// "key=value" string dialect the inline task form produces.
func aptBool(args model.TaskArgs, key string) (bool, error) {
	switch v := args[key].(type) {
	case nil:
		return false, nil
	case bool:
		return v, nil
	case string:
		if v == "" {
			return false, nil
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, fmt.Errorf("%s: %q is not a boolean", key, v)
		}
		return b, nil
	default:
		return false, fmt.Errorf("%s: %v is not a boolean", key, v)
	}
}

// aptSeconds reads a duration given as a plain number of seconds.
func aptSeconds(args model.TaskArgs, key string) (time.Duration, error) {
	var n int64

	switch v := args[key].(type) {
	case nil:
		return 0, nil
	case int:
		n = int64(v)
	case int64:
		n = v
	case float64:
		n = int64(v)
	case string:
		if v == "" {
			return 0, nil
		}
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%s: %q is not a number of seconds", key, v)
		}
		n = parsed
	default:
		return 0, fmt.Errorf("%s: %v is not a number of seconds", key, v)
	}

	return time.Duration(n) * time.Second, nil
}

// aptError renders apt's own complaint. Apt writes the resolver's explanation
// to stderr and exits 100, so the status on its own says nothing.
func aptError(stdout, stderr string, err error) string {
	msg := strings.TrimSpace(stderr)
	if msg == "" {
		msg = strings.TrimSpace(stdout)
	}
	if msg == "" {
		return err.Error()
	}
	return err.Error() + "\n" + msg
}

// runAptCommands executes the rendered invocations in order and returns the
// transcript. The error carries the failing command line and apt's stderr.
func runAptCommands(cmds [][]string) (string, error) {
	out := strings.Builder{}

	for _, cmd := range cmds {
		line := strings.Join(cmd, " ")
		log.Debug("running", line)

		stdout, stderr, err := aptRun(cmd)
		out.WriteString(line + "\n" + stdout)
		if err != nil {
			return out.String(), fmt.Errorf("%s: %s", line, aptError(stdout, stderr, err))
		}
	}

	return out.String(), nil
}

func apt(t *model.Task) (tr model.TaskResult) {
	if !isExecutable(aptBin) {
		return failure("cannot run", aptBin)
	}

	// Parse the arguments before touching the host, so a typo in a state does
	// not leave the index refreshed and half the packages moved.
	wanted, err := buildWanted(t.Args)
	if err != nil {
		return failure(err)
	}

	update, err := shouldUpdateCache(t.Args, time.Now())
	if err != nil {
		return failure(err)
	}

	out := strings.Builder{}
	if update {
		// Refresh before reading the inventory: a task that just shipped a new
		// source list would otherwise resolve against a stale index.
		transcript, err := runAptCommands([][]string{{aptBin, "update", "-q"}})
		out.WriteString(transcript)
		if err != nil {
			return failure(err)
		}
	}

	inv, err := buildInventory()
	if err != nil {
		return failure("cannot get current apt state", err)
	}

	cmds := aptCommands(aptWorklist(wanted, inv), t.Args.String("default_release"))
	if len(cmds) == 0 {
		// Converged. A cache refresh on its own is not a change to the host.
		return model.TaskResult{Status: Success, Output: out.String()}
	}

	transcript, err := runAptCommands(cmds)
	out.WriteString(transcript)
	if err != nil {
		return failure(err)
	}

	return model.TaskResult{Status: Success, Changed: true, Output: out.String()}
}

func init() {
	registerRunner("apt", runner{
		run: apt,
		meta: runnerMeta{
			requiredArgs: []string{"name"},
			optionalArgs: []string{"state", "update_cache", "cache_valid_time", "default_release"},
		},
	})
}
