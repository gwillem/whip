package main

import (
	"testing"

	"github.com/gwillem/whip/internal/runners"
	"github.com/gwillem/whip/internal/whip"
	"github.com/k0kubun/pp"
	"github.com/stretchr/testify/assert"
)

func Test_DeputyIntegration(t *testing.T) {
	pb, err := whip.LoadPlaybook(whip.FixturePath("playbook/simple.yml"))
	assert.NoError(t, err)
	pp.Println(pb)

	for _, play := range *pb {
		for _, task := range play.Tasks {
			res := runners.Run(task, nil)

			pp.Print(res)
			assert.Equal(t, 0, res.Status)
		}
	}
}
