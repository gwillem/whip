package playbook

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/testutil"
)

func dummyJob() *model.Job {
	// asset, _ := assets.DirToAsset(testutil.FixturePath("assets/sample"))
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
		// Assets: asset,
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

	var job2 model.Job
	blob, err = os.ReadFile(testutil.FixturePath("job.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(blob, &job2); err != nil {
		t.Fatal(err)
	}
}
