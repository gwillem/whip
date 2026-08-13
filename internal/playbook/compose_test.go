package playbook

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gwillem/whip/internal/parser"
)

// A command is a command. Splitting it on "=" produced a task that ran, exited
// zero, and did something other than what was written.
func TestStringRunnerValueSurvivesEquals(t *testing.T) {
	pb, err := Parse([]byte(`
- hosts: [x]
  tasks:
    - shell: mysql -e "SET @a=1"
    - command: touch /tmp/a=b
    - apt: nginx state=absent
`), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tasks := (*pb)[0].Tasks
	if got, want := tasks[0].Args.String(parser.DefaultArg), `mysql -e "SET @a=1"`; got != want {
		t.Errorf("shell arg = %q, want %q", got, want)
	}
	if got, want := tasks[1].Args.String(parser.DefaultArg), "touch /tmp/a=b"; got != want {
		t.Errorf("command arg = %q, want %q", got, want)
	}
	// The key=value dialect is still what apt package names are written in.
	if got := tasks[2].Args.String("state"); got != "absent" {
		t.Errorf("apt lost its kv dialect: state=%q", got)
	}
}

func TestVarsFilesMergeUnderPlayVars(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "base.yml", "shared: from-base\noverridden: from-base\n")
	write(t, dir, "extra.yml", "overridden: from-extra\n")

	pb, err := Parse([]byte(`
- hosts: [x]
  vars_files: [base.yml, extra.yml]
  vars:
    own: from-play
    overridden: from-play
  tasks:
    - shell: "true"
`), dir)
	if err != nil {
		t.Fatal(err)
	}
	vars := (*pb)[0].Vars
	for k, want := range map[string]string{
		"shared":     "from-base",
		"own":        "from-play",
		"overridden": "from-play", // the playbook you can read wins
	} {
		if got, _ := vars[k].(string); got != want {
			t.Errorf("vars[%q] = %q, want %q", k, got, want)
		}
	}
}

func TestIncludeSplicesPlaysInOrder(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "middle.yml", "- name: from-include\n  hosts: [x]\n  tasks: [{shell: \"true\"}]\n")

	pb, err := Parse([]byte(`
- name: first
  hosts: [x]
  tasks: [{shell: "true"}]
- include: middle.yml
- name: last
  hosts: [x]
  tasks: [{shell: "true"}]
`), dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range *pb {
		names = append(names, p.Name)
	}
	want := []string{"first", "from-include", "last"}
	if len(names) != 3 || names[0] != want[0] || names[1] != want[1] || names[2] != want[2] {
		t.Errorf("plays = %v, want %v", names, want)
	}
}

// A cycle must be an error with a name in it, not an out-of-memory kill.
func TestIncludeCycleIsRefused(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.yml", "- include: b.yml\n")
	write(t, dir, "b.yml", "- include: a.yml\n")
	if _, err := Parse([]byte("- include: a.yml\n"), dir); err == nil {
		t.Fatal("expected an error for an include cycle")
	}
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
