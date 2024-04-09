package runners

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gwillem/whip/internal/model"
	"github.com/karrick/gobls"
	"github.com/spf13/afero"
)

func ensureLineInFile(path, line string) (bool, error) {
	// will add later
	line = strings.TrimRight(line, "\r\n")

	if strings.Contains(line, "\n") {
		return false, fmt.Errorf("line cannot contain newline")
	}

	pathExists, err := fsutil.Exists(path)
	if err != nil {
		return false, err
	}

	if pathExists {
		fh, err := fs.Open(path)
		if err != nil {
			return false, err
		}
		defer fh.Close()

		ls := gobls.NewScanner(fh)
		for ls.Scan() {
			found := ls.Text()
			if found == line {
				return false, nil
			}
		}

		if err := fh.Close(); err != nil {
			return false, err
		}
	}
	// line not found, append it
	if e := appendLineToFile(path, line); e != nil {
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

func createTestFS() {
	fs = afero.NewMemMapFs()
	fsutil = &afero.Afero{Fs: fs}
}

func tplParse(tpl string, data map[string]any) (string, error) {
	t, err := tplParser.FromString(tpl)
	if err != nil {
		return "", err
	}
	return t.Execute(data)
}

func system(cmd []string) (tr model.TaskResult) {
	data, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	tr.Changed = true
	if err == nil {
		tr.Status = ok
		tr.Output = string(data)
	} else {
		tr.Status = failed
		tr.Output = strings.Join(cmd, " ") + "\n" + err.Error() + ":\n" + string(data)
	}
	return tr
}
