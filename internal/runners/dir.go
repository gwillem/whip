package runners

import (
	"fmt"
	"os"
	"strconv"

	"github.com/gwillem/whip/internal/model"
)

func init() {
	registerRunner("dir", runner{
		run: dirRunner,
		meta: runnerMeta{
			requiredArgs: []string{"path"},
			optionalArgs: []string{"mode", "owner", "group", "state"},
			// A path is one value, so `dir: /srv/honeypot/capture` must not be
			// split on spaces with its key=value tokens stolen.
			stringArg: "path",
		},
	})
}

// defaultDirMode is what install(1) uses without -m, which is what the shell
// blocks this runner replaces relied upon.
const defaultDirMode = os.FileMode(0o755)

// dirRunner creates or removes a directory tree. `install -d` in a shell block
// is idempotent, so the workaround was safe, but its mode and owner were then
// invisible to whip: nothing checked them on the next run, and drift was never
// reported as a change.
func dirRunner(t *model.Task) (tr model.TaskResult) {
	path := argString(t.Args, "path")
	if path == "" {
		return failure("dir requires a path")
	}

	switch state := argString(t.Args, "state"); state {
	case "", "present":
		// present is the default
	case "absent":
		ok, err := fsutil.Exists(path)
		if err != nil {
			return failure("cannot read", path, err)
		}
		if !ok {
			return model.TaskResult{Status: Success, Output: fmt.Sprintf("%-7s %s\n", "skip", path)}
		}
		if err := fs.RemoveAll(path); err != nil {
			return failure("cannot remove", path, err)
		}
		return model.TaskResult{
			Status:  Success,
			Changed: true,
			Output:  fmt.Sprintf("%-7s %s\n", "removed", path),
		}
	default:
		return failure("unknown state", state)
	}

	mode := defaultDirMode
	if s := argString(t.Args, "mode"); s != "" {
		m, err := parseFileMode(s)
		if err != nil {
			return failure(err)
		}
		mode = m
	}

	f := filesObj{path: path, isDir: true, mode: mode}

	if owner := argString(t.Args, "owner"); owner != "" {
		uid, err := lookupUID(owner)
		if err != nil {
			return failure(err)
		}
		f.uid = &uid
	}

	if group := argString(t.Args, "group"); group != "" {
		gid, err := lookupGID(group)
		if err != nil {
			return failure(err)
		}
		f.gid = &gid
	}

	changed := false
	if ok, err := fsutil.Exists(path); err != nil {
		return failure("cannot read", path, err)
	} else if !ok {
		// MkdirAll, because a deployment names /srv/honeypot/capture/bodies
		// before either parent exists. ensureDir below then sets the leaf's
		// mode, which MkdirAll reduced by the process umask, and its owner.
		if err := fs.MkdirAll(path, mode); err != nil {
			return failure("cannot create", path, err)
		}
		changed = true
	}

	c, err := ensureDir(f)
	if err != nil {
		return failure(err)
	}
	changed = changed || c

	status := "skip"
	if changed {
		status = "changed"
	}
	return model.TaskResult{
		Status:  Success,
		Changed: changed,
		Output:  fmt.Sprintf("%-7s %s\n", status, path),
	}
}

// argString reads an argument without assuming its YAML type: a mode written
// unquoted (mode: 0750) arrives as an int, and TaskArgs.String panics on it.
func argString(args model.TaskArgs, key string) string {
	switch v := args[key].(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// parseFileMode reads an octal mode. An unquoted YAML 0750 has already lost its
// octal meaning by the time it gets here, so tell the user to quote it.
func parseFileMode(s string) (os.FileMode, error) {
	m, err := strconv.ParseInt(s, 8, 32)
	if err != nil {
		return 0, fmt.Errorf(`cannot parse octal mode %s, quote it as mode: "0750"`, s)
	}
	return os.FileMode(m), nil
}

func lookupUID(username string) (int, error) {
	owner, err := osUser.Lookup(username)
	if err != nil {
		return 0, fmt.Errorf("cannot find user %s", username)
	}
	uid, err := strconv.Atoi(owner.Uid)
	if err != nil {
		return 0, fmt.Errorf("cannot parse uid %s", owner.Uid)
	}
	return uid, nil
}

func lookupGID(groupname string) (int, error) {
	group, err := osUser.LookupGroup(groupname)
	if err != nil {
		return 0, fmt.Errorf("cannot find group %s", groupname)
	}
	gid, err := strconv.Atoi(group.Gid)
	if err != nil {
		return 0, fmt.Errorf("cannot parse gid %s", group.Gid)
	}
	return gid, nil
}
