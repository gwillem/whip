package parser

import (
	"strconv"
	"strings"
)

// Helper functions for whip + deputy

func ParseArgString(arg string) map[string]string {
	kv := map[string]string{}

	baseArgs := []string{}
	for _, t := range strings.Split(arg, " ") {
		if strings.Contains(t, "=") {
			opt := strings.SplitN(t, "=", 2)

			kv[opt[0]] = unquote(opt[1])
		} else {
			baseArgs = append(baseArgs, t)
		}
	}

	kv["_args"] = strings.Join(baseArgs, " ")
	return kv
}

func unquote(s string) string {
	if n, e := strconv.Unquote(s); e == nil {
		return n
	}
	return s
}
