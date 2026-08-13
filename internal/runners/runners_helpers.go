package runners

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/model"
	"github.com/karrick/gobls"
	"github.com/spf13/afero"
)

func getDataChecksum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func getFileChecksum(fs afero.Fs, filePath string) ([]byte, error) {
	file, err := fs.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}

// lineInFileOpts carries the optional behaviour of the lineinfile runner.
type lineInFileOpts struct {
	// re, when set, names the one line the file is allowed to have: the first
	// match becomes line and any later match is dropped. Without it the runner
	// can only append, so a value that changes leaves BOTH lines in the file
	// and which one wins is per-consumer -- php-fpm and php.ini take the last,
	// so a stale `pm.max_children = 40` from an earlier run silently overrode
	// the intended one.
	re *regexp.Regexp

	// create allows a missing file (and its parent directory) to be created.
	// This is the default, and the historic intent: the append below always
	// passed O_CREATE. Set create: false to fail on a missing file instead.
	create bool

	// mode is applied to a file this runner creates, and never to one that
	// already exists -- whoever owns an existing config file owns its mode.
	mode os.FileMode
}

func ensureLineInFile(path, line string, opts lineInFileOpts) (bool, error) {
	line = strings.TrimRight(line, "\r\n")

	if strings.Contains(line, "\n") {
		return false, fmt.Errorf("line cannot contain newline")
	}

	fi, err := fs.Stat(path)
	switch {
	case err == nil && fi.IsDir():
		return false, fmt.Errorf("path is a directory")
	case err == nil:
		// exists, and is a file
	case !os.IsNotExist(err):
		// A permission problem, or a file where a parent directory should be:
		// worth reporting, unlike a plain absence.
		return false, err
	case !opts.create:
		return false, fmt.Errorf("%s does not exist and create is false", path)
	default:
		// ENOENT. fsutil.IsDir returned this error rather than false, so the
		// runner failed here and could never reach its own O_CREATE append.
		if e := createEmptyFile(path, opts.mode); e != nil {
			return false, e
		}
	}

	if opts.re != nil {
		return replaceLineInFile(path, line, opts.re)
	}

	present, err := fileHasLine(path, line)
	if err != nil {
		return false, err
	}
	if present {
		return false, nil
	}
	// line not found, append it
	if e := appendLineToFile(path, line); e != nil {
		return false, e
	}

	return true, nil
}

// fileHasLine reports whether the file already holds line verbatim. gobls
// rather than bufio because a minified asset on one line would overflow the
// scanner's buffer and be read as "not present".
func fileHasLine(path, line string) (bool, error) {
	fh, err := fs.Open(path)
	if err != nil {
		return false, err
	}
	defer fh.Close() //nolint:errcheck

	ls := gobls.NewScanner(fh)
	for ls.Scan() {
		if ls.Text() == line {
			return true, nil
		}
	}
	return false, ls.Err()
}

// replaceLineInFile converges the file on exactly one line matching re: the
// first match is rewritten and any further match is deleted. Deleting the
// duplicates matters because files written in the append-only era already
// carry stale copies of the directive, and the last one is what php-fpm,
// php.ini and my.cnf take.
func replaceLineInFile(path, line string, re *regexp.Regexp) (bool, error) {
	data, err := fsutil.ReadFile(path)
	if err != nil {
		return false, err
	}

	var lines []string
	if len(data) > 0 {
		lines = strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	}

	out := make([]string, 0, len(lines)+1)
	replaced, changed := false, false
	for _, l := range lines {
		switch {
		case !re.MatchString(l):
			out = append(out, l)
		case replaced:
			changed = true // a stale duplicate, drop it
		default:
			replaced = true
			changed = changed || l != line
			out = append(out, line)
		}
	}
	if !replaced {
		out = append(out, line)
		changed = true
	}
	if !changed {
		return false, nil
	}

	body := strings.Join(out, "\n") + "\n"

	// Written in place rather than through a temp file and a rename, so the
	// file keeps its owner, mode and any ACLs: a rewritten php.ini that
	// suddenly belongs to root is worse than a torn write.
	if e := fsutil.WriteFile(path, []byte(body), 0o644); e != nil {
		return false, e
	}
	return true, nil
}

