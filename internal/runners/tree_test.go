package runners

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/model"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

var (
	testUser  = "testuser"
	testGroup = "testgroup"
	testUID   = 1000
	testGID   = 2000

	testRootUser  = "root"
	testRootGroup = "sys"
	testRootUID   = 1
	testRootGID   = 2

	testHandlerA = "handlerA"
	testHandlerC = "handlerC"
)

func newFileMeta(uid, gid int, notify []string) *fileMeta {
	return &fileMeta{uid: &uid, gid: &gid, notify: notify}
}

func getDummyTaskArgs() model.TaskArgs {
	return model.TaskArgs{
		"/a/b/c": fmt.Sprintf("owner=%s group=%s notify=%s", testRootUser, testRootGroup, testHandlerC),
		"/a":     fmt.Sprintf("owner=%s group=%s notify=%s", testUser, testGroup, testHandlerA),
		"/d":     fmt.Sprintf("owner=%s group=%s", testUser, testGroup),
	}
}

func getDummyOsUser() OsUser {
	return stubOsUser{
		current: &user.User{
			Uid:      fmt.Sprintf("%d", testUID),
			Gid:      fmt.Sprintf("%d", testGID),
			Username: testUser,
			Name:     "Spooky",
			HomeDir:  "/home/spooky",
		},
		group: &user.Group{
			Gid:  fmt.Sprintf("%d", testGID),
			Name: testGroup,
		},
		userMap: map[string]*user.User{
			testUser: {
				Uid:      fmt.Sprintf("%d", testUID),
				Gid:      fmt.Sprintf("%d", testGID),
				Username: testUser,
			},
			testRootUser: {
				Uid:      fmt.Sprintf("%d", testRootUID),
				Gid:      fmt.Sprintf("%d", testRootGID),
				Username: testRootUser,
			},
		},
		groupMap: map[string]*user.Group{
			testGroup: {
				Gid:  fmt.Sprintf("%d", testGID),
				Name: testGroup,
			},
			testRootGroup: {
				Gid:  fmt.Sprintf("%d", testRootGID),
				Name: testRootGroup,
			},
		},
	}
}

