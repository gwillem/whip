package vault

import (
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
)

// FindEncrypted returns the regular files under root that start with the
// magic header of a supported vault, in lexical order.
func FindEncrypted(root string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(root, func(path string, d iofs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		enc, err := hasVaultMagic(path)
		if err != nil {
			return err
		}
		if enc {
			found = append(found, path)
		}
		return nil
	})
	return found, err
}

func hasVaultMagic(path string) (bool, error) {
	fh, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer fh.Close() //nolint:errcheck
	buf := make([]byte, magicSize)
	n, err := io.ReadFull(fh, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, err
	}
	_, err = findVaulter(buf[:n])
	return err == nil, nil
}
