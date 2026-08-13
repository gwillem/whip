package runners

import (
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/stretchr/testify/require"
)

// systemctl exits zero without doing anything when the unit is already in the
// requested state, and the runner reported that as a change, so every notified
// handler fired on every deploy.
func TestServiceIsNoop(t *testing.T) {
	for _, tc := range []struct {
		verb   string
		loaded bool
		active bool
		want   bool
	}{
		{verb: "start", loaded: true, active: true, want: true},
		{verb: "start", loaded: true, active: false, want: false},
		{verb: "stop", loaded: true, active: false, want: true},
		{verb: "stop", loaded: true, active: true, want: false},
		{verb: "restart", loaded: true, active: true, want: false},
		{verb: "reload", loaded: true, active: true, want: false},
		// A typo'd unit name must reach systemctl and fail there, rather than
		// being reported as a state already satisfied.
		{verb: "start", loaded: false, active: false, want: false},
		{verb: "stop", loaded: false, active: false, want: false},
	} {
		got := serviceIsNoop(tc.verb, tc.loaded, tc.active)
		require.Equal(t, tc.want, got, "%s loaded=%v active=%v", tc.verb, tc.loaded, tc.active)
	}
}

func TestParseUnitState(t *testing.T) {
	for _, tc := range []struct {
		name       string
		out        string
		loaded     bool
		active     bool
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:   "a running unit",
			out:    "LoadState=loaded\nActiveState=active\n",
			loaded: true, active: true,
		},
		{
			name:   "a stopped unit",
			out:    "LoadState=loaded\nActiveState=inactive\n",
			loaded: true, active: false,
		},
		{
			name:   "no such unit",
			out:    "LoadState=not-found\nActiveState=inactive\n",
			loaded: false, active: false,
		},
		{
			// Read by key, because the order systemd answers in is its own
			// business.
			name:   "properties in the other order",
			out:    "ActiveState=active\nLoadState=loaded",
			loaded: true, active: true,
		},
		{
			// A unit still coming up needs no second start.
			name:   "activating counts as active",
			out:    "LoadState=loaded\nActiveState=activating\n",
			loaded: true, active: true,
		},
		{
			name:    "a failed unit is loaded and not active",
			out:     "LoadState=loaded\nActiveState=failed\n",
			loaded:  true,
			active:  false,
			wantErr: false,
		},
		{
			name:    "output we do not understand is an error, not a guess",
			out:     "Unit nginx.service could not be found.\n",
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loaded, active, err := parseUnitState(tc.out)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.loaded, loaded)
			require.Equal(t, tc.active, active)
		})
	}
}

// The runner declares name as required but nothing enforced it, so an empty
// name ran `systemctl start` and failed with systemd's own complaint.
func TestServiceValidatesItsArguments(t *testing.T) {
	t.Run("a missing name", func(t *testing.T) {
		tr := Service(&model.Task{Runner: "service", Args: model.TaskArgs{"state": "started"}})
		require.Equal(t, Failed, tr.Status)
		require.Contains(t, tr.Output, "name is a required argument")
	})

	t.Run("an unknown state", func(t *testing.T) {
		tr := Service(&model.Task{Runner: "service", Args: model.TaskArgs{
			"name": "nginx", "state": "reboot",
		}})
		require.Equal(t, Failed, tr.Status)
		require.Contains(t, tr.Output, "unknown state")
	})
}
