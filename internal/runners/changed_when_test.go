package runners

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/stretchr/testify/require"
)

// system() reports every command as changed, so every shell and command task
// was a change on every deploy: the run summary said nothing and every notified
// handler fired regardless.
func TestChangedWhenDecidesTheChangedFlag(t *testing.T) {
	run := func(cmd string, changedWhen any) model.TaskResult {
		task := shellTask(cmd)
		if changedWhen != nil {
			task.Args[changedWhenArg] = changedWhen
		}
		return Run(task, model.TaskVars{})
	}

	t.Run("without changed_when the command still reports changed", func(t *testing.T) {
		tr := run("true", nil)
		require.Equal(t, Success, tr.Status, tr.Output)
		require.True(t, tr.Changed)
	})

	t.Run("a zero exit means changed", func(t *testing.T) {
		tr := run("true", "test 1 = 1")
		require.Equal(t, Success, tr.Status, tr.Output)
		require.True(t, tr.Changed)
	})

	t.Run("a non-zero exit means unchanged", func(t *testing.T) {
		tr := run("true", "test 1 = 2")
		require.Equal(t, Success, tr.Status, tr.Output)
		require.False(t, tr.Changed)
	})

	t.Run("a bool says so outright", func(t *testing.T) {
		tr := run("true", false)
		require.Equal(t, Success, tr.Status, tr.Output)
		require.False(t, tr.Changed)

		tr = run("true", true)
		require.True(t, tr.Changed)
	})

	t.Run("an empty expression is no expression", func(t *testing.T) {
		tr := run("true", "")
		require.True(t, tr.Changed)
	})

	t.Run("a non-string non-bool is an error", func(t *testing.T) {
		tr := run("true", 42)
		require.Equal(t, Failed, tr.Status)
		require.Contains(t, tr.Output, "changed_when")
	})

	t.Run("the command output is in WHIP_OUTPUT", func(t *testing.T) {
		tr := run("echo added a user", `echo "$WHIP_OUTPUT" | grep -q added`)
		require.Equal(t, Success, tr.Status, tr.Output)
		require.True(t, tr.Changed)

		tr = run("echo nothing to do", `echo "$WHIP_OUTPUT" | grep -q added`)
		require.Equal(t, Success, tr.Status, tr.Output)
		require.False(t, tr.Changed)
	})

	t.Run("the expression is templated like any other argument", func(t *testing.T) {
		task := shellTask("echo added a user")
		task.Args[changedWhenArg] = `echo "$WHIP_OUTPUT" | grep -q {{ needle }}`
		task.Vars["needle"] = "added"

		tr := Run(task, model.TaskVars{})
		require.Equal(t, Success, tr.Status, tr.Output)
		require.True(t, tr.Changed)
	})

	t.Run("a failed command is not judged by the expression", func(t *testing.T) {
		witness := filepath.Join(t.TempDir(), "witness")
		task := shellTask("exit 3")
		task.Args[changedWhenArg] = "touch " + witness

		tr := Run(task, model.TaskVars{})
		require.Equal(t, Failed, tr.Status)
		require.NoFileExists(t, witness, "changed_when judges a command that ran")
	})
}

// A skipped task must not run the expression either: it has no output to judge.
func TestChangedWhenIsNotRunForASkippedTask(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "present")
	require.NoError(t, os.WriteFile(present, []byte("x"), 0o644))
	witness := filepath.Join(dir, "witness")

	task := shellTask("true")
	task.Creates = present
	task.Args[changedWhenArg] = "touch " + witness

	tr := Run(task, model.TaskVars{})
	require.Equal(t, Success, tr.Status, tr.Output)
	require.False(t, tr.Changed)
	require.NoFileExists(t, witness)
}
