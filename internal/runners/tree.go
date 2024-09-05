package runners

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/assets"
	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/parser"
	"github.com/spf13/afero"
)

func init() {
	registerRunner("tree", runner{
		run:    tree,
		prerun: treePrerun,
		meta: runnerMeta{
			requiredArgs: []string{"src"},
			optionalArgs: []string{"_assets"},
		},
	})
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
	mode  os.FileMode
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

func treePrerun(t *model.Task) (tr model.TaskResult) {
	// should load assets (if any) into _assets
	// pp.Println(t)
	path := t.Args.String("src")
	assets, err := assets.DirToAsset(path)
	if err != nil {
		return failure("BOOHOO", fmt.Errorf("assets loader on path %s: %s", path, err))
	}
	t.Args["_assets"] = assets
	return model.TaskResult{Status: Success}
}

func tree(t *model.Task) (tr model.TaskResult) {
	// dstRoot is eiter the abs dst or $HOME + dst  or / + dst
	dstRoot := getDstRoot(t.Args["dst"])

	// dst root should exist already (so we won't change perms on / or $HOME)
	if ok, err := fsutil.Exists(dstRoot); !ok || err != nil {
		return failure("cannot read dst path", dstRoot, err)
	}

	pm, err := parsePrefixMeta(t.Args)
	if err != nil {
		return failure(err)
	}
	// log.Debug("prefix meta", pm)

	output := ""
	if t.Args["_assets"] == nil {
		return failure("no assets found")
	}

	rawAssets, ok := t.Args["_assets"].(model.Asset)
	if !ok {
		return failure("wrong type of _assets?")
	}

	srcFs, err := assets.AssetToFS(&rawAssets)
	if err != nil {
		return failure("cannot convert assets to fs", err)
	}

	tr.Notify = make(map[string]bool)

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
			mode:  srcFi.Mode().Perm(),
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
				f.data, err = tplParseBytes(f.data, t.Vars)
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
		status := "skip"
		if changed {
			tr.Changed = true
			status = "changed"
			for _, n := range meta.notify {
				tr.Notify[n] = true
			}
		}
		output += fmt.Sprintf("%-7s %s\n", status, dstPath)
		return nil
	})
	if err != nil {
		return failure(err)
	}

	tr.Output = output
	tr.Status = Success
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
		if attrs.String("umask") != "" {
			ui, err := strconv.ParseInt(attrs.String("umask"), 8, 32)
			if err != nil {
				return nil, fmt.Errorf("cannot parse octal umask %s", attrs.String("umask"))
			}
			fm.umask = os.FileMode(ui)
		}

		var uid, gid int

		username := attrs.String("owner")

		if username != "" {
			owner, err := osUser.Lookup(username)
			if err != nil {
				return nil, fmt.Errorf("cannot find user %s", username)
			}
			uid, err = strconv.Atoi(owner.Uid)
			if err != nil {
				return nil, fmt.Errorf("cannot parse uid %s", owner.Uid)
			}
		}

		if attrs.String("group") != "" {
			group, err := osUser.LookupGroup(attrs.String("group"))
			if err != nil {
				return nil, fmt.Errorf("cannot find group %s", attrs.String("group"))
			}

			gid, err = strconv.Atoi(group.Gid)
			if err != nil {
				return nil, fmt.Errorf("cannot parse gid %s", group.Gid)
			}
		}

		if attrs.String("notify") != "" {
			fm.notify = parser.StringToSlice(attrs.String("notify"))
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
		return false, fmt.Errorf("cannot overwrite dir %s with file", f.path)
	}

	// register delta mode, because we lose old mode during write
	if fi != nil && fi.Mode() != mode {
		changed = true
	}

	// need to write file?
	if os.IsNotExist(err) || dataDiffers() {
		// Create a temporary file in the same directory
		tempFile, err := os.CreateTemp(filepath.Dir(f.path), "temp_*")
		if err != nil {
			return false, fmt.Errorf("create temp file error for %s: %w", f.path, err)
		}
		tempPath := tempFile.Name()
		defer func() {
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
		}()

		// Write data to the temporary file
		_, err = tempFile.Write(f.data)
		if err != nil {
			return false, fmt.Errorf("write error to temp file %s for %s: %w", tempPath, f.path, err)
		}

		if tempFile.Close() != nil {
			return false, fmt.Errorf("error closing temp file for %s: %w", f.path, err)
		}

		if os.Chmod(tempPath, mode) != nil {
			return false, fmt.Errorf("chmod error on temp file %s for %s: %w", tempPath, f.path, err)
		}

		// Perform the atomic rename
		err = os.Rename(tempPath, f.path)
		if err != nil {
			return false, fmt.Errorf("rename error from temp %s to %s: %w", tempPath, f.path, err)
		}

		changed = true
	}

	// need to change mode in case the file existed
	if fi != nil && fi.Mode() != mode {
		log.Debug("needs mode change", f.path)
		if err := fs.Chmod(f.path, mode); err != nil {
			return false, fmt.Errorf("chmod err on %s: %w", f.path, err)
		}
		changed = true
	}

	// need to change owner?
	if c, err := chown(f.path, f.uid, f.gid); err != nil {
		log.Debug("needs owner change", f.path)
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

		if u != nil {
			uid = *u
		}
		if g != nil {
			gid = *g
		}

		// pass for now, our afero.FS test abstraction does not support stat_t (uid/gid)
		// return false, fmt.Errorf("cannot get stat_t for %s", path)
	} else { // temp fix for afero.FS test abstraction, we will just always chown if non-linux
		if u != nil && stat.Uid != uint32(*u) {
			uid = *u
		}
		if g != nil && stat.Gid != uint32(*g) {
			gid = *g
		}

		if uid == -1 && gid == -1 {
			return false, nil
		}
	}

	if err := fs.Chown(path, uid, gid); err != nil {
		return false, err
	}
	return true, nil
}
