package assets

import (
	"io"
	"os"
	"path/filepath"

	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/vault"
	"github.com/spf13/afero"
)

func DirToAsset(root string) (*model.Asset, error) {
	asset := model.Asset{Name: root}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath := path[len(root):]
		var data []byte

		if !info.IsDir() {
			data, err = vault.ReadFile(path)
			if err != nil {
				return err
			}
		}
		asset.Files = append(asset.Files, model.File{Path: relPath, Data: data, Mode: info.Mode()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

func AssetToFS(asset *model.Asset) (afero.Fs, error) {
	fs := afero.NewMemMapFs()
	for _, f := range asset.Files {
		fh, err := fs.OpenFile(f.Path, os.O_CREATE|os.O_WRONLY, f.Mode) // f.Mode
		if err != nil {
			return nil, err
		}
		defer fh.Close()
		if _, err := fh.Write(f.Data); err != nil {
			return nil, err
		}
		fh.Close()

		if _, err := fs.Stat(f.Path); err == nil {
			// log.Debug("Wrote", f.Path, "to fs, mode", fi.Mode())
		}

	}
	return fs, nil
}

type ReadCounter struct {
	r io.Reader
	n int64
}

func NewReadCounter(r io.Reader) *ReadCounter {
	return &ReadCounter{r: r}
}

func (rc *ReadCounter) Read(p []byte) (n int, err error) {
	n, err = rc.r.Read(p)
	rc.n += int64(n)
	return
}

func (rc *ReadCounter) Count() int64 {
	return rc.n
}
