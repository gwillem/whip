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
			// dst was read by the runner and undeclared, so argument
			// validation rejected every real tree task. Prefix metadata keys
			// start with "/" and are skipped by the validator: they are data,
			// not argument names.
			optionalArgs: []string{"dst", "_assets"},
		},
	})
}

const (
	srcRoot      = "/"
	defaultUmask = os.FileMode(0o022)
)

type fileMeta struct {
	uid    *int
	gid    *int
	umask  os.FileMode
	notify []string

	// template is nil unless a prefix names it, so that a longer prefix can
	// switch rendering back on inside an otherwise verbatim tree. Rendering
	// remains the default, hence nil means yes.
	template *bool

	// absent means the prefix is removed from the target rather than shipped.
	absent bool
}
type prefixMetaMap struct {
	orderedPrefixes []string
	metamap         map[string]fileMeta
}

// templating reports whether files under this prefix are rendered.
func (fm fileMeta) templating() bool {
	return fm.template == nil || *fm.template
}

// umaskOr returns the prefix umask, falling back to the tree default.
func (fm fileMeta) umaskOr() os.FileMode {
	if fm.umask > 0 {
		return fm.umask
	}
	return defaultUmask
}

type filesObj struct {
	path  string
	data  []byte
	isDir bool
	mode  os.FileMode
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
			if meta.template != nil {
				finalMeta.template = meta.template
			}
			// Absent is sticky: parsePrefixMeta rejects a declaration inside
			// an absent prefix, so nothing longer can ever override it.
			if meta.absent {
				finalMeta.absent = true
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

	pm, err := parsePrefixMeta(t.Args)
	if err != nil {
		return failure(err)
	}
	// log.Debug("prefix meta", pm)

	tr.Notify = make(map[string]bool)
	output := ""

	// A missing dst root used to be a hard failure, which is why deployments
	// open with an `install -d` shell block. Create it instead, with the root
	// prefix's umask and owner. An EXISTING dst is still left alone, so we
	// never change perms on / or $HOME.
	rootMeta := pm.getMeta(srcRoot)
	if ok, err := fsutil.Exists(dstRoot); err != nil {
		return failure("cannot read dst path", dstRoot, err)
	} else if !ok {
		mode := os.FileMode(0o777) &^ rootMeta.umaskOr()
		if err := fs.MkdirAll(dstRoot, mode); err != nil {
			return failure("cannot create dst path", dstRoot, err)
		}
		// MkdirAll reduces the mode by the process umask, so say it again.
		if err := fs.Chmod(dstRoot, mode); err != nil {
			return failure("cannot chmod dst path", dstRoot, err)
		}
		if _, err := chown(dstRoot, rootMeta.uid, rootMeta.gid); err != nil {
			return failure("cannot chown dst path", dstRoot, err)
		}
		tr.Changed = true
		output += fmt.Sprintf("%-7s %s\n", "created", dstRoot)
	}

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

	// Removals happen before the walk: a decommissioned file used to need a
	// shell block, because the walk only ever ensures that source paths
	// exist. Changed is reported only when something was really there.
	for _, prefix := range pm.orderedPrefixes {
		meta := pm.getMeta(prefix)
		if !meta.absent {
			continue
		}
		path := filepath.Join(dstRoot, prefix)
		ok, err := fsutil.Exists(path)
		if err != nil {
			return failure("cannot read", path, err)
		}
		if !ok {
			output += fmt.Sprintf("%-7s %s\n", "skip", path)
			continue
		}
		if err := fs.RemoveAll(path); err != nil {
			return failure("cannot remove", path, err)
		}
		tr.Changed = true
		for _, n := range meta.notify {
			tr.Notify[n] = true
		}
		output += fmt.Sprintf("%-7s %s\n", "removed", path)
	}

	err = afero.Walk(srcFs, srcRoot, func(srcPath string, srcFi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if srcPath == srcRoot {
			return nil // don't modify root element
		}
		dstPath := filepath.Join(dstRoot, srcPath)

		// prefix meta, if any, decides owner, mode, rendering and existence
		meta := pm.getMeta(srcPath)

		// Removed above; shipping it back would delete and recreate the same
		// path on every run.
		if meta.absent {
			if srcFi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		f := filesObj{
			path:  dstPath,
			isDir: srcFi.IsDir(),
			mode:  srcFi.Mode(),
			uid:   meta.uid,
			gid:   meta.gid,
		}

		if !f.isDir {
			f.data, err = afero.ReadFile(srcFs, srcPath)
			if err != nil {
				return fmt.Errorf("afero read rr on %s: %w", srcPath, err)
			}

			// template=false ships the subtree byte-for-byte. Rendering every
			// text file means a stray brace in third-party config, or a shell
			// ${VAR} beside a Gonja one, is a deploy-time failure for no gain.
			if meta.templating() && isText(f.data) {
				// log.Debug("parsing template", srcPath, "with vars", vars)
				f.data, err = tplParseBytes(f.data, t.Vars)
				if err != nil {
					return fmt.Errorf("tplParseBytes error on %s: %w", srcPath, err)
				}
			}

		}

		// apply umask to the source's own permissions
		f.mode = f.mode &^ meta.umaskOr()

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

		// A prefix changes only what it names: uid and gid stay nil when
		// owner or group are absent, so a line like
		// `/etc/nginx: template=false` no longer chowns that subtree to
		// root:root as a side effect of being mentioned. Every prefix line
		// that wants root ownership already spells out `owner=root
		// group=root`, which is how the old behaviour is reached.
		if username := attrs.String("owner"); username != "" {
			uid, err := lookupUID(username)
			if err != nil {
				return nil, err
			}
			fm.uid = &uid
		}

		if groupname := attrs.String("group"); groupname != "" {
			gid, err := lookupGID(groupname)
			if err != nil {
				return nil, err
			}
			fm.gid = &gid
		}

		if attrs.String("notify") != "" {
			fm.notify = parser.StringToSlice(attrs.String("notify"))
		}

		if s := attrs.String("template"); s != "" {
			b, err := strconv.ParseBool(s)
			if err != nil {
				return nil, fmt.Errorf("cannot parse template=%s as boolean for prefix %s", s, prefix)
			}
			fm.template = &b
		}

		switch state := attrs.String("state"); state {
		case "", "present":
			// present is the default: the prefix is shipped from the source
		case "absent":
			fm.absent = true
		default:
			return nil, fmt.Errorf("unknown state %s for prefix %s", state, prefix)
		}

		pm.metamap[prefix] = fm
	}

	for prefix := range pm.metamap {
		pm.orderedPrefixes = append(pm.orderedPrefixes, prefix)
	}
	slices.Sort(pm.orderedPrefixes) // sort prefixes to ensure shorted prefix is first

	// A prefix declared inside an absent one cannot mean anything: the removal
	// deletes that subtree, so an owner or a handler there would describe a
	// path which is not on the target. Say so rather than pick a winner.
	for _, outer := range pm.orderedPrefixes {
		if !pm.metamap[outer].absent {
			continue
		}
		for _, inner := range pm.orderedPrefixes {
			if inner != outer && strings.HasPrefix(inner, outer) {
				return nil, fmt.Errorf("prefix %s is inside absent prefix %s", inner, outer)
			}
		}
	}

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

	if err != nil && os.IsNotExist(err) {
		// create dir
		if err := fs.Mkdir(f.path, f.mode); err != nil {
			return false, fmt.Errorf("mkdir error on %s: %w", f.path, err)
		}
		changed = true
	} else if err != nil {
		return false, fmt.Errorf("read error on %s: %w", f.path, err)
	}

	if fi != nil && !fi.IsDir() {
		return false, fmt.Errorf("cannot overwrite non-dir %s with dir", f.path)
	}

	if fi != nil && fi.Mode().Perm() != f.mode.Perm() {
		log.Debug("chmod", fi.Mode().String(), "to", f.mode.String(), "for", f.path)
		if err := fs.Chmod(f.path, f.mode.Perm()); err != nil {
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
	if fi != nil && fi.Mode() != f.mode {
		log.Debug("chmod", fi.Mode().String(), "to", f.mode.String(), "for", f.path)
		changed = true
	}

	// need to write file?
	if os.IsNotExist(err) || dataDiffers() {
		// Create a temporary file in the same directory

		tempFile, err := fsutil.TempFile(filepath.Dir(f.path), "temp_*")
		if err != nil {
			return false, fmt.Errorf("create temp file error for %s: %w", f.path, err)
		}
		tempPath := tempFile.Name()
		defer func() {
			_ = tempFile.Close()
			_ = fs.Remove(tempPath)
		}()

		// Write data to the temporary file
		_, err = tempFile.Write(f.data)
		if err != nil {
			return false, fmt.Errorf("write error to temp file %s for %s: %w", tempPath, f.path, err)
		}

		if tempFile.Close() != nil {
			return false, fmt.Errorf("error closing temp file for %s: %w", f.path, err)
		}

		if fs.Chmod(tempPath, f.mode) != nil {
			return false, fmt.Errorf("chmod error on temp file %s for %s: %w", tempPath, f.path, err)
		}

		// Perform the atomic rename
		err = fs.Rename(tempPath, f.path)
		if err != nil {
			return false, fmt.Errorf("rename error from temp %s to %s: %w", tempPath, f.path, err)
		}

		changed = true
	}

	// need to change mode in case the file existed
	if fi != nil && fi.Mode() != f.mode {
		log.Debug("chmod", fi.Mode().String(), "to", f.mode.String(), "for", f.path)
		if err := fs.Chmod(f.path, f.mode); err != nil {
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
