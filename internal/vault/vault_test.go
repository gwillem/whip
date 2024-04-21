package vault

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// BenchmarkAge-8               13791             85455 ns/op
// BenchmarkAnsible-8             364           3281199 ns/op

func BenchmarkAge(b *testing.B) {
	v := ageVault{}
	require.NoError(b, v.loadKey(ageTestKey))
	for i := 0; i < b.N; i++ {
		_ = v.Encrypt(bytes.NewReader([]byte("hoi")), io.Discard)
	}
}

func BenchmarkAnsible(b *testing.B) {
	v := ansibleVault{}
	for i := 0; i < b.N; i++ {
		_ = v.Encrypt(bytes.NewReader([]byte("hoi")), io.Discard)
	}
}
