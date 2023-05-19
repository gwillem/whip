package runners

import (
	"fmt"
	"os"
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestAuthorizedKey(t *testing.T) {
	createTestFS()

	task := model.Task{
		Name: "blabla",
		Args: model.TaskArgs{
			"key":  `{{item}}`,
			"user": "root",
		},
		Loop:   []any{"foo", "bar"},
		Runner: "authorized_key",
	}

	tr := Run(task, nil)
	_ = Run(task, nil)

	_ = fsutil.Walk("/", func(path string, info os.FileInfo, err error) error {
		fmt.Println(path)
		return nil
	})

	fmt.Println(tr.Output)

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

	data, err := fsutil.ReadFile(authFile)
	assert.NoError(t, err)

	fmt.Print(string(data))

	fmt.Println("task runner output:\n" + tr.Output)
}
