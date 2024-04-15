package runners

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/parser"
	"github.com/spf13/afero"
)

func init() {
	registerRunner("files", Files, runnerMeta{})
}

const srcRoot = "/"

var (
	defaultUmask    = os.FileMode(0o022)
	defaultDirMode  = os.FileMode(0o755)
	defaultFileMode = os.FileMode(0o644)
)

type fileMeta struct {
	uid    *int
	gid    *int
	umask  *os.FileMode
	notify []string
}
type prefixMetaMap struct {
	orderedPrefixes []string
	metamap         map[string]fileMeta
}

func (pm *prefixMetaMap) getMeta(path string) fileMeta {
	finalMeta := fileMeta{}
	for _, prefix := range pm.orderedPrefixes {
		if strings.HasPrefix(path, prefix) {
			meta := pm.metamap[prefix]
			if meta.uid != nil {
				finalMeta.uid = meta.uid
			}
			if meta.gid != nil {
				finalMeta.gid = meta.gid
			}
			if meta.umask != nil {
				finalMeta.umask = meta.umask
			}
			if meta.notify != nil {
				finalMeta.notify = append(finalMeta.notify, meta.notify...)
			}
		}
	}
	return finalMeta
}

func Files(args model.TaskArgs) (tr model.TaskResult) {
	// dstRoot is eiter the abs dst or $HOME + dst  or / + dst
	dstRoot, _ := args["dst"].(string)
	switch {
	case dstRoot == "":
		dstRoot = filepath.Join("/", os.ExpandEnv("$HOME"))
	case strings.HasPrefix(dstRoot, "/"):
		// do nothing
	default:
		dstRoot = filepath.Join("/", os.ExpandEnv("$HOME"), dstRoot)
	}

	pm, err := parsePrefixMeta(args)
	if err != nil {
		return failure(err)
	}
	log.Debug("prefix meta", pm)

	// dst root should exist already (so we won't change perms on / or $HOME)
	if ok, err := fsutil.Exists(dstRoot); !ok || err != nil {
		return failure("cannot read dst path", dstRoot, err)
	}

	output := ""
	if args["_assets"] == nil {
		return failure("no assets found")
	}

	srcFs, ok := args["_assets"].(afero.Fs)
	if !ok {
		return failure("wrong type of _assets?")
	}
	err = afero.Walk(srcFs, srcRoot, func(srcPath string, srcFi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if srcPath == srcRoot {
			return nil // don't modify root element
		}

		// update srcFs[srcPath] and srcFi with prefix meta, if any

		dstPath := filepath.Join(dstRoot, srcPath)
		// output += pp.Sprintln(dstPath)
		// from here on, ensure path
		changed, err := ensurePath(srcFs, srcFi, srcPath, dstPath)
		if err != nil {
			return err
		}
		if changed {
			tr.Changed = true
		}
		status := "ok"
		if changed {
			status = "changed"
		}
		output += fmt.Sprintf("%-7s %s\n", status, dstPath)
		return nil
	})
	if err != nil {
		return failure(err)
	}

	tr.Output = output
	tr.Status = success
	return tr
}

// Takes meta attributes for a "files" task and returns a prefixMetaMap so that
// the runner can chown/chmod part of the file tree and notify different handlers
func parsePrefixMeta(args model.TaskArgs) (*prefixMetaMap, error) {
	pm := prefixMetaMap{
		orderedPrefixes: []string{},
		metamap:         map[string]fileMeta{},
	}

	for prefix, v := range args {
		if !strings.HasPrefix(prefix, "/") {
			continue
		}

		argStr, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("prefix args %v is not a string", v)
		}
		attrs := parser.ParseArgString(argStr)

		fm := fileMeta{
			umask: &defaultUmask,
		}
		if attrs["umask"] != "" {
			if ui, err := strconv.Atoi(attrs["umask"]); err != nil {
				return nil, fmt.Errorf("cannot parse umask %s", attrs["umask"])
			} else {
				um := os.FileMode(ui)
				fm.umask = &um
			}
		}

		var uid, gid int

		if attrs["owner"] != "" {
			owner, err := user.Lookup(attrs["owner"])
			if err != nil {
				return nil, fmt.Errorf("cannot find user %s", attrs["owner"])
			}
			uid, err = strconv.Atoi(owner.Uid)
			if err != nil {
				return nil, fmt.Errorf("cannot parse uid %s", owner.Uid)
			}
		}

		if attrs["group"] != "" {
			group, err := user.LookupGroup(attrs["group"])
			if err != nil {
				return nil, fmt.Errorf("cannot find group %s", attrs["group"])
			}

			gid, err = strconv.Atoi(group.Gid)
			if err != nil {
				return nil, fmt.Errorf("cannot parse gid %s", group.Gid)
			}
		}

		if attrs["notify"] != "" {
			fm.notify = strings.Split(attrs["notify"], ",") // TODO generalize
		}

		fm.uid = &uid
		fm.gid = &gid

		pm.metamap[prefix] = fm
	}

	for prefix := range pm.metamap {
		pm.orderedPrefixes = append(pm.orderedPrefixes, prefix)
	}
	slices.Sort(pm.orderedPrefixes) // sort prefixes to ensure shorted prefix is first

	return &pm, nil
}

func ensurePath(srcFs afero.Fs, srcFi os.FileInfo, srcPath, dstPath string) (changed bool, err error) {
	// log.Debug("ensure path", srcPath, "scrFi mode", srcFi.Mode())
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

	if os.IsNotExist(err) || dstFi.Size() != srcFi.Size() ||
		!filesAreEqual(srcFs, fs, srcPath, path) { // todo: also compare perms/owner

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
