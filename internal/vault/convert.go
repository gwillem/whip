package vault

import (
	"bytes"
	"fmt"
	"os"

	log "github.com/gwillem/go-simplelog"
)

// ConvertAnsibleToWhip converts an Ansible Vault file to a Whip (Age) file.
func ConvertAnsibleToWhip(root string) error {
	// ensure that both ansible and whip are ready
	var ansible Vaulter = &ansibleVault{}
	var age Vaulter = &ageVault{}
	if !ansible.Ready() {
		return fmt.Errorf("err, Ansible Vault is not ready, set $ANSIBLE_VAULT_PASSWORD")
	}
	if !age.Ready() {
		return fmt.Errorf("err, Whip Vault is not ready, set $WHIP_KEY")
	}

	counter := 0

	// walk over the root and find ansible encrypted files
	err := fsutil.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		isEnc, err := isEncrypted(path, ansible)
		if err != nil || !isEnc {
			return err
		}

		fmt.Println("ansible encrypted:", path)
		counter++

		source, err := ReadFile(path)
		if err != nil {
			return fmt.Errorf("oops reading %s: %w", path, err)
		}

		w, err := fs.Create(path)
		if err != nil {
			return err
		}
		defer w.Close()
		if err := age.Encrypt(bytes.NewReader(source), w); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	log.Ok("Converted", counter, "Ansible Vault files to Whip")
	return nil
}
