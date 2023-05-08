package whip

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// define Job struct
type (
	Job struct {
		Vars   Vars    `json:"vars,omitempty"`
		Tasks  []Task  `json:"tasks,omitempty"`
		Assets []Asset `json:"assets,omitempty"`
	}

	Vars map[string]any

	Task struct {
		Type string   `json:"type,omitempty"`
		Name string   `json:"name,omitempty"`
		Args TaskArgs `json:"args,omitempty"`
	}

	TaskArgs map[string]string

	Asset struct {
		Name  string `json:"name,omitempty"`
		Files []File `json:"files,omitempty"`
	}
	File struct {
		Path string `json:"path,omitempty"`
		Data []byte `json:"data,omitempty"`
	}

	TaskResult struct {
		Changed  bool          `json:"changed,omitempty"`
		Output   string        `json:"output,omitempty"`
		Status   int           `json:"status_code,omitempty"`
		Duration time.Duration `json:"duration,omitempty"`
	}

	// to help with parsing yaml
	RawPlaybook []RawPlay
	RawPlay     struct {
		Hosts    string   `yaml:"hosts,omitempty,flow"`
		RawTasks []AnyMap `yaml:"tasks,omitempty"`
	}
	AnyMap map[string]any

	Playbook []Play
	Play     struct {
		Hosts string
		Tasks []Task
	}
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
