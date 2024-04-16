package vault

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"filippo.io/age"
)

const defaultEditor = "vim"

// headerGPG = []byte{0x85, 0x01, 0x8C, 0x03, 0x93, 0xE5, 0x4C, 0x74, 0x67, 0x58, 0x3E, 0x45}
var headerAge = []byte{
	0x61, 0x67, 0x65, 0x2d, 0x65, 0x6e, 0x63, 0x72,
	0x79, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x6f,
	0x72, 0x67, 0x2f, 0x76, 0x31,
}

var ageID *age.X25519Identity

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

func Open(path string) (io.ReadCloser, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, len(headerAge))

	if _, e := io.ReadFull(fh, buffer); e != nil {
		if e != io.ErrUnexpectedEOF && e != io.EOF {
			return nil, fmt.Errorf("non-eof error %w", e)
		}
	}

	// reset
	if _, e2 := fh.Seek(0, io.SeekStart); e2 != nil {
		return nil, fmt.Errorf("seek error %w", e2)
	}

	// non encrypted file
	if !bytes.Equal(headerAge, buffer) {
		return fh, nil
	}

	return decrypt(fh)
	// return runCommand("gpg", "-qd", path)
}

func decrypt(fh io.ReadCloser) (io.ReadCloser, error) {
	id, err := getID()
	if err != nil {
		return nil, err
	}
	decryptor, err := age.Decrypt(fh, id)
	if err != nil {
		return nil, err
	}

	return &readCloserWrapper{Reader: decryptor, fh: fh}, nil
}

func encrypt(in io.Reader, out io.Writer) error {
	id, err := getID()
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

/*

# public key: age1lquxy2rqn77fahedz50kdmy2vg4ex09umqys6fxl526c5wjh6flq7elfdm
AGE-SECRET-KEY-1X9QJ6JCA6T7YPLN8QNDN9SVCLV89V2CL9Q75M0JESHW8YPJNU5FQ9UMU54

echo hoi | gpg -e -r info@sansec.io -a --trust-model=always

*/

// runCommand executes the specified command with given arguments and returns its stdout as an os.File
// func runCommand(name string, args ...string) (*os.File, error) {
// 	// Create a pipe
// 	r, w, err := os.Pipe()
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Set up the command that will write to the write-end of our pipe
// 	cmd := exec.Command(name, args...)
// 	cmd.Stdout = w
// 	cmd.Stderr = os.Stderr

// 	// Start the command
// 	if err := cmd.Start(); err != nil {
// 		w.Close()
// 		r.Close()
// 		return nil, err
// 	}

// 	// Close the write end of the pipe in the current goroutine after command starts
// 	go func() {
// 		defer w.Close()
// 		cmd.Wait()
// 	}()

// 	// Return the read end of the pipe
// 	return r, nil
// }

func LaunchEditor(path string) error {
	// key loaded?
	if _, err := getID(); err != nil {
		return err
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
	cmd := exec.Command(getEditor(), tmp.Name())
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
	return encrypt(in, out)
}

func getID() (id *age.X25519Identity, err error) {
	if ageID != nil {
		return ageID, nil
	}

	keyStr := os.Getenv("WHIP_KEY")

	if keyStr != "" {
		if err := loadID(keyStr); err != nil {
			return nil, err
		}
		return ageID, nil
	}

	id, _ = age.GenerateX25519Identity()
	return nil, fmt.Errorf("no $WHIP_KEY set, here's a new one: %s", id)
}

func loadID(key string) (err error) {
	ageID, err = age.ParseX25519Identity(key)
	return err
}

// todo? wrapper to decrypt whip.gpg

func getEditor() string {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		return defaultEditor
	}
	return editor
}
