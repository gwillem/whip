package runners

import (
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/stretchr/testify/require"
)

func init() {
	registerRunner("dummy", runner{
		prerun: func(t *model.Task) model.TaskResult {
			// fmt.Println("inside dummy runner")
			return model.TaskResult{Status: Skipped}
		},
	})
}

func TestPreRun(t *testing.T) {
	const old = "appel"
	const new = "banana"
	task := model.Task{
		Vars:   model.TaskVars{"key": old},
		Runner: "dummy",
	}
	extraVars := map[string]any{"key": new}
	tr := PreRun(&task, extraVars)
	require.Equal(t, tr.Status, Skipped)
	require.Equal(t, new, task.Vars["key"])
}
