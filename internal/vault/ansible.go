package vault

import (
	"bytes"
	"fmt"
	"io"
	"os"

	ansible "github.com/sosedoff/ansible-vault-go"
)

type ansibleVault struct {
	pass string
}

func (v *ansibleVault) Encrypt(in io.Reader, out io.Writer) error {
	if !v.Ready() {
		return fmt.Errorf("ANSIBLE_VAULT_PASSWORD not set")
	}
	data, err := io.ReadAll(in)
	if err != nil {
		return err
	}

	encrypted, err := ansible.Encrypt(string(data), v.pass)
	if err != nil {
		return err
	}
	_, err = out.Write([]byte(encrypted))
	return err
}

func (v *ansibleVault) Decrypt(in io.Reader) (io.Reader, error) {
	if !v.Ready() {
		return nil, fmt.Errorf("ANSIBLE_VAULT_PASSWORD not set")
	}
	data, err := io.ReadAll(in)
	if err != nil {
		return nil, err
	}

	plain, err := ansible.Decrypt(string(data), v.pass)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader([]byte(plain)), nil
}

func (v *ansibleVault) Magic() []byte {
	return []byte("$ANSIBLE_VAULT;1.1;AES256")
}

func (v *ansibleVault) Ready() bool {
	v.pass = os.Getenv("ANSIBLE_VAULT_PASSWORD") // todo, also allow ANSIBLE_VAULT_PASSWORD_FILE
	return v.pass != ""
}
