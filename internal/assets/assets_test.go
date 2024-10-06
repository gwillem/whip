package assets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirToAsset(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "dirasset_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create some test files and directories
	testFiles := map[string]string{
		"file1.txt":        "content of file1",
		"subdir/file2.txt": "content of file2",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0o755)
		assert.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0o644)
		assert.NoError(t, err)
	}

	// Call DirToAsset
	asset, err := DirToAsset(tempDir)
	assert.NoError(t, err)
	assert.NotNil(t, asset)

	// Check asset properties
	assert.Equal(t, tempDir, asset.Name)
	assert.Len(t, asset.Files, 3) // 2 files + 2 directories

	// Check file contents and properties
	for _, file := range asset.Files {
		switch file.Path {
		case string(filepath.Separator) + "file1.txt":
			assert.Equal(t, []byte("content of file1"), file.Data)
			assert.Equal(t, os.FileMode(0o666), file.Mode)
		case string(filepath.Separator) + "subdir":
			assert.Empty(t, file.Data)
			assert.True(t, file.Mode.IsDir())
		case filepath.Join(string(filepath.Separator)+"subdir", "file2.txt"):
			assert.Equal(t, []byte("content of file2"), file.Data)
			assert.Equal(t, os.FileMode(0o666), file.Mode)

		default:
			t.Errorf("Unexpected file: %s", file.Path)
		}
	}
}
