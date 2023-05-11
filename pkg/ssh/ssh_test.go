package ssh

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
)

var (
	target = "ubuntu@192.168.64.10"
)

func Test_Connect(t *testing.T) {
	c, err := Connect(target)
	assert.NoError(t, err)
	defer c.Close()

	output, err := c.Run("hostname")
	assert.NoError(t, err)
	assert.Equal(t, "primary\n", output)

	output, err = c.Run(`
		hostname && /bin/id;
		whoami;
		head -10 /etc/passwd;	
	`)
	assert.NoError(t, err)
	assert.Contains(t, output, "root:x:")

	output, err = c.Run("echo hoi && /bin/false")
	assert.Error(t, err)
	assert.Equal(t, "hoi\n", output)

	// assert.ErrorIs(t, err, &ssh.ExitError{})
	if err, ok := err.(*ssh.ExitError); ok {
		assert.Equal(t, 1, err.ExitStatus())
	} else {
		t.Error("unexpected ExitError")
	}
}

func Test_Upload(t *testing.T) {
	c, _ := Connect(target)
	defer c.Close()

	err := c.UploadFile("/tmp/banaan", "/tmp/echo.sh")
	assert.NoError(t, err)

	output, err := c.Run("test -f /tmp/echo.sh && echo 'exists' && ls -la /tmp/echo.sh && rm /tmp/echo.sh")
	assert.NoError(t, err)
	assert.Contains(t, output, "exists\n")
	assert.Contains(t, output, "rwx")

	err = c.UploadBytes([]byte("echo hoi"), "/tmp/x/y/z/echo.sh", 0755)
	assert.Error(t, err)

}

func Test_Stdin(t *testing.T) {
	c, _ := Connect(target)
	defer c.Close()

	output, err := c.RunWriteRead("wc -l", []byte("hoi\nhoi\nhoi\n"))
	assert.NoError(t, err)
	assert.Equal(t, "3\n", string(output))
}

func Test_RunLineStreamer(t *testing.T) {
	c, _ := Connect(target)
	defer c.Close()

	script := "sed -e 's/^/hoi: /'"
	data := []byte("1\n2\n3\n4\n")
	counter := 0

	err := c.RunLineStreamer(script, data, func(line []byte) {
		fmt.Println(string(line))
		counter++
	})

	assert.NoError(t, err)
	assert.Equal(t, 4, counter)
}
