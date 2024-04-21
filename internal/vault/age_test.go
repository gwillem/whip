package vault

import (
	"io"
	"os"
	"testing"

	"github.com/gwillem/whip/internal/testutil"
	"github.com/stretchr/testify/require"
)

const ageTestKey = "AGE-SECRET-KEY-1ATU93PUH73GSD6UXHVU4GYQ2JKM5SJ0SNUH8UWPGCQ0HWYUEL5WQRVYT4V"

func Test_OpenRegular(t *testing.T) {
	fh, err := Open(testutil.FixturePath("vault/non-secret"))
	require.NoError(t, err)
	data, err := io.ReadAll(fh)
	require.NoError(t, err)
	require.Equal(t, "boe\n", string(data))
	fh.Close()
}

func Test_OpenAge(t *testing.T) {
	fh, err := Open(testutil.FixturePath("vault/sample.age"))
	require.NoError(t, err)
	data, err := io.ReadAll(fh)
	require.NoError(t, err)
	require.Equal(t, "hoi\n", string(data))
	fh.Close()
}

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
