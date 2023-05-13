package whip

import (
	"testing"

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
			Tasks: []Task{
				{
					Runner: "shell",
					Name:   "sleep random",
					Args: TaskArgs{
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
			Tasks: []Task{
				{
					Runner: "authorized_key",
					Name:   "install ssh keys",
					Args: TaskArgs{
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
