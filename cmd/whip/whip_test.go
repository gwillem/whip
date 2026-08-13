package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/playbook"
)

func TestParseExtraVars(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    model.Vars
		wantErr bool
	}{
		{"no flags", nil, nil, false},
		{"one pair", []string{"docroot=/srv/web"}, model.Vars{"docroot": "/srv/web"}, false},
		{
			"several pairs",
			[]string{"docroot=/srv/web", "db_name=magento"},
			model.Vars{"docroot": "/srv/web", "db_name": "magento"},
			false,
		},
		// A value may itself contain '=' (a password, a query string), so only
		// the first separator counts.
		{"value with equals", []string{"dsn=a=b"}, model.Vars{"dsn": "a=b"}, false},
		{"empty value is explicit", []string{"tag="}, model.Vars{"tag": ""}, false},
		{"later flag wins", []string{"a=1", "a=2"}, model.Vars{"a": "2"}, false},
		{"no separator", []string{"docroot"}, nil, true},
		{"no key", []string{"=value"}, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseExtraVars(c.args)
			if (err != nil) != c.wantErr {
				t.Fatalf("parseExtraVars(%v) err = %v, wantErr %v", c.args, err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if len(got) != len(c.want) {
				t.Fatalf("parseExtraVars(%v) = %v, want %v", c.args, got, c.want)
			}
			for k, v := range c.want {
				if got[k] != v {
					t.Errorf("var %q = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestApplyExtraVars(t *testing.T) {
	cases := []struct {
		name  string
		play  model.Play
		extra model.Vars
		want  map[string]any
	}{
		{
			// vars_files are merged into Vars by playbook.Load, so overriding
			// Vars overrides them too.
			name:  "extra beats play vars",
			play:  model.Play{Vars: map[string]any{"docroot": "/from/playbook", "web_user": "www-data"}},
			extra: model.Vars{"docroot": "/from/cli"},
			want:  map[string]any{"docroot": "/from/cli", "web_user": "www-data"},
		},
		{
			name:  "extra adds to a play without vars",
			play:  model.Play{},
			extra: model.Vars{"docroot": "/from/cli"},
			want:  map[string]any{"docroot": "/from/cli"},
		},
		{
			name:  "no extra vars leaves the play alone",
			play:  model.Play{Vars: map[string]any{"docroot": "/from/playbook"}},
			extra: nil,
			want:  map[string]any{"docroot": "/from/playbook"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Two plays, to prove every play is reached and not just the first.
			pb := model.Playbook{c.play, c.play}
			applyExtraVars(&pb, c.extra)
			for i, play := range pb {
				if len(play.Vars) != len(c.want) {
					t.Fatalf("play %d vars = %v, want %v", i, play.Vars, c.want)
				}
				for k, v := range c.want {
					if play.Vars[k] != v {
						t.Errorf("play %d var %q = %v, want %v", i, k, play.Vars[k], v)
					}
				}
			}
		})
	}
}

// TestExtraVarsPrecedenceOverLoadedPlaybook walks the whole chain a real run
// walks: vars_files are merged by the loader, the play's own vars beat them, and
// -e beats both. Written against the loader rather than a hand-built play,
// because the point of the flag is to override what a file said.
func TestExtraVarsPrecedenceOverLoadedPlaybook(t *testing.T) {
	dir := t.TempDir()
	varsFile := filepath.Join(dir, "common.yml")
	if err := os.WriteFile(varsFile, []byte("docroot: /from/varsfile\nweb_user: www-data\ndb_name: from_varsfile\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pbFile := filepath.Join(dir, "pb.yml")
	pbSrc := "- hosts: [web1]\n" +
		"  vars_files: [common.yml]\n" +
		"  vars:\n" +
		"    docroot: /from/play\n" +
		"  tasks: []\n"
	if err := os.WriteFile(pbFile, []byte(pbSrc), 0o600); err != nil {
		t.Fatal(err)
	}

	pb, err := playbook.Load(pbFile)
	if err != nil {
		t.Fatal(err)
	}

	extra, err := parseExtraVars([]string{"docroot=/from/cli", "db_name=from_cli"})
	if err != nil {
		t.Fatal(err)
	}
	applyExtraVars(pb, extra)

	want := map[string]any{
		"docroot":  "/from/cli", // -e beats vars, which beat vars_files
		"db_name":  "from_cli",  // -e beats vars_files directly
		"web_user": "www-data",  // untouched by -e
	}
	for k, v := range want {
		if got := (*pb)[0].Vars[k]; got != v {
			t.Errorf("var %q = %v, want %v", k, got, v)
		}
	}
}

func TestPrefixWriter(t *testing.T) {
	cases := []struct {
		name   string
		writes []string
		want   string
	}{
		{"whole lines", []string{"one\ntwo\n"}, "p one\np two\n"},
		// A build tool writes in arbitrary chunks; a line split across two
		// writes must still be prefixed exactly once.
		{"split line", []string{"on", "e\n"}, "p one\n"},
		{"trailing partial needs flush", []string{"no newline"}, "p no newline\n"},
		{"nothing written", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			w := &prefixWriter{w: buf, prefix: "p "}
			for _, s := range c.writes {
				n, err := w.Write([]byte(s))
				if err != nil || n != len(s) {
					t.Fatalf("Write(%q) = %d, %v", s, n, err)
				}
			}
			w.Flush()
			if got := buf.String(); got != c.want {
				t.Errorf("output = %q, want %q", got, c.want)
			}
		})
	}
}
