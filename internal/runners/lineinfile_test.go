package runners

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/stretchr/testify/require"
)

func lineInFileTask(args model.TaskArgs) *model.Task {
	return &model.Task{Runner: "lineinfile", Args: args, Vars: model.TaskVars{}}
}

// The append path always passed O_CREATE, but the stat in front of it returned
// the ENOENT error rather than false, so the runner could never create the file
// it plainly meant to. Every target had to be pre-created by a shell task.
func TestLineInFileCreatesMissingFile(t *testing.T) {
	t.Run("creates the file and its parent directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "php", "conf.d", "99-store.ini")

		tr := LineInFile(lineInFileTask(model.TaskArgs{"path": path, "line": "memory_limit = 512M"}))
		require.Equal(t, Success, tr.Status, tr.Output)
		require.True(t, tr.Changed)

		body, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "memory_limit = 512M\n", string(body))

		fi, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o644), fi.Mode().Perm())
	})

	t.Run("mode applies to the created file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "secret.cnf")

		tr := LineInFile(lineInFileTask(model.TaskArgs{
			"path": path, "line": "password = hunter2", "mode": "0600",
		}))
		require.Equal(t, Success, tr.Status, tr.Output)

		fi, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
	})

	t.Run("mode leaves an existing file alone", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "existing.cnf")
		require.NoError(t, os.WriteFile(path, []byte("a = 1\n"), 0o640))

		tr := LineInFile(lineInFileTask(model.TaskArgs{
			"path": path, "line": "b = 2", "mode": "0600",
		}))
		require.Equal(t, Success, tr.Status, tr.Output)

		fi, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o640), fi.Mode().Perm())
	})

	t.Run("create false fails instead of creating", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "absent.ini")

		tr := LineInFile(lineInFileTask(model.TaskArgs{
			"path": path, "line": "a = 1", "create": false,
		}))
		require.Equal(t, Failed, tr.Status)
		require.Contains(t, tr.Output, "create is false")
		require.NoFileExists(t, path)
	})

	t.Run("a directory is still an error", func(t *testing.T) {
		dir := t.TempDir()

		tr := LineInFile(lineInFileTask(model.TaskArgs{"path": dir, "line": "a = 1"}))
		require.Equal(t, Failed, tr.Status)
		require.Contains(t, tr.Output, "is a directory")
	})
}

// Append-only means a value that changes leaves BOTH lines in the file, and
// which one wins is per-consumer: php-fpm and php.ini take the last, so a stale
// pm.max_children silently overrode the intended one. regexp says "this
// directive has one value".
func TestLineInFileRegexpReplacesInPlace(t *testing.T) {
	const seed = "; managed by whip\npm.max_children = 40\npm.max_spare_servers = 3\n"

	seedFile := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "www.conf")
		require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
		return path
	}

	task := func(path string) *model.Task {
		return lineInFileTask(model.TaskArgs{
			"path":   path,
			"line":   "pm.max_children = 5",
			"regexp": `^pm\.max_children\s*=`,
		})
	}

	t.Run("replaces the matching line, keeping its position", func(t *testing.T) {
		path := seedFile(t, seed)

		tr := LineInFile(task(path))
		require.Equal(t, Success, tr.Status, tr.Output)
		require.True(t, tr.Changed)

		body, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t,
			"; managed by whip\npm.max_children = 5\npm.max_spare_servers = 3\n",
			string(body))
	})

	t.Run("rewriting keeps the file's mode", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "www.conf")
		require.NoError(t, os.WriteFile(path, []byte(seed), 0o640))

		require.True(t, LineInFile(task(path)).Changed)

		fi, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o640), fi.Mode().Perm(),
			"a rewritten php.ini that suddenly becomes world-readable is a regression")
	})

	t.Run("is idempotent on a second run", func(t *testing.T) {
		path := seedFile(t, seed)

		require.True(t, LineInFile(task(path)).Changed)
		after, err := os.ReadFile(path)
		require.NoError(t, err)

		tr := LineInFile(task(path))
		require.Equal(t, Success, tr.Status, tr.Output)
		require.False(t, tr.Changed)

		again, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, string(after), string(again))
	})

	t.Run("drops duplicates left by the append-only era", func(t *testing.T) {
		path := seedFile(t, "pm.max_children = 5\nother = 1\npm.max_children = 40\n")

		tr := LineInFile(task(path))
		require.True(t, tr.Changed, "the stale duplicate is a change")

		body, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "pm.max_children = 5\nother = 1\n", string(body))
	})

	t.Run("appends when nothing matches", func(t *testing.T) {
		path := seedFile(t, "; managed by whip\n")

		require.True(t, LineInFile(task(path)).Changed)

		body, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "; managed by whip\npm.max_children = 5\n", string(body))
	})

	t.Run("creates the file and writes the line", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "pool.d", "www.conf")

		require.True(t, LineInFile(task(path)).Changed)

		body, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "pm.max_children = 5\n", string(body))
	})

	t.Run("an invalid regexp is an error, not a silent append", func(t *testing.T) {
		path := seedFile(t, seed)

		tr := LineInFile(lineInFileTask(model.TaskArgs{
			"path": path, "line": "a = 1", "regexp": "([",
		}))
		require.Equal(t, Failed, tr.Status)
		require.Contains(t, tr.Output, "regexp")

		body, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, seed, string(body))
	})
}

