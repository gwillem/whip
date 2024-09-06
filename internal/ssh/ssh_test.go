//go:build integration
// +build integration

package ssh

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

var target = "ubuntu@192.168.64.10"

func Test_UploadBytesXZ(t *testing.T) {
	c, err := Connect(target)
	require.NoError(t, err)
	defer c.Close()

	// Create a dummy slice of 10 MB (0's)
	dummyData, err := os.ReadFile("testfile.xz")
	require.NoError(t, err)

	// err = c.UploadBytes(dummyData, "/tmp/dummy_file", 0o644)
	// require.NoError(t, err)

	err = c.UploadBytesXZ(dummyData, "/tmp/dummy_file", 0o644)
	require.NoError(t, err)
}

func Test_Connect(t *testing.T) {
	c, err := Connect(target)
	require.NoError(t, err)
	defer c.Close()

	output, err := c.Run("echo primary")
	require.NoError(t, err)
	require.Equal(t, "primary\n", output)

	output, err = c.Run(`
		hostname && /bin/id;
		whoami;
		head -10 /etc/passwd;	
	`)
	require.NoError(t, err)
	require.Contains(t, output, "root:x:")

	output, err = c.Run("echo hoi && /bin/false")
	require.Error(t, err)
	require.Equal(t, "hoi\n", output)

	// require.ErrorIs(t, err, &ssh.ExitError{})
	if err, ok := err.(*ssh.ExitError); ok {
		require.Equal(t, 1, err.ExitStatus())
	} else {
		t.Error("unexpected ExitError")
	}
}

func Test_Upload(t *testing.T) {
	c, err := Connect(target)
	require.NoError(t, err)
	defer c.Close()

	err = c.UploadFile("/tmp/banaan", "/tmp/echo.sh")
	require.NoError(t, err)

	output, err := c.Run("test -f /tmp/echo.sh && echo 'exists' && ls -la /tmp/echo.sh && rm /tmp/echo.sh")
	require.NoError(t, err)
	require.Contains(t, output, "exists\n")
	require.Contains(t, output, "rw-r")

	err = c.UploadBytes([]byte("echo hoi"), "/tmp/x/y/z/echo.sh", 0o755)
	require.Error(t, err)
}

func Test_Stdin(t *testing.T) {
	c, err := Connect(target)
	require.NoError(t, err)
	defer c.Close()

	output, err := c.RunWriteRead("wc -l", []byte("hoi\nhoi\nhoi\n"))
	require.NoError(t, err)
	require.Equal(t, "3\n", string(output))
}

func Test_RunLineStreamer(t *testing.T) {
	c, err := Connect(target)
	require.NoError(t, err)
	defer c.Close()

	script := "sed -e 's/^/hoi: /'"
	data := []byte("1\n2\n3\n4\n")
	counter := 0

	err = c.RunLineStreamer(script, data, func(line []byte) {
		// fmt.Println(string(line))
		counter++
	})

	require.NoError(t, err)
	require.Equal(t, 4, counter)
}