func Test_parsePrefixMeta(t *testing.T) {
	osUser = getDummyOsUser()
	defer func() {
		osUser = realOsUser{}
	}()

	tests := []struct {
		name    string
		args    model.TaskArgs
		want    *prefixMetaMap
		wantErr bool
	}{
		{
			name: "Valid input with proper attributes",
			args: getDummyTaskArgs(),
			want: &prefixMetaMap{
				orderedPrefixes: []string{
					"/a",
					"/a/b/c",
					"/d",
				},
				metamap: map[string]fileMeta{
					"/a":     *newFileMeta(testUID, testGID, []string{testHandlerA}),
					"/a/b/c": *newFileMeta(testRootUID, testRootGID, []string{testHandlerC}),
					"/d":     *newFileMeta(testUID, testGID, nil),
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid umask value",
			args: model.TaskArgs{
				"/badUmask": map[string]string{
					"umask": "invalid",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePrefixMeta(tt.args)
			if !tt.wantErr {
				require.Equal(t, tt.want, got)
				return
			}
			require.Error(t, err)
		})
	}
}

func Test_prefixMetaMap_getMeta(t *testing.T) {
	osUser = getDummyOsUser()
	defer func() {
		osUser = realOsUser{}
	}()

	pmm, err := parsePrefixMeta(getDummyTaskArgs())
	require.NoError(t, err)

	var fm fileMeta

	fm = pmm.getMeta("/banaan")
	require.Equal(t, fileMeta{}, fm)
	require.Empty(t, fm.notify)

	fm = pmm.getMeta("/a/lsdjflsd")
	require.Equal(t, *newFileMeta(testUID, testGID, []string{testHandlerA}), fm)

	fm = pmm.getMeta("/a/b/c/d/e")
	require.Equal(t, *newFileMeta(testRootUID, testRootGID, []string{testHandlerA, testHandlerC}), fm)

	fm = pmm.getMeta("/d/lkkijfksdlsdf/dsfsdf")
	require.Equal(t, *newFileMeta(testUID, testGID, nil), fm)
}

func Test_ensurePathUpdatesFileMode(t *testing.T) {
	log.SetLevel(log.LevelError)
	defer log.SetLevel(log.LevelDebug)

	var changed bool
	var err error

	target := os.FileMode(0o631)

	oldFs := fs
	oldFsutil := fsutil
	defer func() {
		fs = oldFs
		fsutil = oldFsutil
	}()
	fs = afero.NewCopyOnWriteFs(afero.NewOsFs(), afero.NewMemMapFs())
	fsutil = &afero.Afero{Fs: fs}

	fh, err := fsutil.TempFile("", "tree_test")
	testPath := fh.Name()
	require.NoError(t, err)
	require.NoError(t, fh.Close())
	defer os.Remove(fh.Name()) //nolint:errcheck

	dummyID := 0

	testFile := filesObj{
		path: testPath,
		data: []byte("hoi"),
		mode: target,
		uid:  &dummyID,
		gid:  &dummyID,
	}

	changed, err = ensureFile(testFile)
	require.NoError(t, err)
	require.True(t, changed)

	changed, err = ensurePath(testFile)
	require.NoError(t, err)
	require.True(t, changed) // should be false, but there is no way for Afero to retrieve the uid of a memfs file

	fi, err := fs.Stat(testPath)
	require.NoError(t, err)
	require.Equal(t, fi.Mode(), target)
}

// writeTreeSrc lays out a source tree on disk, as a playbook's files/ dir.
func writeTreeSrc(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for path, data := range files {
		full := filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(data), 0o644))
	}
	return root
}

// runTree ships src to dst, mimicking the deputy: prerun loads the asset here,
// and gob flattens the pointer to a value on the way over the wire.
func runTree(t *testing.T, src, dst string, args model.TaskArgs, vars model.TaskVars) model.TaskResult {
	t.Helper()
	task := model.Task{Runner: "tree", Args: model.TaskArgs{"src": src, "dst": dst}, Vars: vars}
	for k, v := range args {
		task.Args[k] = v
	}
	require.Equal(t, Success, treePrerun(&task).Status)
	asset, ok := task.Args["_assets"].(*model.Asset)
	require.True(t, ok)
	task.Args["_assets"] = *asset
	return tree(&task)
}

func Test_treeTemplateOptOut(t *testing.T) {
	src := writeTreeSrc(t, map[string]string{
		"raw/verbatim.conf": "set braces { {{ this }} }\n",
		"tpl/rendered.conf": "greeting = {{ greeting }}\n",
	})
	dst := t.TempDir()

	tr := runTree(t, src, dst, model.TaskArgs{"/raw": "template=false"},
		model.TaskVars{"greeting": "hello"})
	require.Equal(t, Success, tr.Status, tr.Output)
	require.True(t, tr.Changed)

	raw, err := os.ReadFile(filepath.Join(dst, "raw/verbatim.conf"))
	require.NoError(t, err)
	require.Equal(t, "set braces { {{ this }} }\n", string(raw))

	rendered, err := os.ReadFile(filepath.Join(dst, "tpl/rendered.conf"))
	require.NoError(t, err)
	require.Equal(t, "greeting = hello\n", string(rendered))
}

func Test_treeTemplateOptOutIsPerPrefix(t *testing.T) {
	// A longer prefix switches rendering back on inside a verbatim tree.
	src := writeTreeSrc(t, map[string]string{
		"third-party/app.conf":     "${SHELL_VAR} {{ this }}\n",
		"third-party/own/own.conf": "greeting = {{ greeting }}\n",
	})
	dst := t.TempDir()

	tr := runTree(t, src, dst, model.TaskArgs{
		"/third-party":     "template=false",
		"/third-party/own": "template=true",
	}, model.TaskVars{"greeting": "hello"})
	require.Equal(t, Success, tr.Status, tr.Output)

	verbatim, err := os.ReadFile(filepath.Join(dst, "third-party/app.conf"))
	require.NoError(t, err)
	require.Equal(t, "${SHELL_VAR} {{ this }}\n", string(verbatim))

	rendered, err := os.ReadFile(filepath.Join(dst, "third-party/own/own.conf"))
	require.NoError(t, err)
	require.Equal(t, "greeting = hello\n", string(rendered))
}

func Test_treeStateAbsent(t *testing.T) {
	src := writeTreeSrc(t, map[string]string{"keep/keep.conf": "keep me\n"})
	dst := t.TempDir()

	stale := filepath.Join(dst, "decommissioned/stale.conf")
	require.NoError(t, os.MkdirAll(filepath.Dir(stale), 0o755))
	require.NoError(t, os.WriteFile(stale, []byte("old\n"), 0o644))

	args := model.TaskArgs{"/decommissioned": fmt.Sprintf("state=absent notify=%s", testHandlerA)}

	tr := runTree(t, src, dst, args, nil)
	require.Equal(t, Success, tr.Status, tr.Output)
	require.True(t, tr.Changed)
	require.True(t, tr.Notify[testHandlerA])
	require.NoFileExists(t, stale)
	require.NoDirExists(t, filepath.Join(dst, "decommissioned"))
	require.FileExists(t, filepath.Join(dst, "keep/keep.conf"))

	// nothing left to remove, so nothing to report
	tr = runTree(t, src, dst, args, nil)
	require.Equal(t, Success, tr.Status, tr.Output)
	require.False(t, tr.Changed)
	require.Empty(t, tr.Notify)
}

func Test_treeStateAbsentBeatsSource(t *testing.T) {
	// A source subtree under an absent prefix must not be shipped back, or the
	// tree would delete and recreate the same paths on every run.
	src := writeTreeSrc(t, map[string]string{"gone/leftover.conf": "old\n"})
	dst := t.TempDir()

	tr := runTree(t, src, dst, model.TaskArgs{"/gone": "state=absent"}, nil)
	require.Equal(t, Success, tr.Status, tr.Output)
	require.False(t, tr.Changed)
	require.NoDirExists(t, filepath.Join(dst, "gone"))
}

func Test_treeCreatesMissingDst(t *testing.T) {
	src := writeTreeSrc(t, map[string]string{"etc/app.conf": "hoi\n"})
	dst := filepath.Join(t.TempDir(), "srv", "honeypot")

	tr := runTree(t, src, dst, model.TaskArgs{"/": "umask=027"}, nil)
	require.Equal(t, Success, tr.Status, tr.Output)
	require.True(t, tr.Changed)

	fi, err := os.Stat(dst)
	require.NoError(t, err)
	require.True(t, fi.IsDir())
	require.Equal(t, os.FileMode(0o750), fi.Mode().Perm()) // 0777 &^ 027
	require.FileExists(t, filepath.Join(dst, "etc/app.conf"))
}

func Test_parsePrefixMetaTemplateAndState(t *testing.T) {
	tests := []struct {
		name       string
		args       model.TaskArgs
		wantErr    bool
		wantAbsent bool
		wantRender bool
		wantPrefix string
	}{
		{name: "template off", args: model.TaskArgs{"/a": "template=false"}, wantPrefix: "/a"},
		{name: "template on", args: model.TaskArgs{"/a": "template=true"}, wantPrefix: "/a", wantRender: true},
		{name: "template default", args: model.TaskArgs{"/a": "umask=022"}, wantPrefix: "/a", wantRender: true},
		{name: "state absent", args: model.TaskArgs{"/a": "state=absent"}, wantPrefix: "/a", wantAbsent: true, wantRender: true},
		{name: "state present", args: model.TaskArgs{"/a": "state=present"}, wantPrefix: "/a", wantRender: true},
		{name: "bad template", args: model.TaskArgs{"/a": "template=maybe"}, wantErr: true},
		{name: "bad state", args: model.TaskArgs{"/a": "state=liquid"}, wantErr: true},
		{name: "declaration inside absent", args: model.TaskArgs{
			"/a":   "state=absent",
			"/a/b": "umask=022",
		}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm, err := parsePrefixMeta(tt.args)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			meta := pm.getMeta(tt.wantPrefix + "/somefile")
			require.Equal(t, tt.wantAbsent, meta.absent)
			require.Equal(t, tt.wantRender, meta.templating())
		})
	}
}

