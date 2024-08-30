package fsutil

import (
	"os"
	"path/filepath"
)

// FindAncestorPath searches for a file or directory with the given name (s)
// in the current directory and all parent directories.
// It returns the full path if found, or an empty string if not found.
func FindAncestorPath(s string) string {
	currentDir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		fullPath := filepath.Join(currentDir, s)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}

		// Move to the parent directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// We've reached the root directory
			break
		}
		currentDir = parentDir
	}

	return ""
}
