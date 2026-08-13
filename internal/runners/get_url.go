package runners

import (
	"bytes"
	"os"

	"github.com/gwillem/urlfilecache"
	"github.com/gwillem/whip/internal/model"
)

func init() {
	registerRunner("get_url", runner{
		run: getURL,
		meta: runnerMeta{
			requiredArgs: []string{"url", "dest"},
			optionalArgs: []string{},
		},
	})
}

func getURL(t *model.Task) (tr model.TaskResult) {
	url := t.Args.String("url")
	dest := t.Args.String("dest")

	if url == "" || dest == "" {
		return failure("url and dest are required arguments")
	}

	hash, _ := getFileChecksum(fs, dest) // could not exist yet

	if _, e := urlfilecache.ToPath(url, urlfilecache.WithPath(dest)); e != nil {
		return failure("failed to get_url:", e)
	}

	newHash, err := getFileChecksum(fs, dest)
	if err != nil {
		// The fetcher keeps ETag and Last-Modified sidecars in its own cache
		// directory, so a dest deleted by hand can be answered with a 304 and
		// never written. Say that, instead of blaming the checksum.
		if os.IsNotExist(err) {
			return failure(dest, "was not written: the server reported no change against a stale cache entry")
		}
		return failure("failed to get hash for new file:", dest, err)
	}

	// Status was never set, so a real download arrived in the report as "ok"
	// rather than "changed": the summary only counts Changed && Success, and a
	// notified handler never fired.
	tr.Status = Success
	tr.Changed = !bytes.Equal(hash, newHash)
	return tr
}
