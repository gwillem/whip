package runners

import "github.com/gwillem/whip/internal/model"

func init() {
	registerRunner("lineinfile", runner{run: LineInFile})
}

func LineInFile(t *model.Task) (tr model.TaskResult) {
	line := t.Args.String("line")
	path := t.Args.String("path")
	if line == "" || path == "" {
		return failure("line and path are required arguments")
	}
	changed, err := ensureLineInFile(path, line)
	if err != nil {
		return failure("failed to ensure line in file:", err)
	}
	tr.Changed = changed
	tr.Status = Success
	return tr
}
