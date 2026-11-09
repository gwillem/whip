package main

import (
	"fmt"
	"strings"

	"github.com/gwillem/whip/internal/fsutil"
	"github.com/gwillem/whip/internal/vault"
)

const defaultFilesPath = ".whip/files"

// editTarget returns the file to edit: arg when given, else the only
// encrypted file under .whip/files in the current or a parent directory.
func editTarget(arg string) (string, error) {
	if arg != "" {
		return arg, nil
	}
	root := fsutil.FindAncestorPath(defaultFilesPath)
	if root == "" {
		return "", fmt.Errorf("no file given and no %s found in current or parent directories", defaultFilesPath)
	}
	found, err := vault.FindEncrypted(root)
	if err != nil {
		return "", err
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("no file given and no encrypted files in %s", root)
	case 1:
		return found[0], nil
	}
	return "", fmt.Errorf("no file given and %d encrypted files in %s, pick one:\n  %s",
		len(found), root, strings.Join(found, "\n  "))
}
