package internal

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
				Args: []TaskArg{
					{
						Name: "foo",
						Val:  "bar",
					},
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
