package parser

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/gwillem/whip/internal/model"
)

const (
	DefaultArg = "_args"
)

// Helper functions for whip + deputy
// todo, should take quotes into account, plus only take simple x=y pairs
func ParseArgString(arg string) model.TaskArgs {
	kv := map[string]any{}

	baseArgs := []string{}
	for _, t := range strings.Split(arg, " ") {
		if strings.Contains(t, "=") {
			opt := strings.SplitN(t, "=", 2)

			kv[opt[0]] = unquote(opt[1])
		} else {
			baseArgs = append(baseArgs, t)
		}
	}

	kv[DefaultArg] = strings.Join(baseArgs, " ")
	return kv
}

func unquote(s string) string {
	if n, e := strconv.Unquote(s); e == nil {
		return n
	}
	return s
}

var commaSep = regexp.MustCompile(`,\s*`)

func StringToSlice(s string) []string {
	return commaSep.Split(s, -1)
}
