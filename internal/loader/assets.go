package loader

import (
	"os"
	"path/filepath"

	"github.com/gwillem/whip/internal/model"
)

func DirToAsset(root string) model.Asset {
	asset := model.Asset{Name: root}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		relPath := path[len(root):]

		asset.Files = append(asset.Files, model.File{Path: relPath, Data: data})
		return nil
	})
	if err != nil {
		panic(err)
	}
	return asset
}
