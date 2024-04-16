package runners

import (
	"fmt"
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/stretchr/testify/require"
)

func TestPreRun(t *testing.T) {
	registerRunner("dummy", runner{
		preRun: func(t *model.Task) model.TaskResult {
			fmt.Println("inside dummy runner")
			return model.TaskResult{Status: Skipped}
		},
	})

	const old = "appel"
	const new = "banana"
	task := model.Task{
		Vars:   model.TaskVars{"fruit": old},
		Runner: "dummy",
	}
	extraVars := map[string]any{"fruit": new}
	tr := PreRun(&task, extraVars)
	require.Equal(t, tr.Status, Skipped)
	require.Equal(t, new, task.Vars["fruit"])
}
