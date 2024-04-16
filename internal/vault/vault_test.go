package vault

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/gwillem/whip/internal/testutil"
	"github.com/stretchr/testify/require"
)

const testKey = "AGE-SECRET-KEY-1KPE56S4T0723PUFNH3CP6Z9JYSW9SRLM4C876TX9R5GZX8T3J5YSLCE90H"

// func Test_OpenGPG(t *testing.T) {
// 	fh, err := Open(testutil.FixturePath("vault/secret"))
// 	require.NoError(t, err)
// 	data, err := io.ReadAll(fh)
// 	require.NoError(t, err)
// 	require.Equal(t, "hoi\n", string(data))
// 	fh.Close()
// }

func Test_OpenRegular(t *testing.T) {
	fh, err := Open(testutil.FixturePath("vault/non-secret"))
	require.NoError(t, err)
	data, err := io.ReadAll(fh)
	require.NoError(t, err)
	require.Equal(t, "boe\n", string(data))
	fh.Close()
}

func Test_OpenAge(t *testing.T) {
	fh, err := Open(testutil.FixturePath("vault/sample-pki.age"))
	require.NoError(t, err)
	data, err := io.ReadAll(fh)
	require.NoError(t, err)
	require.Equal(t, "hoi\n", string(data))
	fh.Close()
}

func Test_newId(t *testing.T) {
	// id, err := age.GenerateX25519Identity()
	// require.NoError(t, err)
	// fmt.Println(id)
	_, err := getID()
	require.NoError(t, err)
	if err != nil {
		fmt.Println(err)
	}
}

// func Test_Editor(t *testing.T) {
// 	old := os.Getenv("EDITOR")
// 	os.Unsetenv("EDITOR")
// 	defer os.Setenv("EDITOR", old)
// 	require.NoError(t, loadID(testKey))
// 	require.NoError(t, Edit("/tmp/testfile"))
// }

func Test_OpenNonExistingFile(t *testing.T) {
	fh, err := Open("/doesnotexist_92387892348")
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
	if fh != nil {
		fh.Close()
	}
}

func Test_OpenEmptyFile(t *testing.T) {
	tmp, err := os.CreateTemp("", "dlsfsd")
	require.NoError(t, err)
	tmp.Close()
	defer os.Remove(tmp.Name())

	fh, err := Open(tmp.Name())
	require.NoError(t, err)
	data, err := io.ReadAll(fh)
	require.NoError(t, err)
	require.Empty(t, data)
	fh.Close()
}
