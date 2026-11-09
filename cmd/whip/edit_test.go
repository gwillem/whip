package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeVaultFile(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile("../../internal/vault/testdata/sample.age")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

func TestEditTargetExplicit(t *testing.T) {
	got, err := editTarget("some/file")
	require.NoError(t, err)
	require.Equal(t, "some/file", got)
}

func TestEditTargetSingle(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, ".whip/files/etc/app/config.toml")
	writeVaultFile(t, want)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".whip/files/etc/plain"), []byte("x"), 0o600))
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))
	t.Chdir(sub)

	got, err := editTarget("")
	require.NoError(t, err)
	gotReal, _ := filepath.EvalSymlinks(got)
	wantReal, _ := filepath.EvalSymlinks(want)
	require.Equal(t, wantReal, gotReal)
}

func TestEditTargetMultiple(t *testing.T) {
	root := t.TempDir()
	writeVaultFile(t, filepath.Join(root, ".whip/files/a"))
	writeVaultFile(t, filepath.Join(root, ".whip/files/b"))
	t.Chdir(root)

	_, err := editTarget("")
	require.ErrorContains(t, err, "2 encrypted files")
	require.ErrorContains(t, err, ".whip/files/a")
	require.ErrorContains(t, err, ".whip/files/b")
}

func TestEditTargetNone(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".whip/files"), 0o755))
	t.Chdir(root)
	_, err := editTarget("")
	require.ErrorContains(t, err, "no encrypted files")
}

func TestEditTargetNoTree(t *testing.T) {
	t.Chdir(t.TempDir())
	_, err := editTarget("")
	require.ErrorContains(t, err, ".whip/files")
}
