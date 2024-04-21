/*

https://manpages.ubuntu.com/manpages/xenial/man8/apt.8.html

    Performs the requested action on one or more packages specified via regex(7),
    glob(7) or exact match. The requested action can be overridden for
    specific packages by append a plus (+) to the package name to install
    this package or a minus (-) to remove it.

*/

package runners

import (
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/stretchr/testify/assert"
)

func Test_buildAptCmd(t *testing.T) {
	args := model.TaskArgs{
		"name":  []string{"foo", "bar"},
		"state": "latest",
	}
	// want := aptPkgState(aptPkgState{"install": map[string]bool{"bar": true, "foo": true}})
	want := aptPkgState{"install": map[string]bool{"bar": true, "foo": true}}
	got, err := buildWanted(args)
	assert.NoError(t, err)
	assert.Equal(t, want, got)
}
