package vault

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/spf13/afero"
)

type Vaulter interface {
	Encrypt(in io.Reader, out io.Writer) error
	Decrypt(in io.Reader) (io.Reader, error)
	Magic() []byte
	Ready() bool
}

var (
	allVaulters = []Vaulter{&ageVault{}, &ansibleVault{}}
	magicSize   = findMagicSize()
	fs          = afero.NewOsFs()
	afs         = afero.Afero{Fs: fs}
)

const (
	defaultEditor = "vim"
)

// readCloserWrapper wraps a reader and a closer, so a decrypter reader can be
// used as os.File
type readCloserWrapper struct {
	io.Reader
	fh io.Closer
}

func (rcw *readCloserWrapper) Close() error {
	return rcw.fh.Close()
}

func ReadFile(path string) ([]byte, error) {
	fh, err := Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	return io.ReadAll(fh)
}

// Open opens a file and decrypts it if it is encrypted. If it is not encrypted,
// it returns the original file. If no valid decryptor is found, it returns an
// error.
func Open(path string) (io.ReadCloser, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, magicSize)
	if _, e := io.ReadFull(fh, buffer); e != nil {
		if e != io.ErrUnexpectedEOF && e != io.EOF {
			return nil, fmt.Errorf("non-eof error %w", e)
		}
	}

	// reset
	if _, e2 := fh.Seek(0, io.SeekStart); e2 != nil {
		return nil, fmt.Errorf("seek error %w", e2)
	}

	// compare with magics
	vault, err := findVaulter(buffer)
	if err != nil { // non encrypted or non-supported
		return fh, nil
	}
	r, err := vault.Decrypt(fh)
	if err != nil {
		return nil, err
	}
	return &readCloserWrapper{r, fh}, nil
}

func isEncrypted(path string, v Vaulter) (bool, error) {
	fh, err := os.Open(path)
	if err != nil {
		return false, nil
	}
	defer fh.Close()

	buffer := make([]byte, len(v.Magic()))
	if _, e := io.ReadFull(fh, buffer); e != nil {
		if e != io.ErrUnexpectedEOF && e != io.EOF {
			return false, fmt.Errorf("non-eof error %w", e)
		}
	}
	if _, e2 := fh.Seek(0, io.SeekStart); e2 != nil {
		return false, fmt.Errorf("seek error %w", e2)
	}
	return bytes.Equal(buffer, v.Magic()), nil
}

func findVaulter(buffer []byte) (Vaulter, error) {
	for _, v := range allVaulters {
		if bytes.HasPrefix(buffer, v.Magic()) {
			// log.Debug("Found encrypted file:", reflect.TypeOf(v).Name())
			return v, nil
		}
	}
	return nil, fmt.Errorf("no valid encryption method found")
}

// LaunchEditor opens a file in the editor. If the file is encrypted, it
// temporarily decrypts. After the $EDITOR terminates, it encrypts the file with
// the first available encryptor.
func LaunchEditor(path string) error {
	if len(readyVaulters()) == 0 {
		key := (&ageVault{}).genkey()
		return fmt.Errorf("no valid encryption method found, "+
			"set WHIP_KEY or ANSIBLE_VAULT_PASSWORD\n"+
			"for example, export WHIP_KEY=\"%s\"", key)
	}

	// decode orig or start empty
	src, err := Open(path)
	if err != nil && os.IsNotExist(err) {
		// dummy reader
		src, err = os.Open("/dev/null")
		if err != nil {
			return fmt.Errorf("cannot open: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("cannot run vault.Open: %w", err)
	}

	tmp, err := os.CreateTemp("", "whip-vault")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	_, err = io.Copy(tmp, src)
	if err != nil {
		return fmt.Errorf("cannot copy: %w", err)
	}

	if e := tmp.Close(); e != nil {
		return e
	}
	if e := src.Close(); e != nil {
		return e
	}

	// start the editing magic!
	cmd := exec.Command("/bin/sh", "-c", getEditor()+" "+tmp.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if e := cmd.Run(); e != nil {
		return e
	}

	// encrypt to orig path
	in, err := os.Open(tmp.Name())
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	return readyVaulters()[0].Encrypt(in, out) // take first valid
}

func getEditor() string {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = defaultEditor
	}
	return editor
}

func findMagicSize() int {
	max := 0
	for _, v := range allVaulters {
		if len(v.Magic()) > max {
			max = len(v.Magic())
		}
	}
	return max
}

func readyVaulters() []Vaulter {
	ready := make([]Vaulter, 0)
	for _, v := range allVaulters {
		if v.Ready() {
			ready = append(ready, v)
		}
	}
	return ready
}
