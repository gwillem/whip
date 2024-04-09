package playbook

import (
	"testing"

	"github.com/gwillem/whip/internal/model"
	tu "github.com/gwillem/whip/internal/testutil"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
)

func TestLoadPlaybookSimple1(t *testing.T) {
	pb, err := Load(tu.FixturePath("playbook/simple.yml"))
	assert.NoError(t, err)
	want := &model.Playbook{
		model.Play{
			Hosts: []model.Host{
				"ubuntu@192.168.64.10",
			},
			Tasks: []model.Task{
				{
					Runner: "shell",
					Name:   "sleep random",
					Args: model.TaskArgs{
						"_args": "sleep $[ $RANDOM % 3 ]",
					},
					Loop: nil,
				},
			},
		},
	}

	assert.Equal(t, want, pb)

	assert.Len(t, *pb, 1)
}

func TestExpandTaskLoops(t *testing.T) {
	pb, err := Load(tu.FixturePath("playbook/task-loop.yml"))
	assert.NoError(t, err)
	want := &model.Playbook{
		model.Play{
			Hosts: []model.Host{
				"ubuntu@192.168.64.10",
			},
			Tasks: []model.Task{
				{
					Runner: "authorized_key",
					Name:   "install ssh keys",
					Args: model.TaskArgs{
						"key":  "{{item}}",
						"user": "ubuntu",
					},
					Vars: map[string]any{
						"item": "abc",
					},
				},
				{
					Runner: "authorized_key",
					Name:   "install ssh keys",
					Args: model.TaskArgs{
						"key":  "{{item}}",
						"user": "ubuntu",
					},
					Vars: map[string]any{
						"item": "xyz",
					},
				},
			},
		},
	}
	assert.Equal(t, want, pb)
}

func TestDuplicateRunner(t *testing.T) {
	_, err := Load(tu.FixturePath("playbook/duplicate_runner.yml"))
	var e *mapstructure.Error
	assert.ErrorAs(t, err, &e)
}

func TestTaskArgList(t *testing.T) {
	pb, err := Load(tu.FixturePath("playbook/apt.yml"))
	assert.NoError(t, err)

	task := (*pb)[0].Tasks[0]

	assert.ElementsMatch(t, task.Args.StringSlice("name"), []string{"gunicorn", "nginx"})

}
