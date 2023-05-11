package runners

import (
	"os"
	"strings"

	"github.com/karrick/gobls"
)

func ensureLineInFile(path, line string) (bool, error) {

	lastchar := ""
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}

	exists, err := fsutil.Exists(path)
	if err != nil {
		return false, err
	}

	if exists {

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
			lastchar = found[len(found)-1:]
		}

		if lastchar != "\n" && lastchar != "" {
			line = "\n" + line
		}

		// line not found, append it
		if err := fh.Close(); err != nil {
			return false, err
		}
	}
	if e := appendLineToFile(path, line); e != nil {
		return false, e
	}

	return true, nil
}

func appendLineToFile(path, line string) error {
	f, err := fs.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
