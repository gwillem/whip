package runners

import (
	"fmt"
	"strings"

	"github.com/gwillem/whip/internal/model"
)

var ServiceStateMap = map[string]string{
	"started":   "start",
	"stopped":   "stop",
	"restarted": "restart",
	"reloaded":  "reload",
}

func Service(t *model.Task) (tr model.TaskResult) {
	name := t.Args.String("name")
	if name == "" {
		return failure("name is a required argument")
	}
	want := t.Args.String("state")
	verb := ServiceStateMap[want]
	if verb == "" {
		return failure("unknown state, try started|stopped|restarted|reloaded")
	}

	// started and stopped are declarative, and systemctl exits zero without
	// doing anything when the unit is already in that state. Reporting that as
	// a change fired every notified handler on every deploy, so ask first.
	if loaded, active, err := unitState(name); err == nil && serviceIsNoop(verb, loaded, active) {
		tr.Status = Success
		tr.Output = fmt.Sprintf("%s is already %s", name, want)
		return tr
	}

	// No shell: the unit name used to be interpolated into a `bash -c` string,
	// so a name carrying a space became two arguments and a name carrying a
	// semicolon became two commands.
	return systemTask(t, []string{"systemctl", verb, name})
}

// serviceIsNoop reports whether the verb has nothing left to do. A unit that is
// not loaded is never a no-op: systemctl has to run and fail, or a typo'd unit
// name would be reported as a state already satisfied.
func serviceIsNoop(verb string, loaded, active bool) bool {
	if !loaded {
		return false
	}
	switch verb {
	case "start":
		return active
	case "stop":
		return !active
	}
	// restart and reload are imperative: only the playbook knows whether they
	// were needed, which is what changed_when is for.
	return false
}

// unitState asks for load and active state in one call. LoadState is what
// separates "stopped" from "no such unit", which is-active alone does not say.
func unitState(name string) (loaded, active bool, err error) {
	out, err := execCommand([]string{"systemctl", "show", "--property=LoadState", "--property=ActiveState", name})
	if err != nil {
		return false, false, fmt.Errorf("systemctl show %s: %w: %s", name, err, out)
	}
	return parseUnitState(string(out))
}

// parseUnitState reads systemctl's key=value block. Parsed by key rather than
// by position, because the order properties come back in is systemd's business.
func parseUnitState(out string) (loaded, active bool, err error) {
	seen := 0
	for _, l := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(l), "=")
		if !ok {
			continue
		}
		switch k {
		case "LoadState":
			loaded, seen = v == "loaded", seen+1
		case "ActiveState":
			// activating counts as active for `started`: the unit is up and
			// systemctl start would do nothing but wait for it.
			active, seen = v == "active" || v == "activating", seen+1
		}
	}
	if seen != 2 {
		return false, false, fmt.Errorf("unexpected systemctl show output: %q", out)
	}
	return loaded, active, nil
}

func init() {
	registerRunner("service", runner{
		run: Service,
		meta: runnerMeta{
			requiredArgs: []string{"name", "state"},
			optionalArgs: []string{},
		},
	})
}
