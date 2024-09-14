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
		if relPath == "" {
			return nil
		}
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
		if f.Mode.IsDir() {
			if err := fs.MkdirAll(f.Path, f.Mode); err != nil {
				return nil, err
			}
			continue
		}
		fh, err := fs.OpenFile(f.Path, os.O_CREATE|os.O_WRONLY, f.Mode)
		if err != nil {
			return nil, err
		}
		defer fh.Close()
		_, err = fh.Write(f.Data)
		if err != nil {
			return nil, err
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
