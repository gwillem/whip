package playbook

import (
	"testing"
)

func Test_AferoMemFsSerialization(t *testing.T) {
	// fs := afero.NewMemMapFs()
	// fs.Create("testfile")
	// fs.Mkdir("testdir", 0o755)
	// fs.Mkdir("testdir/testsubdir", 0o755)
	// fs.Create("testdir/testfile")
	// fs.Create("testdir/testsubdir/testfile")

	// fs.(*afero.MemMapFs).List()

	// cannot serialize because unexported
	// var buffer bytes.Buffer
	// assert.NoError(t, gob.NewEncoder(&buffer).Encode(fs))
	// fmt.Println(buffer.Len())
}
