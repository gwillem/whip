package playbook

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/testutil"
)

func dummyJob() *model.Job {
	return &model.Job{
		Vars: model.Vars{
			"foo": "bar",
		},
		Playbook: []model.Play{{
			Name: "dummy play",
			Tasks: []model.Task{{
				Name:   "foo",
				Runner: "command",
				Args:   model.TaskArgs{"cmd": "date"},
			}},
		}},
		Assets: []model.Asset{
			DirToAsset(testutil.FixturePath("assets/sample")),
		},
	}
}

func Test_JobFixture(t *testing.T) {
	job := dummyJob()
	// fmt.Println(job)

	blob, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if e := os.WriteFile(testutil.FixturePath("job.json"), blob, 0o644); e != nil {
		t.Fatal(e)
	}
}