func isExecutable(path string) bool {
	fi, err := fs.Stat(path)
	if err != nil {
		return false
	}
	return fi.Mode().Perm()&0o111 != 0
}

// createEmptyFile creates path plus any missing parent directory.
func createEmptyFile(path string, mode os.FileMode) error {
	if e := fs.MkdirAll(filepath.Dir(path), 0o755); e != nil {
		return e
	}
	f, err := fs.OpenFile(path, os.O_CREATE|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if e := f.Close(); e != nil {
		return e
	}
	// OpenFile's mode is masked by the umask, and a config file that has to be
	// world-readable is worth being exact about.
	return fs.Chmod(path, mode)
}

func appendLineToFile(path, line string) error {
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}

	f, err := fs.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write([]byte(line)); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

func tplParseString(tpl string, data map[string]any) (string, error) {
	t, err := tplParser.FromString(tpl)
	if err != nil {
		return "", err
	}
	return t.Execute(data)
}

func tplParseBytes(tpl []byte, data map[string]any) ([]byte, error) {
	t, err := tplParser.FromBytes(tpl)
	if err != nil {
		return nil, err
	}
	return t.ExecuteBytes(data)
}

func system(cmd []string) (tr model.TaskResult) {
	tr.Changed = true
	if len(cmd) == 0 {
		return failure("no command")
	}

	data, err := execCommand(cmd)

	if err == nil {
		tr.Status = Success
		tr.Output = string(data)
	} else {
		tr.Status = Failed
		tr.Output = strings.Join(cmd, " ") + "\n" + err.Error() + ":\n" + string(data)
	}
	return tr
}

// changedWhenArg is read by the framework rather than by any one runner, and
// is listed in universalArgs so every runner accepts it.
const changedWhenArg = "changed_when"

// systemTask runs cmd on behalf of a task and lets `changed_when` decide
// whether the task counts as a change. system() reports every command as
// changed, which is the only honest default for an opaque command but makes
// the run summary meaningless and fires every notified handler on every
// deploy.
func systemTask(t *model.Task, cmd []string) model.TaskResult {
	return applyChangedWhen(t, system(cmd))
}

// applyChangedWhen resolves the task's changed_when argument, which is either
//
//	changed_when: false          -- this command is never a change
//	changed_when: <shell expr>   -- exit zero means changed
//
// The command's own output is in $WHIP_OUTPUT, so the expression can grep what
// the task printed instead of looking the same thing up a second time.
//
// A failed task keeps whatever changed flag it had: the expression would be
// judging a command that did not run to completion.
func applyChangedWhen(t *model.Task, tr model.TaskResult) model.TaskResult {
	if t == nil || tr.Status != Success {
		return tr
	}

	switch v := t.Args[changedWhenArg].(type) {
	case nil:
		return tr
	case bool:
		tr.Changed = v
	case string:
		if strings.TrimSpace(v) == "" {
			return tr
		}
		c := exec.Command("/bin/sh", "-c", v)
		c.Env = append(os.Environ(), "WHIP_OUTPUT="+tr.Output)
		out, err := c.CombinedOutput()
		tr.Changed = err == nil
		log.Debug("changed_when", v, "->", tr.Changed, strings.TrimSpace(string(out)))
	default:
		return failure("changed_when must be a shell expression or a bool, got", v)
	}
	return tr
}

func execCommand(cmd []string) ([]byte, error) {
	args := []string{}
	if len(cmd) > 1 {
		args = cmd[1:]
	}
	return exec.Command(cmd[0], args...).CombinedOutput()
}

func isText(s []byte) bool {
	const max = 1024 // at least utf8.UTFMax
	if len(s) > max {
		s = s[0:max]
	}
	for i, c := range string(s) {
		if i+utf8.UTFMax > len(s) {
			// last char may be incomplete - ignore
			break
		}
		if c == 0xFFFD || c < ' ' && c != '\n' && c != '\t' && c != '\f' {
			// decoding error or control character - not a text file
			return false
		}
	}
	return true
}