// A tree's source is as much a candidate for a variable as anything else, and
// it is read on the controller, so PreRun has to render the arguments before
// the pre-run sees them. Without this the glob matched the literal string,
// found nothing, and the run failed later on the target with "no assets
// found" - a complaint about the destination, for a mistake made here.
func Test_preRunRendersTheTreeSource(t *testing.T) {
	root := writeTreeSrc(t, map[string]string{
		"apiarist/etc/store.conf": "name = apiarist\n",
		"nuclear/etc/store.conf":  "name = nuclear\n",
	})
	dst := t.TempDir()

	task := model.Task{
		Runner: "tree",
		Args:   model.TaskArgs{"src": filepath.Join(root, "{{ instance }}"), "dst": dst},
	}
	tr := PreRun(&task, model.TaskVars{"instance": "apiarist"})
	require.Equal(t, Success, tr.Status, tr.Output)

	// The rendered path is written back into the shared Args map, which is
	// also how the assets reach the job that is sent to the target.
	require.Equal(t, filepath.Join(root, "apiarist"), task.Args["src"])

	asset, ok := task.Args["_assets"].(*model.Asset)
	require.True(t, ok, "the pre-run attached no assets")
	var got string
	for _, f := range asset.Files {
		if f.Path == "/etc/store.conf" {
			got = string(f.Data)
		}
	}
	require.Equal(t, "name = apiarist\n", got,
		"the wrong instance's tree was loaded, or none was")
}

// A source naming a variable nobody defined must fail here, loudly, rather
// than reach the target with nothing attached. cmd/whip turns this status into
// a fatal; it used to be logged at debug and the run carried on.
func Test_preRunFailsOnAnUndefinedVariableInTheSource(t *testing.T) {
	task := model.Task{
		Runner: "tree",
		Args:   model.TaskArgs{"src": "/srv/seed/{{ nope }}", "dst": t.TempDir()},
	}
	tr := PreRun(&task, model.TaskVars{"instance": "apiarist"})
	require.Equal(t, Failed, tr.Status)
	require.Contains(t, tr.Output, "src")
}
