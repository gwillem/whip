package runners

import (
	"fmt"
	"os"
	"os/user"
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func init() {
}

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
		"/a/b/c": fmt.Sprintf("umask=%o owner=%s group=%s notify=%s", testRootUser, testRootGroup, testHandlerC),
		"/a":     fmt.Sprintf("umask=%o owner=%s group=%s notify=%s", testUser, testGroup, testHandlerA),
		"/d":     fmt.Sprintf("umask=%o owner=%s group=%s", testUser, testGroup),
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
	var changed bool
	var err error

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
	defer os.Remove(fh.Name())

	changed, err = ensureFile(filesObj{
		path: testPath,
		data: []byte("hoi"),
		uid:  &testUID,
		gid:  &testGID,
	})
	require.NoError(t, err)
	require.True(t, changed)

	changed, err = ensurePath(filesObj{
		path: testPath,
		data: []byte("hoi"),
		uid:  &testUID,
		gid:  &testGID,
	})
	require.NoError(t, err)
	require.True(t, changed) // should be false, but there is no way for Afero to retrieve the uid of a memfs file

	changed, err = ensurePath(filesObj{
		path: testPath,
		data: []byte("hoi"),
		uid:  &testUID,
		gid:  &testGID,
	})
	require.NoError(t, err)
	require.True(t, changed)

	fi, err := fs.Stat(testPath)
	require.NoError(t, err)
	require.Equal(t, fi.Mode(), os.FileMode(0o600))
}
