package runners

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

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
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}

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

	isDir, err := fsutil.IsDir(path)
	if err != nil {
		return false, err
	}
	if isDir {
		return false, fmt.Errorf("path is a directory")
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

		if e := fh.Close(); e != nil {
			return false, e
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
