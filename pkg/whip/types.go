package whip

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gwillem/chief-whip/pkg/runners"
)

// define Job struct
type (
	Job struct {
		Vars   Vars           `json:"vars,omitempty"`
		Tasks  []runners.Task `json:"tasks,omitempty"`
		Assets []Asset        `json:"assets,omitempty"`
	}

	Vars map[string]any

	Asset struct {
		Name  string `json:"name,omitempty"`
		Files []File `json:"files,omitempty"`
	}
	File struct {
		Path string `json:"path,omitempty"`
		Data []byte `json:"data,omitempty"`
	}

	Playbook []Play
	Play     struct {
		Hosts []Host
		// Targets []string
		Tasks []runners.Task
	}
	Host string
)

func (j *Job) String() string {
	return fmt.Sprintf("Job: %d tasks, %d assets, %d vars", len(j.Tasks), len(j.Assets), len(j.Vars))
}

func DirToAsset(root string) Asset {
	asset := Asset{Name: root}
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

		asset.Files = append(asset.Files, File{Path: relPath, Data: data})
		return nil
	})
	if err != nil {
		panic(err)
	}
	return asset
}
