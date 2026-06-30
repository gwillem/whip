package vault

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncryptDecryptStream(t *testing.T) {
	require.NoError(t, ageTestVault.loadKey(ageTestKey))
	oldAll := allVaulters
	defer func() {
		allVaulters = oldAll
	}()
	allVaulters = []Vaulter{ageTestVault}

	plain := []byte("secret data\n")
	encrypted := &bytes.Buffer{}
	require.NoError(t, EncryptStream(bytes.NewReader(plain), encrypted))
	require.NotContains(t, encrypted.String(), string(plain))

	decrypted := &bytes.Buffer{}
	require.NoError(t, DecryptStream(bytes.NewReader(encrypted.Bytes()), decrypted))
	require.Equal(t, plain, decrypted.Bytes())
}

func TestDecryptStreamPlaintextFails(t *testing.T) {
	out := &bytes.Buffer{}
	err := DecryptStream(bytes.NewReader([]byte("plain\n")), out)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no valid encryption method")
	require.Empty(t, out.Bytes())
}

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