// Without a regexp the runner appends and nothing else, which is what an
// authorized_keys or a "one more line" playbook means. Existing playbooks rely
// on it, including on the duplicate a changed value leaves behind.
func TestLineInFileAppendOnlyIsUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "60-store.cnf")
	require.NoError(t, os.WriteFile(path, []byte("[mysqld]\nkey_buffer = 16M\n"), 0o644))

	add := func(line string) model.TaskResult {
		return LineInFile(lineInFileTask(model.TaskArgs{"path": path, "line": line}))
	}

	require.False(t, add("key_buffer = 16M").Changed, "the line is already there verbatim")

	require.True(t, add("max_connections = 100").Changed)
	require.False(t, add("max_connections = 100").Changed)

	// A changed value appends a second line: unchanged behaviour, and the
	// reason regexp exists.
	require.True(t, add("key_buffer = 32M").Changed)

	body, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t,
		"[mysqld]\nkey_buffer = 16M\nmax_connections = 100\nkey_buffer = 32M\n",
		string(body))
}

func TestArgBool(t *testing.T) {
	for _, tc := range []struct {
		in      any
		want    bool
		wantErr bool
	}{
		{in: true, want: true},
		{in: false, want: false},
		{in: "true", want: true},
		{in: "False", want: false},
		{in: "yes", want: true},
		{in: "no", want: false},
		{in: " off ", want: false},
		{in: "maybe", wantErr: true},
		{in: 3, wantErr: true},
	} {
		got, err := argBool(tc.in)
		if tc.wantErr {
			require.Error(t, err, tc.in)
			continue
		}
		require.NoError(t, err, tc.in)
		require.Equal(t, tc.want, got, tc.in)
	}
}

func TestArgMode(t *testing.T) {
	for _, tc := range []struct {
		in      any
		want    os.FileMode
		wantErr bool
	}{
		// YAML reads a leading zero as octal, so an int arrives resolved.
		{in: 0o644, want: 0o644},
		{in: "0644", want: 0o644},
		{in: "600", want: 0o600},
		{in: "0o644", wantErr: true},
		{in: "rwxr-xr-x", wantErr: true},
		{in: -1, wantErr: true},
		{in: 0o10000, wantErr: true},
	} {
		got, err := argMode(tc.in)
		if tc.wantErr {
			require.Error(t, err, tc.in)
			continue
		}
		require.NoError(t, err, tc.in)
		require.Equal(t, tc.want, got, tc.in)
	}
}
