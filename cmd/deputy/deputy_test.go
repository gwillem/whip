package main

import (
	"testing"

	"github.com/gwillem/chief-whip/pkg/runners"
	"github.com/gwillem/chief-whip/pkg/whip"
	"github.com/k0kubun/pp"
	"gotest.tools/assert"
)

func Test_DeputyIntegration(t *testing.T) {
	pb := whip.LoadPlaybook(whip.FixturePath("playbook/simple.yml"))
	pp.Println(pb)

	for _, play := range pb {
		for _, task := range play.Tasks {
			res := runners.Run(task)

			pp.Print(res)
			assert.Equal(t, 0, res.Status)
		}

	}

}
