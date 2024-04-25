package assets

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_CompressDecompress(t *testing.T) {
	// compress random binary
	gopath, err := exec.LookPath("ls")
	require.NoError(t, err)

	data, err := os.ReadFile(gopath)
	require.NoError(t, err)

	buf := bytes.NewBuffer(data)
	pr, pw := io.Pipe()
	plainSize := buf.Len()
	go func() {
		// Encode and compress the data
		require.NoError(t, Compress(buf, pw))
		require.NoError(t, pw.Close())
	}()

	data, err = io.ReadAll(pr)
	require.NoError(t, err)

	compressedSize := len(data)

	// Check compression rate. go binary reduced to 55%, ls to %15
	rate := float32(compressedSize) / float32(plainSize) * 100
	require.Less(t, rate, float32(80))
	require.Greater(t, rate, float32(5))

	// Decompress and decode the data
	require.Equal(t, 0, buf.Len())
	require.NoError(t, Decompress(bytes.NewReader(data), buf))

	require.Equal(t, plainSize, buf.Len())
}
