package whip

import (
	"testing"

	"github.com/gwillem/chief-whip/pkg/runners"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
)

func TestLoadPlaybookSimple1(t *testing.T) {
	pb, err := LoadPlaybook(FixturePath("playbook/simple.yml"))
	assert.NoError(t, err)
	want := &Playbook{
		Play{
			Hosts: []Host{
				"ubuntu@192.168.64.10",
			},
			Tasks: []runners.Task{
				{
					Runner: "shell",
					Name:   "sleep random",
					Args: runners.TaskArgs{
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
	pb, err := LoadPlaybook(FixturePath("playbook/task-loop.yml"))
	assert.NoError(t, err)
	want := &Playbook{
		Play{
			Hosts: []Host{
				"ubuntu@192.168.64.10",
			},
			Tasks: []runners.Task{
				{
					Runner: "authorized_key",
					Name:   "install ssh keys",
					Args: runners.TaskArgs{
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
					Args: runners.TaskArgs{
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
	_, err := LoadPlaybook(FixturePath("playbook/duplicate_runner.yml"))
	var e *mapstructure.Error
	assert.ErrorAs(t, err, &e)
}
