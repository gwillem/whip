package runners

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gwillem/whip/internal/model"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_buildWanted(t *testing.T) {
	cases := []struct {
		name string
		args model.TaskArgs
		want aptPkgState
		err  bool
	}{
		{
			name: "task state with a per-package override",
			args: model.TaskArgs{
				"name":  []string{"foo", "bar", "mlocate state=absent"},
				"state": "present",
			},
			want: aptPkgState{
				"install": map[string]bool{"foo": true, "bar": true},
				"remove":  map[string]bool{"mlocate": true},
			},
		},
		{
			name: "latest is its own state, not a silent present",
			args: model.TaskArgs{"name": []string{"foo"}, "state": "latest"},
			want: aptPkgState{"latest": map[string]bool{"foo": true}},
		},
		{
			name: "a version pin survives as one package name",
			args: model.TaskArgs{"name": []string{"php8.2-fpm=8.2.1-1", "nginx"}},
			want: aptPkgState{"install": map[string]bool{"php8.2-fpm=8.2.1-1": true, "nginx": true}},
		},
		{
			name: "a pinned package may still override the state",
			args: model.TaskArgs{"name": []string{"nginx=1.24.0-1 state=purged"}},
			want: aptPkgState{"purge": map[string]bool{"nginx=1.24.0-1": true}},
		},
		{
			name: "several packages in one entry",
			args: model.TaskArgs{"name": "curl gnupg"},
			want: aptPkgState{"install": map[string]bool{"curl": true, "gnupg": true}},
		},
		{
			name: "no state means present",
			args: model.TaskArgs{"name": []string{"foo"}},
			want: aptPkgState{"install": map[string]bool{"foo": true}},
		},
		{
			name: "an unknown task state is an error",
			args: model.TaskArgs{"name": []string{"foo"}, "state": "instaled"},
			err:  true,
		},
		{
			name: "an unknown per-package state is an error",
			args: model.TaskArgs{"name": []string{"foo state=gone"}},
			err:  true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := buildWanted(c.args)
			if c.err {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func Test_splitPkgSpec(t *testing.T) {
	cases := []struct{ spec, name, version string }{
		{"nginx", "nginx", ""},
		{"nginx=1.24.0-1", "nginx", "1.24.0-1"},
		{"php8.2-fpm=1:8.2.1-1ubuntu2", "php8.2-fpm", "1:8.2.1-1ubuntu2"},
	}
	for _, c := range cases {
		name, version := splitPkgSpec(c.spec)
		assert.Equal(t, c.name, name, c.spec)
		assert.Equal(t, c.version, version, c.spec)
	}
}

func Test_aptWorklist(t *testing.T) {
	inv := aptInventory{
		installed: map[string]string{
			"nginx":   "1.24.0-1",
			"apache2": "2.4.58-1",
			"curl":    "8.5.0-2",
		},
		upgradable: map[string]bool{"curl": true},
	}

	cases := []struct {
		name   string
		wanted aptPkgState
		want   aptPkgState
	}{
		{
			name: "a converged host has nothing to do",
			wanted: aptPkgState{
				"install": map[string]bool{"nginx": true, "nginx=1.24.0-1": true},
				"remove":  map[string]bool{"mlocate": true},
				"purge":   map[string]bool{"snapd": true},
			},
			want: aptPkgState{},
		},
		{
			name:   "a missing package is installed",
			wanted: aptPkgState{"install": map[string]bool{"jq": true}},
			want:   aptPkgState{"install": map[string]bool{"jq": true}},
		},
		{
			name:   "a pin that does not match the installed version is work",
			wanted: aptPkgState{"install": map[string]bool{"nginx=1.25.3-1": true}},
			want:   aptPkgState{"install": map[string]bool{"nginx=1.25.3-1": true}},
		},
		{
			name:   "latest is work only when apt has a newer candidate",
			wanted: aptPkgState{"latest": map[string]bool{"curl": true, "nginx": true}},
			want:   aptPkgState{"latest": map[string]bool{"curl": true}},
		},
		{
			name:   "an installed package is purged, by bare name",
			wanted: aptPkgState{"purge": map[string]bool{"apache2=2.4.58-1": true}},
			want:   aptPkgState{"purge": map[string]bool{"apache2": true}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, aptWorklist(c.wanted, inv))
		})
	}
}

func Test_aptCommands(t *testing.T) {
	cases := []struct {
		name    string
		work    aptPkgState
		release string
		want    [][]string
	}{
		{
			name: "an empty worklist runs no apt",
			work: aptPkgState{},
			want: [][]string{},
		},
		{
			name: "present and latest share one install",
			work: aptPkgState{
				"install": map[string]bool{"nginx": true, "curl": true},
				"latest":  map[string]bool{"jq": true},
			},
			want: [][]string{{
				"/usr/bin/apt-get", "install", "-y", "-q",
				"-o", "Dpkg::Options::=--force-confdef",
				"-o", "Dpkg::Options::=--force-confold",
				"curl", "jq", "nginx",
			}},
		},
		{
			name: "remove and purge are separate transactions",
			work: aptPkgState{
				"remove": map[string]bool{"mlocate": true},
				"purge":  map[string]bool{"snapd": true, "avahi-daemon": true},
			},
			want: [][]string{
				{
					"/usr/bin/apt-get", "remove", "-y", "-q",
					"-o", "Dpkg::Options::=--force-confdef",
					"-o", "Dpkg::Options::=--force-confold",
					"mlocate",
				},
				{
					"/usr/bin/apt-get", "purge", "-y", "-q",
					"-o", "Dpkg::Options::=--force-confdef",
					"-o", "Dpkg::Options::=--force-confold",
					"avahi-daemon", "snapd",
				},
			},
		},
		{
			name:    "default_release becomes -t on the install",
			work:    aptPkgState{"install": map[string]bool{"nginx=1.24.0-1": true}},
			release: "noble-backports",
			want: [][]string{{
				"/usr/bin/apt-get", "install", "-y", "-q",
				"-o", "Dpkg::Options::=--force-confdef",
				"-o", "Dpkg::Options::=--force-confold",
				"-t", "noble-backports",
				"nginx=1.24.0-1",
			}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, aptCommands(c.work, c.release))
		})
	}
}

func Test_parseAptList(t *testing.T) {
	out := `Listing...
nginx/noble,now 1.24.0-2ubuntu7 amd64 [installed]
zlib1g/now 1:1.3.dfsg-3.1ubuntu2 amd64 [installed,local]

curl/noble-updates 8.5.0-2ubuntu10.6 amd64 [upgradable from: 8.5.0-2ubuntu10]
`
	want := map[string]string{
		"nginx":  "1.24.0-2ubuntu7",
		"zlib1g": "1:1.3.dfsg-3.1ubuntu2",
		"curl":   "8.5.0-2ubuntu10.6",
	}
	assert.Equal(t, want, parseAptList(out))
}

func Test_buildInventory(t *testing.T) {
	oldRun := aptRun
	defer func() { aptRun = oldRun }()

	aptRun = func(cmd []string) (string, string, error) {
		switch cmd[2] {
		case "--installed":
			return "Listing...\nnginx/noble,now 1.24.0-1 amd64 [installed]\ncurl/noble,now 8.5.0-2 amd64 [installed]\n", "", nil
		case "--upgradable":
			return "Listing...\ncurl/noble-updates 8.5.0-3 amd64 [upgradable from: 8.5.0-2]\n", "", nil
		}
		return "", "", errors.New("unexpected " + strings.Join(cmd, " "))
	}

	inv, err := buildInventory()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"nginx": "1.24.0-1", "curl": "8.5.0-2"}, inv.installed)
	assert.Equal(t, map[string]bool{"curl": true}, inv.upgradable)

	// The upgradable set is what makes `state: latest` idempotent.
	work := aptWorklist(aptPkgState{"latest": map[string]bool{"nginx": true, "curl": true}}, inv)
	assert.Equal(t, [][]string{{
		aptBin, "install", "-y", "-q",
		"-o", "Dpkg::Options::=--force-confdef",
		"-o", "Dpkg::Options::=--force-confold",
		"curl",
	}}, aptCommands(work, ""))
}

