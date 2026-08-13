package runners

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gwillem/whip/internal/model"
)

// defaultCreateMode is what a file this runner had to create gets. 0644
// because the runner writes configuration, and configuration is read by
// daemons running as somebody else.
const defaultCreateMode os.FileMode = 0o644

func init() {
	registerRunner("lineinfile", runner{
		run: LineInFile,
		meta: runnerMeta{
			requiredArgs: []string{"path", "line"},
			// Declared, so that an argument this runner does not read --
			// Ansible's insertafter, state, backup -- is a load-time error
			// rather than a line quietly appended to the wrong place.
			optionalArgs: []string{"regexp", "create", "mode"},
		},
	})
}

func LineInFile(t *model.Task) (tr model.TaskResult) {
	line := t.Args.String("line")
	path := t.Args.String("path")
	if line == "" || path == "" {
		return failure("line and path are required arguments")
	}

	opts, err := lineInFileArgs(t.Args)
	if err != nil {
		return failure(err)
	}

	changed, err := ensureLineInFile(path, line, opts)
	if err != nil {
		return failure("failed to ensure line in file:", err)
	}
	tr.Changed = changed
	tr.Status = Success
	return tr
}

// lineInFileArgs reads the optional arguments. The defaults are the previous
// behaviour plus creation: a missing file is created, and without a regexp the
// line is appended if it is not already there verbatim.
func lineInFileArgs(args model.TaskArgs) (lineInFileOpts, error) {
	opts := lineInFileOpts{create: true, mode: defaultCreateMode}

	if v, ok := args["create"]; ok {
		b, err := argBool(v)
		if err != nil {
			return opts, fmt.Errorf("create: %w", err)
		}
		opts.create = b
	}

	if v, ok := args["mode"]; ok {
		m, err := argMode(v)
		if err != nil {
			return opts, fmt.Errorf("mode: %w", err)
		}
		opts.mode = m
	}

	if v, ok := args["regexp"]; ok {
		s, ok := v.(string)
		if !ok {
			return opts, fmt.Errorf("regexp must be a string, got %v", v)
		}
		if s != "" {
			re, err := regexp.Compile(s)
			if err != nil {
				return opts, fmt.Errorf("regexp: %w", err)
			}
			opts.re = re
		}
	}

	return opts, nil
}

// argBool reads a YAML boolean argument. Strings are accepted too, because a
// templated argument arrives as one, and because YAML 1.1 spelled booleans
// half a dozen ways that yaml.v3 hands over as plain text.
func argBool(v any) (bool, error) {
	switch v := v.(type) {
	case bool:
		return v, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "yes", "on":
			return true, nil
		case "no", "off":
			return false, nil
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, fmt.Errorf("%q is not a boolean", v)
		}
		return b, nil
	default:
		return false, fmt.Errorf("%v is not a boolean", v)
	}
}

// argMode reads a file mode. An integer arrives already resolved, because YAML
// reads a leading zero as octal; a string is read as octal digits, which is how
// a mode is written everywhere else.
func argMode(v any) (os.FileMode, error) {
	var n uint64
	switch v := v.(type) {
	case int:
		if v < 0 {
			return 0, fmt.Errorf("%d is not a mode", v)
		}
		n = uint64(v)
	case string:
		parsed, err := strconv.ParseUint(strings.TrimSpace(v), 8, 32)
		if err != nil {
			return 0, fmt.Errorf("%q is not an octal mode", v)
		}
		n = parsed
	default:
		return 0, fmt.Errorf("%v is not a mode", v)
	}
	if n > 0o7777 {
		return 0, fmt.Errorf("mode %o is out of range", n)
	}
	return os.FileMode(n), nil
}
