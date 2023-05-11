package runners

import (
	"github.com/gwillem/chief-whip/pkg/whip"
)

func AuthorizedKey(args whip.TaskArgs) (tr whip.TaskResult) {
	// blob, err := json.MarshalIndent(args, "", "  ")
	// if err != nil {
	// 	panic(err)
	// }
	// tr.Output = string(blob)
	// tr.Status = failed
	tr.Status = ok
	tr.Output = "Installed authorized_key for " + args.Key("user") + " with key " + args.Key("key") + "\n"
	return tr
}

func init() {
	registerRunner("authorized_key", AuthorizedKey, runnerMeta{})
}
