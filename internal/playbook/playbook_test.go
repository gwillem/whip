package playbook

import (
	"testing"

	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/model"
	tu "github.com/gwillem/whip/internal/testutil"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_LoadPlaybookSimple1(t *testing.T) {
	pb, err := Load(tu.FixturePath("playbook/simple.yml"))
	assert.NoError(t, err)
	want := &model.Playbook{
		model.Play{
			Hosts: []model.TargetName{
				"ubuntu@192.168.64.10",
			},
			Tasks: []model.Task{
				{
					Runner: "shell",
					Name:   "sleep random",
					Args: model.TaskArgs{
						"_args": "sleep $[ $RANDOM % 3 ]",
					},
					Notify: []string{"nginx", "systemd"},
					Loop:   nil,
				},
				{
					Runner: "command",
					Args: model.TaskArgs{
						"_args":  "update-locale a=b",
						"unless": "echo $LANG | grep C.UTF-8",
					},
				},
			},
			Handlers: []model.Task{
				{
					Runner: "command",
					Name:   "nginx",
					Args: model.TaskArgs{
						"_args": "echo restarting nginx",
					},
					Loop: nil,
				},
			},
		},
	}

	assert.Equal(t, want, pb)

	assert.Len(t, *pb, 1)
}

func Test_ExpandTaskLoops(t *testing.T) {
	defer log.Silence(log.Silence(true))
	pb, err := Load(tu.FixturePath("playbook/task_loop.yml"))
	assert.NoError(t, err)
	want := &model.Playbook{
		model.Play{
			Hosts: []model.TargetName{
				"ubuntu@192.168.64.16",
			},
			Tasks: []model.Task{
				{
					Runner: "command",
					Name:   "install ssh keys",
					Args: model.TaskArgs{
						"key":  "{{item}}",
						"user": "ubuntu",
					},
					Vars: map[string]any{
						"item": "abc",
					},
					Tags:   []string{},
					Notify: []string{},
				},
				{
					Runner: "command",
					Name:   "install ssh keys",
					Args: model.TaskArgs{
						"key":  "{{item}}",
						"user": "ubuntu",
					},
					Vars: map[string]any{
						"item": "xyz",
					},
					Tags:   []string{},
					Notify: []string{},
				},
			},
		},
	}
	assert.Equal(t, want, pb)
}

func Test_FilesWithMeta(t *testing.T) {
	_, err := Load(tu.FixturePath("playbook/tree.yml"))
	require.NoError(t, err)
}

func Test_DuplicateRunner(t *testing.T) {
	_, err := Load(tu.FixturePath("playbook/duplicate_runner.yml"))
	require.Error(t, err)
	var e *mapstructure.Error
	assert.ErrorAs(t, err, &e)
}

func Test_TaskArgList(t *testing.T) {
	pb, err := Load(tu.FixturePath("playbook/apt.yml"))
	assert.NoError(t, err)

	task := (*pb)[0].Tasks[0]

	assert.ElementsMatch(t, task.Args.StringSlice("name"), []string{"gunicorn", "nginx"})
}

func Test_TaskArgs(t *testing.T) {
	defer log.Silence(log.Silence(true))

	pb, err := Load(tu.FixturePath("playbook/task_args.yml"))
	require.NoError(t, err)
	play := (*pb)[0]
	require.Equal(t, "/bin/true", play.Tasks[0].Unless)
	require.Equal(t, "echo hi", play.Tasks[0].Args.String("_args"))
}
