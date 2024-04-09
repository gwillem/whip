package model

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/barkimedes/go-deepcopy"
	"github.com/charmbracelet/log"
	"github.com/mitchellh/mapstructure"
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
		Tasks []Task         `json:"tasks,omitempty"`
	}
	Host string

	Target struct {
		User string
		Host string
		Port int
		Tag  string
	}
	Inventory []Target

	Task struct {
		Runner string         `json:"runner,omitempty"`
		Name   string         `json:"name,omitempty"`
		Args   TaskArgs       `json:"args,omitempty"`
		Loop   []any          `json:"loop,omitempty"`
		Vars   map[string]any `json:"vars,omitempty"`
	}

	TaskArgs map[string]any

	TaskResult struct {
		PlayIdx   int           `json:"play_idx,omitempty"`
		TaskIdx   int           `json:"task_idx,omitempty"`
		TaskTotal int           `json:"task_total,omitempty"`
		Host      string        `json:"target,omitempty"`
		Changed   bool          `json:"changed,omitempty"`
		Output    string        `json:"output,omitempty"`
		Status    int           `json:"status_code,omitempty"`
		Duration  time.Duration `json:"duration,omitempty"`
		Task      Task          `json:"task,omitempty"`
	}
)

func (j *Job) Tasks() []Task {
	tasks := []Task{}
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

func (tr TaskResult) String() string {
	return fmt.Sprintf("TaskResult %s from %s (%.2f sec)", tr.Task.Runner, tr.Host, tr.Duration.Seconds())
}

func (ta TaskArgs) String(s string) string {
	return ta[s].(string)
}

func (ta TaskArgs) StringSlice(s string) []string {
	switch ta[s].(type) {
	case string:
		return []string{ta[s].(string)}
	default:
		out := []string{}
		if err := mapstructure.Decode(ta[s], &out); err != nil {
			log.Error(err)
			return []string{}
		}
		return out
	}
}

func (t Task) Clone() Task {
	return deepcopy.MustAnything(t).(Task)
}
