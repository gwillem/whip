package runners

import (
	"bytes"

	"github.com/gwillem/urlfilecache"
	"github.com/gwillem/whip/internal/model"
)

func init() {
	registerRunner("get_url", runner{run: getURL})
}

func getURL(t *model.Task) (tr model.TaskResult) {
	url := t.Args.String("url")
	dest := t.Args.String("dest")

	if url == "" || dest == "" {
		return failure("url and dest are required arguments")
	}

	hash, _ := getFileChecksum(fs, dest) // could not exist yet

	if e := urlfilecache.ToCustomPath(url, dest); e != nil {
		return failure("failed to get_url:", e)
	}

	newHash, err := getFileChecksum(fs, dest)
	if err != nil {
		return failure("failed to get hash for new file:", dest, err)
	}

	tr.Changed = !bytes.Equal(hash, newHash)
	return tr
}
