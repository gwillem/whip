package whip

import (
	"testing"

	"github.com/gwillem/chief-whip/pkg/runners"
	"github.com/k0kubun/pp"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
)

func TestLoadPlaybookSimple1(t *testing.T) {
	pb, err := LoadPlaybook(FixturePath("playbook/simple1.yml"))
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

func TestLoadPlaybookSimple2(t *testing.T) {
	pb, err := LoadPlaybook(FixturePath("playbook/simple2.yml"))
	assert.NoError(t, err)
	assert.NotNil(t, pb)
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
					Loop: []interface{}{
						"abc",
						"xyz",
					},
				},
			},
		},
	}
	assert.Equal(t, want, pb)
}

func TestDuplicateRunner(t *testing.T) {
	_, err := LoadPlaybook(FixturePath("playbook/duplicate_runner.yml"))
	pp.Print(err)
	var e *mapstructure.Error
	assert.ErrorAs(t, err, &e)
}
