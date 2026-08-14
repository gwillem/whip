package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_uploadHint(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{
			// what a minimal image without xz-utils actually reports
			name: "missing xz gets the package name",
			err:  errors.New(`remote command failed: xz -d > .cache/whip/deputy.tmp: Process exited with status 127: bash: line 1: xz: command not found`),
			want: " (the target needs xz, from the xz-utils or xz package)",
		},
		{
			name: "bare 127 is enough, some shells say nothing",
			err:  errors.New("remote command failed: xz -d > x: Process exited with status 127"),
			want: " (the target needs xz, from the xz-utils or xz package)",
		},
		{
			name: "other failures get no hint",
			err:  errors.New("remote command failed: xz -d > x: Process exited with status 1: No space left on device"),
			want: "",
		},
	} {
		require.Equal(t, tc.want, uploadHint(tc.err), tc.name)
	}
}
