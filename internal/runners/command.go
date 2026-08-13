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
	return systemTask(t, tokens)
}

func init() {
	// stringArg: the value is an argv, parsed by shlex, not by the key=value
	// splitter. `command: touch /tmp/a=b` used to lose most of itself.
	registerRunner("command", runner{
		run:  Command,
		meta: runnerMeta{stringArg: parser.DefaultArg},
	})
}
