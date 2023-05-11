package whip

import (
	"testing"

	"github.com/k0kubun/pp"
	"github.com/stretchr/testify/assert"
)

func TestLoadPlaybook(t *testing.T) {
	pb := LoadPlaybook(FixturePath("playbook/simple.yml"))
	assert.NotNil(t, pb)
	assert.Len(t, pb[0].Hosts, 2)
	pp.Print(pb)
}
