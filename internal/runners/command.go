package runners

import (
	"github.com/google/shlex"
	log "github.com/gwillem/go-simplelog"
	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/parser"
)

func Command(t *model.Task) (tr model.TaskResult) {
	log.Debug("command args:", t.Args)
	tokens, err := shlex.Split(t.Args.String(parser.DefaultArg))
	if err != nil {
		tr.Status = Failed
		tr.Output = err.Error()
		return tr
	}
	return system(tokens)
}

func init() {
	registerRunner("command", runner{run: Command})
}
