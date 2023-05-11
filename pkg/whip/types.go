package whip

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/barkimedes/go-deepcopy"
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
		Type  string   `json:"type,omitempty"`
		Name  string   `json:"name,omitempty"`
		Args  TaskArgs `json:"args,omitempty"`
		Items any      `json:"items,omitempty"`
	}

	TaskArgs map[string]any

	Asset struct {
		Name  string `json:"name,omitempty"`
		Files []File `json:"files,omitempty"`
	}
	File struct {
		Path string `json:"path,omitempty"`
		Data []byte `json:"data,omitempty"`
	}

	TaskResult struct {
		TaskID    int           `json:"task_id,omitempty"`
		TaskTotal int           `json:"task_total,omitempty"`
		Host      Host          `json:"target,omitempty"`
		Changed   bool          `json:"changed,omitempty"`
		Output    string        `json:"output,omitempty"`
		Status    int           `json:"status_code,omitempty"`
		Duration  time.Duration `json:"duration,omitempty"`
		Task      Task          `json:"task,omitempty"`
	}

	// to help with parsing yaml
	RawPlaybook []RawPlay
	RawPlay     struct {
		Hosts    any      `yaml:"hosts,omitempty"`
		Targets  []string `yaml:"targets,omitempty"`
		RawTasks []AnyMap `yaml:"tasks,omitempty"`
	}
	AnyMap map[string]any

	Playbook []Play
	Play     struct {
		Hosts []Host
		// Targets []string
		Tasks []Task
	}
	Host string
)

func (j *Job) String() string {
	return fmt.Sprintf("Job: %d tasks, %d assets, %d vars", len(j.Tasks), len(j.Assets), len(j.Vars))
}

func (tr TaskResult) String() string {
	return fmt.Sprintf("TaskResult %s from %s (%.2f sec)", tr.Task.Type, tr.Host, tr.Duration.Seconds())
}

func (ta TaskArgs) Key(s string) string {
	return ta[s].(string)
}

func (t Task) Clone() Task {
	return deepcopy.MustAnything(t).(Task)
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
