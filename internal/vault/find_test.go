package vault

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func copyTestdata(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", src))
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0o755))
	require.NoError(t, os.WriteFile(dst, data, 0o600))
}

func TestFindEncrypted(t *testing.T) {
	root := t.TempDir()
	copyTestdata(t, "sample.age", filepath.Join(root, "etc/app/config.toml"))
	copyTestdata(t, "sample.ansible", filepath.Join(root, "etc/legacy.yml"))
	copyTestdata(t, "plaintext", filepath.Join(root, "etc/motd"))
	require.NoError(t, os.WriteFile(filepath.Join(root, "empty"), nil, 0o600))

	got, err := FindEncrypted(root)
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(root, "etc/app/config.toml"),
		filepath.Join(root, "etc/legacy.yml"),
	}, got)
}

func TestFindEncryptedNone(t *testing.T) {
	root := t.TempDir()
	copyTestdata(t, "plaintext", filepath.Join(root, "motd"))
	got, err := FindEncrypted(root)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestFindEncryptedMissingRoot(t *testing.T) {
	_, err := FindEncrypted(filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
}
