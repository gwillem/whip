package vault

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"filippo.io/age"
	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/fsutil"
)

const (
	ageEnv       = "WHIP_KEY"
	ageEnvScript = ".whip/secret.sh"
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
	keyStr := os.Getenv(ageEnv)
	if keyStr == "" {
		if sp := fsutil.FindAncestorPath(ageEnvScript); sp != "" {
			log.Debug("Using script", sp, "to generate vault key")
			keyStr, err = readFromScript(sp)
			if err != nil {
				log.Error(err)
				return nil, err
			}
		}
	}
	if keyStr != "" {
		v.id, err = age.ParseX25519Identity(keyStr)
		if err != nil {
			return nil, err
		}
		return v.id, nil
	}

	id, _ = age.GenerateX25519Identity()
	return nil, fmt.Errorf("no $%s set, here's a new one: %s\n(or create .whip/secret.sh to generate it dynamically)", ageEnv, id)
}

func (v *ageVault) Ready() bool {
	_, err := v.getID()
	return err == nil
}

func (v *ageVault) genkey() string {
	id, _ := age.GenerateX25519Identity()
	return id.String()
}

func readFromScript(path string) (string, error) {
	fi, err := os.Stat(path)
	if err == nil && !fi.IsDir() {
		if fi.Mode()&os.ModePerm&0o100 == 0 {
			fmt.Println("oops", fi.Mode())
			return "", fmt.Errorf("script %s is not executable", path)
		}

		data, err := exec.Command(path).CombinedOutput()
		data = bytes.TrimSpace(data)
		// fmt.Printf("got '%s'\n", string(data))
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return "", nil
}
