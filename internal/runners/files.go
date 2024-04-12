package runners

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/model"
	"github.com/spf13/afero"
)

func init() {
	registerRunner("files", Files, runnerMeta{})
}

const (
	defaultDirMode  = 0o755
	defaultFileMode = 0o644
)

func Files(args model.TaskArgs) (tr model.TaskResult) {
	// root is eiter the abs dst or $HOME + dst  or / + dst
	root, _ := args["dst"].(string)

	switch {
	case root == "":
		root = filepath.Join("/", os.ExpandEnv("$HOME"))
	case strings.HasPrefix(root, "/"):
		// do nothing
	default:
		root = filepath.Join("/", os.ExpandEnv("$HOME"), root)
	}

	output := ""
	if args["_assets"] == nil {
		return failure("no assets found")
	}

	srcFs, ok := args["_assets"].(afero.Fs)
	if !ok {
		return failure("wrong type of _assets?")
	}

	err := afero.Walk(srcFs, "/", func(srcPath string, srcFi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		dstPath := filepath.Join(root, srcPath)
		// output += pp.Sprintln(dstPath)
		// from here on, ensure path
		changed, err := ensurePath(srcFs, srcFi, srcPath, dstPath)
		if err != nil {
			return err
		}
		if changed {
			tr.Changed = true
		}
		output += fmt.Sprintf("%v %s\n", changed, dstPath)
		return nil
	})
	if err != nil {
		return failure(err)
	}

	tr.Output = output
	tr.Status = success
	return tr
}

func ensurePath(srcFs afero.Fs, srcFi os.FileInfo, srcPath, dstPath string) (changed bool, err error) {
	log.Debug("ensure path", srcPath, "scrFi mode", srcFi.Mode())
	if srcFi.IsDir() {
		changed, err = ensureDir(dstPath, srcFi.Mode())
	} else {
		changed, err = ensureFile(dstPath, srcFi, srcFs, srcPath)
	}

	if err != nil {
		return false, err
	}
	return
}

func ensureDir(path string, _ os.FileMode) (bool, error) {
	dstFi, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		// create dir
		return true, fs.Mkdir(path, defaultDirMode)
	}
	if err != nil {
		return false, fmt.Errorf("read error on %s: %w", path, err)
	}

	if !dstFi.IsDir() {
		return false, fmt.Errorf("cannot overwrite path %s with dir", path)
	}
	if dstFi.Mode()&os.ModePerm != defaultDirMode {
		log.Debug("changing mode", uint32(dstFi.Mode()), "to", defaultDirMode)
		return true, fs.Chmod(path, defaultDirMode)
	}
	return false, nil
}

func ensureFile(path string, srcFi os.FileInfo, srcFs afero.Fs, srcPath string) (bool, error) {
	dstFi, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read error on %s: %w", path, err)
	}

	if os.IsNotExist(err) || dstFi.Size() != srcFi.Size() { // todo: actually run checksums or check on date
		// create file

		srcFile, err := srcFs.Open(srcPath)
		if err != nil {
			return false, err
		}
		defer srcFile.Close()
		dstFile, err := fs.OpenFile(path, os.O_CREATE|os.O_WRONLY, srcFi.Mode())
		if err != nil {
			return false, err
		}
		defer dstFile.Close()
		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	if dstFi.IsDir() {
		return false, fmt.Errorf("cannot overwrite path %s with file", path)
	}
	if dstFi.Mode() != srcFi.Mode() {
		return true, fs.Chmod(path, srcFi.Mode())
	}
	return false, nil
}
