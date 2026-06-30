package vault

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const ageTestKey = "AGE-SECRET-KEY-1ATU93PUH73GSD6UXHVU4GYQ2JKM5SJ0SNUH8UWPGCQ0HWYUEL5WQRVYT4V"

var ageTestVault = &ageVault{}

func init() {
	_ = ageTestVault.loadKey(ageTestKey)
}

func Test_OpenRegular(t *testing.T) {
	fh, err := Open("testdata/plaintext")
	require.NoError(t, err)
	data, err := io.ReadAll(fh)
	require.NoError(t, err)
	require.Equal(t, "boe\n", string(data))
	_ = fh.Close()
}

func Test_OpenAge(t *testing.T) {
	require.NoError(t, ageTestVault.loadKey(ageTestKey))
	oldAll := allVaulters
	defer func() {
		allVaulters = oldAll
	}()
	allVaulters = []Vaulter{ageTestVault}
	fh, err := Open("testdata/sample.age")
	require.NoError(t, err)
	data, err := io.ReadAll(fh)
	require.NoError(t, err)
	require.Equal(t, "hoi\n", string(data))
	_ = fh.Close()
}

func Test_OpenNonExistingFile(t *testing.T) {
	fh, err := Open("/doesnotexist_92387892348")
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
	if fh != nil {
		_ = fh.Close()
	}
}

func Test_OpenEmptyFile(t *testing.T) {
	tmp, err := os.CreateTemp("", "dlsfsd")
	require.NoError(t, err)
	_ = tmp.Close()
	defer os.Remove(tmp.Name()) //nolint:errcheck

	fh, err := Open(tmp.Name())
	require.NoError(t, err)
	data, err := io.ReadAll(fh)
	require.NoError(t, err)
	require.Empty(t, data)
	_ = fh.Close()
}

func Test_readFromScript(t *testing.T) {
	got, err := readFromScript("bogus script")
	require.Empty(t, got)
	require.Error(t, err)
	require.Contains(t, err.Error(), "bogus script")
}

func Test_readFromScript_nonZeroExit(t *testing.T) {
	// Create a temporary script that fails with an error message
	script, err := os.CreateTemp("", "test-secret-*.sh")
	require.NoError(t, err)
	defer os.Remove(script.Name()) //nolint:errcheck

	_, err = script.WriteString("#!/bin/sh\necho 'custom error message' >&2\nexit 1\n")
	require.NoError(t, err)
	_ = script.Close()

	require.NoError(t, os.Chmod(script.Name(), 0o755))

	got, err := readFromScript(script.Name())
	require.Empty(t, got)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exit status 1")
	require.Contains(t, err.Error(), "custom error message")
}
