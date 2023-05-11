package runners

import (
	"fmt"
	"testing"

	"github.com/gwillem/chief-whip/pkg/whip"
)

func TestAuthorizedKey(t *testing.T) {
	task := whip.Task{
		Name: "blabla",
		Args: whip.TaskArgs{
			"key":  `{{item}}`,
			"user": "root",
		},
		Items: []string{"foo", "bar"},
		Type:  "authorized_key",
	}

	tr := Run(task)

	fmt.Println("task runner output:\n" + tr.Output)

}
