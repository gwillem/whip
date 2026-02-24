package runners

import (
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/stretchr/testify/require"
)

func init() {
	registerRunner("dummy", runner{
		run: func(t *model.Task) model.TaskResult {
			return model.TaskResult{Status: Success}
		},
		prerun: func(t *model.Task) model.TaskResult {
			// fmt.Println("inside dummy runner")
			return model.TaskResult{Status: Skipped}
		},
	})
}

func Test_RunWithNilPlayVars(t *testing.T) {
	task := model.Task{
		Runner: "dummy",
		Args:   model.TaskArgs{"_args": "hello"},
		Vars:   model.TaskVars{"key": "val"},
	}
	tr := Run(&task, nil)
	require.Equal(t, Success, tr.Status)
	require.Equal(t, "val", task.Vars["key"])
}

func Test_RunWithNilTaskVars(t *testing.T) {
	task := model.Task{
		Runner: "dummy",
		Args:   model.TaskArgs{"_args": "hello"},
	}
	playVars := model.TaskVars{"key": "from_play"}
	tr := Run(&task, playVars)
	require.Equal(t, Success, tr.Status)
	require.Equal(t, "from_play", task.Vars["key"])
}

func Test_PreRun(t *testing.T) {
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
