package vault

import (
	"fmt"
	"io"
	"os"

	"filippo.io/age"
)

// headerGPG = []byte{0x85, 0x01, 0x8C, 0x03, 0x93, 0xE5, 0x4C, 0x74, 0x67, 0x58, 0x3E, 0x45}
var headerAge = []byte{
	0x61, 0x67, 0x65, 0x2d, 0x65, 0x6e, 0x63, 0x72,
	0x79, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x6f,
	0x72, 0x67, 0x2f, 0x76, 0x31,
}

type ageVault struct {
	id *age.X25519Identity
}

func (v *ageVault) Encrypt(in io.Reader, out io.Writer) error {
	id, err := v.getID()
	if err != nil {
		return err
	}
	rcpt := id.Recipient()
	encryptor, err := age.Encrypt(out, rcpt)
	if err != nil {
		return fmt.Errorf("encryptor err %w", err)
	}
	defer encryptor.Close()
	_, err = io.Copy(encryptor, in)
	if err != nil {
		return fmt.Errorf("cannot copy to encryptor: %w", err)
	}

	err = encryptor.Close()
	if err != nil {
		return fmt.Errorf("cannot close encryptor %w", err)
	}
	return nil
}

func (v *ageVault) Decrypt(in io.Reader) (io.Reader, error) {
	id, err := v.getID()
	if err != nil {
		return nil, err
	}
	return age.Decrypt(in, id)
}

func (v *ageVault) Magic() []byte {
	return headerAge
}

func (v *ageVault) loadKey(key string) error {
	id, err := age.ParseX25519Identity(key)
	if err != nil {
		return err
	}
	v.id = id
	return nil
}

func (v *ageVault) getID() (id *age.X25519Identity, err error) {
	if v.id != nil {
		return v.id, nil
	}
	keyStr := os.Getenv("WHIP_KEY")
	if keyStr != "" {
		v.id, err = age.ParseX25519Identity(keyStr)
		if err != nil {
			return nil, err
		}
		return v.id, nil
	}
	id, _ = age.GenerateX25519Identity()
	return nil, fmt.Errorf("no $WHIP_KEY set, here's a new one: %s", id)
}

func (v *ageVault) Ready() bool {
	_, err := v.getID()
	return err == nil
}

func (v *ageVault) genkey() string {
	id, _ := age.GenerateX25519Identity()
	return id.String()
}
