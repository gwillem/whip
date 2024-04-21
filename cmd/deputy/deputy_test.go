//go:build integration
// +build integration

package main

import (
	"testing"

	"github.com/gwillem/whip/internal/playbook"
	"github.com/gwillem/whip/internal/runners"
	"github.com/gwillem/whip/internal/testutil"
	"github.com/stretchr/testify/require"
)

func Test_DeputyIntegration(t *testing.T) {
	pb, err := playbook.Load(testutil.FixturePath("playbook/simple.yml"))
	require.NoError(t, err)

	for _, play := range *pb {
		for _, task := range play.Tasks {
			res := runners.Run(&task, nil)
			require.Equal(t, runners.Failed, res.Status)
		}
	}
}
