package model

import (
	"encoding/json"
	"fmt"

	"github.com/gwillem/whip/internal/runners"
)

type (
	Job struct {
		Vars     Vars     `json:"vars,omitempty"`
		Playbook Playbook `json:"playbook,omitempty"`
		Assets   []Asset  `json:"assets,omitempty"`
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
		Name  string         `json:"name,omitempty"`
		Hosts []Host         `json:"hosts,omitempty"`
		Vars  map[string]any `json:"vars,omitempty"`
		Tasks []runners.Task `json:"tasks,omitempty"`
	}
	Host string

	Target struct {
		User string
		Host string
		Port int
		Tag  string
	}
	Inventory []Target
)

func (j *Job) Tasks() []runners.Task {
	tasks := []runners.Task{}
	for _, play := range j.Playbook {
		tasks = append(tasks, play.Tasks...)
	}
	return tasks
}

func (j *Job) String() string {
	return fmt.Sprintf("Job: %d tasks, %d assets, %d vars", len(j.Tasks()), len(j.Assets), len(j.Vars))
}

func (j *Job) ToJSON() ([]byte, error) {
	return json.Marshal(j)
}