func Test_shouldUpdateCache(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name     string
		args     model.TaskArgs
		stampAge time.Duration // negative: no stamp on disk at all
		want     bool
	}{
		{
			name:     "no update_cache means no refresh",
			args:     model.TaskArgs{},
			stampAge: time.Hour,
			want:     false,
		},
		{
			name:     "update_cache without a validity always refreshes",
			args:     model.TaskArgs{"update_cache": true},
			stampAge: time.Minute,
			want:     true,
		},
		{
			name:     "a fresh index is left alone",
			args:     model.TaskArgs{"update_cache": true, "cache_valid_time": 3600},
			stampAge: 10 * time.Minute,
			want:     false,
		},
		{
			name:     "a stale index is refreshed",
			args:     model.TaskArgs{"update_cache": true, "cache_valid_time": 3600},
			stampAge: 2 * time.Hour,
			want:     true,
		},
		{
			name:     "no index at all is refreshed",
			args:     model.TaskArgs{"update_cache": true, "cache_valid_time": 3600},
			stampAge: -1,
			want:     true,
		},
		{
			name:     "the key=value dialect is understood too",
			args:     model.TaskArgs{"update_cache": "true", "cache_valid_time": "3600"},
			stampAge: 10 * time.Minute,
			want:     false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			oldFs, oldFsutil := fs, fsutil
			defer func() { fs, fsutil = oldFs, oldFsutil }()
			fs = afero.NewMemMapFs()
			fsutil = &afero.Afero{Fs: fs}

			if c.stampAge >= 0 {
				require.NoError(t, fsutil.WriteFile(aptStampFile, []byte("stamp"), 0o644))
				require.NoError(t, fs.Chtimes(aptStampFile, now, now.Add(-c.stampAge)))
			}

			got, err := shouldUpdateCache(c.args, now)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func Test_shouldUpdateCacheRejectsNonsense(t *testing.T) {
	_, err := shouldUpdateCache(model.TaskArgs{"update_cache": "maybe"}, time.Now())
	assert.Error(t, err)

	_, err = shouldUpdateCache(model.TaskArgs{"update_cache": true, "cache_valid_time": "1h"}, time.Now())
	assert.Error(t, err)
}

func Test_runAptCommandsReportsStderr(t *testing.T) {
	oldRun := aptRun
	defer func() { aptRun = oldRun }()

	ran := [][]string{}
	aptRun = func(cmd []string) (string, string, error) {
		ran = append(ran, cmd)
		if cmd[1] == "purge" {
			return "Reading package lists...\n", "E: Internal error, problem resolver broke stuff\n",
				errors.New("exit status 100")
		}
		return "Setting up nginx\n", "", nil
	}

	cmds := [][]string{
		{aptBin, "install", "-y", "-q", "nginx"},
		{aptBin, "purge", "-y", "-q", "snapd"},
		{aptBin, "remove", "-y", "-q", "mlocate"},
	}
	out, err := runAptCommands(cmds)

	require.Error(t, err)
	// The exit code alone is what whip used to report; apt's own complaint is
	// the only thing that names the problem.
	assert.Contains(t, err.Error(), "E: Internal error, problem resolver broke stuff")
	assert.Contains(t, err.Error(), "apt-get purge -y -q snapd")
	assert.Contains(t, out, "Setting up nginx")
	// A failing transaction stops the run: the third command never happens.
	assert.Len(t, ran, 2)
}
