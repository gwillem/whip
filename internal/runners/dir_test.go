package runners

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/stretchr/testify/require"
)

func runDir(t *testing.T, args model.TaskArgs) model.TaskResult {
	t.Helper()
	return dirRunner(&model.Task{Runner: "dir", Args: args})
}

func Test_dirCreatesAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "srv", "honeypot", "capture", "bodies")

	tr := runDir(t, model.TaskArgs{"path": path, "mode": "0750"})
	require.Equal(t, Success, tr.Status, tr.Output)
	require.True(t, tr.Changed)

	fi, err := os.Stat(path)
	require.NoError(t, err)
	require.True(t, fi.IsDir())
	require.Equal(t, os.FileMode(0o750), fi.Mode().Perm())

	// second run does nothing, so it reports nothing
	tr = runDir(t, model.TaskArgs{"path": path, "mode": "0750"})
	require.Equal(t, Success, tr.Status, tr.Output)
	require.False(t, tr.Changed)
}

func Test_dirFixesMode(t *testing.T) {
	// drift is a change: this is the whole point of not using `install -d`
	path := filepath.Join(t.TempDir(), "spool")
	require.NoError(t, os.Mkdir(path, 0o700))

	tr := runDir(t, model.TaskArgs{"path": path, "mode": "0755"})
	require.Equal(t, Success, tr.Status, tr.Output)
	require.True(t, tr.Changed)

	fi, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o755), fi.Mode().Perm())
}

func Test_dirDefaultMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bundles")

	tr := runDir(t, model.TaskArgs{"path": path})
	require.Equal(t, Success, tr.Status, tr.Output)
	require.True(t, tr.Changed)

	fi, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, defaultDirMode, fi.Mode().Perm())
}

func Test_dirStateAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "obsolete")
	require.NoError(t, os.MkdirAll(filepath.Join(path, "sub"), 0o755))

	tr := runDir(t, model.TaskArgs{"path": path, "state": "absent"})
	require.Equal(t, Success, tr.Status, tr.Output)
	require.True(t, tr.Changed)
	require.NoDirExists(t, path)

	// already gone
	tr = runDir(t, model.TaskArgs{"path": path, "state": "absent"})
	require.Equal(t, Success, tr.Status, tr.Output)
	require.False(t, tr.Changed)
}

func Test_dirStringArg(t *testing.T) {
	// `dir: /srv/honeypot/x` must land in path, verbatim
	arg, ok := StringArg("dir")
	require.True(t, ok)
	require.Equal(t, "path", arg)
}

func Test_dirBadArgs(t *testing.T) {
	tests := []struct {
		name string
		args model.TaskArgs
	}{
		{name: "no path", args: model.TaskArgs{"mode": "0755"}},
		{name: "unknown state", args: model.TaskArgs{"path": "/tmp/whatever", "state": "sublimated"}},
		{name: "unquoted mode", args: model.TaskArgs{"path": "/tmp/whatever", "mode": 493}},
		{name: "unknown owner", args: model.TaskArgs{"path": "/tmp/whatever", "owner": "nosuchuser-whip"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := runDir(t, tt.args)
			require.Equal(t, Failed, tr.Status, tr.Output)
			require.False(t, tr.Changed)
		})
	}
}

func Test_dirOwner(t *testing.T) {
	osUser = getDummyOsUser()
	defer func() {
		osUser = realOsUser{}
	}()

	// chown to a stub uid must fail as a normal user rather than silently pass
	path := filepath.Join(t.TempDir(), "owned")
	tr := runDir(t, model.TaskArgs{"path": path, "owner": testUser, "group": testGroup})
	if os.Geteuid() == 0 {
		require.Equal(t, Success, tr.Status, tr.Output)
		return
	}
	require.Equal(t, Failed, tr.Status)
}
