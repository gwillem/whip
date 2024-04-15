package model

import (
	"encoding/gob"
	"fmt"
	"io/fs"
	"time"

	"github.com/barkimedes/go-deepcopy"
	"github.com/charmbracelet/log"
	"github.com/mitchellh/mapstructure"
)

type (
	Job struct {
		Vars     Vars     `json:"vars,omitempty"`
		Playbook Playbook `json:"playbook,omitempty"`
		Assets   *Asset   `json:"assets,omitempty"`
	}

	Vars map[string]any

	Asset struct {
		Name  string `json:"name,omitempty"`
		Files []File `json:"files,omitempty"`
	}
	File struct {
		Path string      `json:"path,omitempty"`
		Data []byte      `json:"data,omitempty"`
		Mode fs.FileMode `json:"mode,omitempty"`
	}
	Playbook []Play
	Play     struct {
		Name      string         `json:"name,omitempty"`
		AssetPath string         `json:"assets,omitempty"`
		Hosts     []TargetName   `json:"hosts,omitempty"`
		Vars      map[string]any `json:"vars,omitempty"`
		Tasks     []Task         `json:"tasks,omitempty"`
		Handlers  []Task         `json:"handlers,omitempty"`
	}
	TargetName string
	Target     struct {
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
		Notify []string       `json:"notify,omitempty"`
		Loop   []any          `json:"loop,omitempty"`
		Vars   map[string]any `json:"vars,omitempty"`
		Tags   []string       `json:"tags,omitempty"`
	}

	TaskArgs map[string]any

	TaskResult struct {
		PlayIdx   int           `json:"play_idx,omitempty"`
		TaskIdx   int           `json:"task_idx,omitempty"` // TODO ugly, should refactor
		TaskTotal int           `json:"task_total,omitempty"`
		Host      string        `json:"target,omitempty"`
		Changed   bool          `json:"changed,omitempty"`
		Output    string        `json:"output,omitempty"`
		Status    int           `json:"status_code,omitempty"`
		Duration  time.Duration `json:"duration,omitempty"`
		Task      Task          `json:"task,omitempty"`
	}
)

func init() {
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
}

func (j *Job) Tasks() []Task {
	tasks := []Task{}
	for _, play := range j.Playbook {
		tasks = append(tasks, play.Tasks...)
		tasks = append(tasks, play.Handlers...)
	}
	return tasks
}

func (j *Job) String() string {
	return fmt.Sprintf("Job: %d tasks, %d assets, %d vars", len(j.Tasks()), len(j.Assets.Files), len(j.Vars))
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
