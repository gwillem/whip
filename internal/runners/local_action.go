package runners

import (
	"fmt"

	"github.com/gwillem/whip/internal/model"
)

func LocalAction(t *model.Task) (tr model.TaskResult) {
	tr.Output = fmt.Sprint("ran local action with args:", t.Args)
	// pp.Println(t.Args)
	return tr
}

func init() {
	registerRunner("local_action", runner{local: LocalAction})
}
