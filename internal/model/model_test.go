package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_TaskArgsStringSlice(t *testing.T) {
	ta := TaskArgs{"names": []any{"bar", "baz"}}
	got := ta.StringSlice("names")[0]
	assert.Equal(t, "bar", got)

	ta = TaskArgs{"names": "foo"}
	got = ta.StringSlice("names")[0]
	assert.Equal(t, "foo", got)
}
