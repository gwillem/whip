package runners

import (
	"fmt"
	"testing"

	"github.com/gwillem/chief-whip/pkg/whip"
	"github.com/stretchr/testify/assert"
)

func TestAuthorizedKey(t *testing.T) {

	createTestFS()

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

	// _ = fsutil.Walk("/", func(path string, info os.FileInfo, err error) error {
	// 	fmt.Println(path)
	// 	return nil
	// })

	// assert that fake fs has now an SSH auth key
	assert.Equal(t, ok, tr.Status)

	authFile := "/var/root/.ssh/authorized_keys"

	// TODO mock the homedir func as well
	if ok, e := fsutil.Exists(authFile); e != nil || !ok {
		t.Fatal(e)
	}

	if ok, e := fsutil.FileContainsBytes(authFile, []byte("foo")); e != nil || !ok {
		t.Fatal(e)
	}
	if ok, e := fsutil.FileContainsBytes(authFile, []byte("bar")); e != nil || !ok {
		t.Fatal(e)
	}

	fmt.Println("task runner output:\n" + tr.Output)

}
