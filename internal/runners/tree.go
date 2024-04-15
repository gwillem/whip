package runners

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/parser"
	"github.com/spf13/afero"
)

func init() {
	registerRunner("tree", Tree, runnerMeta{})
}

const srcRoot = "/"

var (
	defaultUmask    = os.FileMode(0o022)
	defaultDirMode  = os.FileMode(0o777)
	defaultFileMode = os.FileMode(0o666)
)

type fileMeta struct {
	uid    *int
	gid    *int
	umask  os.FileMode
	notify []string
}
type prefixMetaMap struct {
	orderedPrefixes []string
	metamap         map[string]fileMeta
}

type filesObj struct {
	path  string
	data  []byte
	isDir bool
	umask os.FileMode
	uid   *int
	gid   *int
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
			if meta.umask > 0 {
				finalMeta.umask = meta.umask
			}
			if meta.notify != nil {
				finalMeta.notify = append(finalMeta.notify, meta.notify...)
			}
		}
	}
	return finalMeta
}

func Tree(args model.TaskArgs, vars model.TaskVars) (tr model.TaskResult) {
	// dstRoot is eiter the abs dst or $HOME + dst  or / + dst
	dstRoot := getDstRoot(args["dst"])

	// dst root should exist already (so we won't change perms on / or $HOME)
	if ok, err := fsutil.Exists(dstRoot); !ok || err != nil {
		return failure("cannot read dst path", dstRoot, err)
	}

	pm, err := parsePrefixMeta(args)
	if err != nil {
		return failure(err)
	}
	// log.Debug("prefix meta", pm)

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
		dstPath := filepath.Join(dstRoot, srcPath)

		f := filesObj{
			path:  dstPath,
			isDir: srcFi.IsDir(),
			umask: defaultUmask,
		}

		if !f.isDir {
			f.data, err = afero.ReadFile(srcFs, srcPath)
			if err != nil {
				return fmt.Errorf("afero read rr on %s: %w", srcPath, err)
			}

			// template?
			if isText(f.data) {
				// log.Debug("parsing template", srcPath, "with vars", vars)
				f.data, err = tplParseBytes(f.data, vars)
				if err != nil {
					return fmt.Errorf("tplParseBytes error on %s: %w", srcPath, err)
				}
			}

		}

		// update srcFs[srcPath] and srcFi with prefix meta, if any
		meta := pm.getMeta(srcPath)
		if meta.umask > 0 {
			f.umask = meta.umask
		}
		f.uid = meta.uid
		f.gid = meta.gid

		// output += pp.Sprintln(dstPath)
		// from here on, ensure path
		changed, err := ensurePath(f)
		if err != nil {
			return fmt.Errorf("ensurePath error on %s: %w", dstPath, err)
		}
		if changed {
			tr.Changed = true
		}
		status := "ok"
		if changed {
			status = "changed"

			if meta.notify != nil {
				tr.Task.Notify = append(tr.Task.Notify, meta.notify...)
				// activate handlers
			}

		}
		output += fmt.Sprintf("%-7s %s\n", status, dstPath)
		return nil
	})
	if err != nil {
		return failure(err)
	}

	tr.Output = output
	tr.Status = success

	// remove dupes, todo, should use set
	slices.Sort(tr.Task.Notify)
	tr.Task.Notify = slices.Compact(tr.Task.Notify)

	return tr
}

func getDstRoot(arg any) string {
	dstRoot, _ := arg.(string)
	switch {
	case dstRoot == "":
		return filepath.Join("/", os.ExpandEnv("$HOME"))
	case strings.HasPrefix(dstRoot, "/"):
		// do nothing
	default:
		return filepath.Join("/", os.ExpandEnv("$HOME"), dstRoot)
	}
	return dstRoot
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

		fm := fileMeta{}
		if attrs["umask"] != "" {
			if ui, err := strconv.ParseInt(attrs["umask"], 8, 32); err != nil {
				return nil, fmt.Errorf("cannot parse octal umask %s", attrs["umask"])
			} else {
				fm.umask = os.FileMode(ui)
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

func ensurePath(f filesObj) (changed bool, err error) {
	if f.isDir {
		return ensureDir(f)
	}
	return ensureFile(f)
}

func ensureDir(f filesObj) (changed bool, err error) {
	if !f.isDir {
		return false, fmt.Errorf("ensureDir called on non-dir? %s", f.path)
	}

	fi, err := os.Stat(f.path)

	mode := defaultDirMode &^ f.umask
	if err != nil && os.IsNotExist(err) {
		// create dir
		if err := fs.Mkdir(f.path, mode); err != nil {
			return false, fmt.Errorf("mkdir error on %s: %w", f.path, err)
		}
		changed = true
	} else if err != nil {
		return false, fmt.Errorf("read error on %s: %w", f.path, err)
	}

	if fi != nil && !fi.IsDir() {
		return false, fmt.Errorf("cannot overwrite non-dir %s with dir", f.path)
	}
	if fi != nil && fi.Mode()&os.ModePerm != mode {
		log.Debug("changing mode", uint32(fi.Mode()), "to", mode)
		if err := fs.Chmod(f.path, mode); err != nil {
			return false, err
		}
		changed = true
	}

	if c, e := chown(f.path, f.uid, f.gid); e != nil {
		return false, e
	} else if c {
		changed = true
	}
	return changed, nil
}

func ensureFile(f filesObj) (changed bool, err error) {
	if f.isDir {
		return false, fmt.Errorf("ensureFile called on dir? %s", f.path)
	}
	mode := defaultFileMode &^ f.umask

	fi, err := fs.Stat(f.path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read error on %s: %w", f.path, err)
	}

	dataDiffers := func() bool {
		if fi.Size() != int64(len(f.data)) {
			return true
		}
		chk, err := getFileChecksum(fs, f.path)
		if err != nil {
			log.Warn("cannot get checksum for", f.path, err)
			return true
		}
		return !bytes.Equal(getDataChecksum(f.data), chk)
	}

	if fi != nil && fi.IsDir() {
		return false, fmt.Errorf("cannot overwrite path %s with file", f.path)
	}

	// need to write file?
	if os.IsNotExist(err) || dataDiffers() {
		fh, err := fs.OpenFile(f.path, os.O_CREATE|os.O_WRONLY, mode) // todo: does this update the mode?
		if err != nil {
			return false, fmt.Errorf("open error on %s: %w", f.path, err)
		}
		defer fh.Close()

		_, err = fh.Write(f.data)
		if err != nil {
			return false, fmt.Errorf("write error on %s: %w", f.path, err)
		}
		changed = true
	}

	// need to change mode?
	if fi != nil && fi.Mode() != mode {
		if err := fs.Chmod(f.path, mode); err != nil {
			return false, fmt.Errorf("chmod err on %s: %w", f.path, err)
		}
		changed = true
	}

	// need to change owner?
	if c, err := chown(f.path, f.uid, f.gid); err != nil {
		return false, fmt.Errorf("chown err on %s: %w", f.path, err)
	} else if c {
		changed = true
	}

	return changed, nil
}

func chown(path string, u, g *int) (changed bool, err error) {
	uid := -1
	gid := -1

	fi, err := fs.Stat(path)
	if err != nil {
		return false, err
	}

	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false, fmt.Errorf("cannot get stat_t for %s", path)
	}

	if u != nil && stat.Uid != uint32(*u) {
		uid = *u
	}
	if g != nil && stat.Gid != uint32(*g) {
		gid = *g
	}

	if uid == -1 && gid == -1 {
		return false, nil
	}

	if err := fs.Chown(path, uid, gid); err != nil {
		return false, err
	}
	return true, nil
}
