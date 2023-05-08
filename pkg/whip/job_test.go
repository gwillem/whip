package whip

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func dummyJob() *Job {

	return &Job{
		Vars: Vars{
			"foo": "bar",
		},
		Tasks: []Task{
			{
				Name: "foo",
				Type: "command",
				Args: map[string]string{
					"cmd": "date",
				},
			},
		},
		Assets: []Asset{
			DirToAsset(FixturePath("assets/sample")),
		},
	}
}

func Test_JobFixture(t *testing.T) {
	job := dummyJob()
	fmt.Println(job)

	blob, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if e := os.WriteFile(FixturePath("job.json"), blob, 0644); e != nil {
		t.Fatal(e)
	}

}
