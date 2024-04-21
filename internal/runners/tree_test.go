package runners

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testUser  = getUser()
	testGroup = getGroup()
	testUID   = getUID(testUser)
	testGID   = getGID(testGroup)

	testRootUser  = "root"
	testRootGroup = "sys"
	testRootUID   = getUID(testRootUser)
	testRootGID   = getGID(testRootGroup)

	testUmask    = os.FileMode(0o22)
	testHandlerA = "handlerA"
	testHandlerC = "handlerC"
)

func getUser() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	return usr.Name
}

func getGroup() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	grp, err := user.LookupGroupId(usr.Gid)
	if err != nil {
		panic(err)
	}
	return grp.Name
}

func getUID(usr string) int {
	u, err := user.Lookup(usr)
	if err != nil {
		panic(err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		panic(err)
	}
	return uid
}

func getGID(grp string) int {
	g, err := user.LookupGroup(grp)
	if err != nil {
		panic(err)
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		panic(err)
	}
	return gid
}

func newFileMeta(uid, gid int, umask os.FileMode, notify []string) *fileMeta {
	return &fileMeta{uid: &uid, gid: &gid, umask: umask, notify: notify}
}

func getDummyTaskArgs() model.TaskArgs {
	return model.TaskArgs{
		"/a/b/c": fmt.Sprintf("umask=%o owner=%s group=%s notify=%s", defaultUmask, testRootUser, testRootGroup, testHandlerC),
		"/a":     fmt.Sprintf("umask=%o owner=%s group=%s notify=%s", defaultUmask, testUser, testGroup, testHandlerA),
		"/d":     fmt.Sprintf("umask=%o owner=%s group=%s", defaultUmask, testUser, testGroup),
	}
}

func Test_parsePrefixMeta(t *testing.T) {
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
					"/a":     *newFileMeta(testUID, testGID, testUmask, []string{testHandlerA}),
					"/a/b/c": *newFileMeta(testRootUID, testRootGID, testUmask, []string{testHandlerC}),
					"/d":     *newFileMeta(testUID, testGID, testUmask, nil),
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
	pmm, err := parsePrefixMeta(getDummyTaskArgs())
	assert.NoError(t, err)

	var fm fileMeta

	fm = pmm.getMeta("/banaan")
	assert.Equal(t, fileMeta{}, fm)
	assert.Empty(t, fm.notify)

	fm = pmm.getMeta("/a/lsdjflsd")
	assert.Equal(t, *newFileMeta(testUID, testGID, testUmask, []string{testHandlerA}), fm)

	fm = pmm.getMeta("/a/b/c/d/e")
	assert.Equal(t, *newFileMeta(getUID(testRootUser), getGID(testRootGroup), testUmask, []string{testHandlerA, testHandlerC}), fm)

	fm = pmm.getMeta("/d/lkkijfksdlsdf/dsfsdf")
	assert.Equal(t, *newFileMeta(testUID, testGID, testUmask, nil), fm)
}

func Test_ensurePathUpdatesFileMode(t *testing.T) {
	var changed bool
	var err error
	var dir string

	dir, err = os.MkdirTemp("", "test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	testPath := filepath.Join(dir, "hsdfjkjskdfsd")

	changed, err = ensureFile(filesObj{
		path:  testPath,
		data:  []byte("hoi"),
		umask: os.FileMode(0o22),
		uid:   &testUID,
		gid:   &testGID,
	})
	assert.NoError(t, err)
	assert.True(t, changed)

	changed, err = ensurePath(filesObj{
		path:  testPath,
		data:  []byte("hoi"),
		umask: os.FileMode(0o22),
		uid:   &testUID,
		gid:   &testGID,
	})
	assert.NoError(t, err)
	assert.False(t, changed)

	changed, err = ensurePath(filesObj{
		path:  testPath,
		data:  []byte("hoi"),
		umask: os.FileMode(0o77),
		uid:   &testUID,
		gid:   &testGID,
	})
	assert.NoError(t, err)
	assert.True(t, changed)

	fi, err := os.Stat(testPath)
	assert.NoError(t, err)
	assert.Equal(t, fi.Mode(), os.FileMode(0o600))
}
