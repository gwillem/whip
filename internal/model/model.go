package model

import (
	"encoding/gob"
	"fmt"
	"io/fs"
	"time"

	"github.com/barkimedes/go-deepcopy"
	log "github.com/gwillem/go-simplelog"
	"github.com/mitchellh/mapstructure"
)

type (
	Job struct {
		Vars     Vars     `json:"vars,omitempty"`
		Playbook Playbook `json:"playbook,omitempty"`
		// Assets   *Asset   `json:"assets,omitempty"`
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
		Runner string   `json:"runner,omitempty"`
		Name   string   `json:"name,omitempty"`
		Args   TaskArgs `json:"args,omitempty"`
		Notify []string `json:"notify,omitempty"`
		Loop   []any    `json:"loop,omitempty"`
		Vars   TaskVars `json:"vars,omitempty"`
		Tags   []string `json:"tags,omitempty"`
	}

	TaskArgs map[string]any
	TaskVars map[string]any

	TaskResult struct {
		Host     TargetName      `json:"target,omitempty"`
		Changed  bool            `json:"changed,omitempty"`
		Output   string          `json:"output,omitempty"`
		Status   int             `json:"status_code,omitempty"`
		Duration time.Duration   `json:"duration,omitempty"`
		Task     *Task           `json:"task,omitempty"`
		Notify   map[string]bool `json:"notify,omitempty"`
	}
	ReportMsg struct {
		TaskIdx    int
		TaskTotal  int
		TaskResult TaskResult
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
	return fmt.Sprintf("Job: %d tasks, %d vars", len(j.Tasks()), len(j.Vars))
}

func (tr TaskResult) String() string {
	runner := ""
	if tr.Task != nil {
		runner = tr.Task.Runner
	}

	return fmt.Sprintf("TaskResult %s from %s (%.2f sec) -- %s:%d", runner, tr.Host, tr.Duration.Seconds())
}

func (ta TaskArgs) String(s string) string {
	if arg := ta[s]; arg != nil {
		return arg.(string)
	}
	return ""
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
