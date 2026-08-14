package runners

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/parser"
)

func shellTask(cmd string) *model.Task {
	return &model.Task{
		Runner: "shell",
		Args:   model.TaskArgs{parser.DefaultArg: cmd},
		Vars:   model.TaskVars{},
	}
}

// creates and removes say "has this already happened" without a shell, which
// is what the overwhelming majority of `unless` guards were written to say.
func TestCreatesAndRemovesSkipWithoutAShell(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "present")
	absent := filepath.Join(dir, "absent")
	if err := os.WriteFile(present, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	witness := filepath.Join(dir, "witness")

	run := func(task *model.Task) model.TaskResult {
		return Run(task, model.TaskVars{})
	}

	t.Run("creates skips when the path exists", func(t *testing.T) {
		task := shellTask("touch " + witness)
		task.Creates = present
		tr := run(task)
		if !strings.Contains(tr.Output, "already exists") {
			t.Errorf("output = %q, want a skip", tr.Output)
		}
		if _, err := os.Stat(witness); err == nil {
			t.Error("the task ran despite its creates guard")
		}
	})

	t.Run("creates runs when the path does not", func(t *testing.T) {
		task := shellTask("touch " + witness)
		task.Creates = absent
		if tr := run(task); tr.Status != Success {
			t.Fatalf("status %d: %s", tr.Status, tr.Output)
		}
		if _, err := os.Stat(witness); err != nil {
			t.Error("the task was skipped although its creates path was absent")
		}
		os.Remove(witness)
	})

	t.Run("removes skips when the path is already gone", func(t *testing.T) {
		task := shellTask("touch " + witness)
		task.Removes = absent
		tr := run(task)
		if !strings.Contains(tr.Output, "already absent") {
			t.Errorf("output = %q, want a skip", tr.Output)
		}
	})
}

// The guard used to be evaluated straight from the playbook, so a variable in
// it reached /bin/sh as literal braces and silently tested the wrong thing.
func TestUnlessIsTemplated(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	if err := os.WriteFile(marker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	witness := filepath.Join(dir, "witness")

	task := shellTask("touch " + witness)
	task.Unless = "test -f {{ marker }}"
	task.Vars = model.TaskVars{"marker": marker}

	tr := Run(task, model.TaskVars{})
	if !strings.Contains(tr.Output, "skipped") {
		t.Fatalf("guard did not fire: %q", tr.Output)
	}
	if !strings.Contains(tr.Output, marker) {
		t.Errorf("the reported guard is unrendered: %q", tr.Output)
	}
	if _, err := os.Stat(witness); err == nil {
		t.Error("the task ran although its templated guard was true")
	}
}

// A variable inside a list or a map used to survive as literal text, because
// only top-level string arguments were rendered.
func TestNestedArgumentsAreTemplated(t *testing.T) {
	task := &model.Task{
		Runner: "shell",
		Args: model.TaskArgs{
			parser.DefaultArg: "true",
			"list":            []any{"a-{{ v }}", "b-{{ v }}"},
			"nested":          map[string]any{"k": "c-{{ v }}"},
		},
		Vars: model.TaskVars{"v": "rendered"},
	}
	if tr := Run(task, model.TaskVars{}); tr.Status != Success {
		t.Fatalf("status %d: %s", tr.Status, tr.Output)
	}
	list, _ := task.Args["list"].([]any)
	if len(list) != 2 || list[0] != "a-rendered" || list[1] != "b-rendered" {
		t.Errorf("list args unrendered: %v", task.Args["list"])
	}
	nested, _ := task.Args["nested"].(map[string]any)
	if nested["k"] != "c-rendered" {
		t.Errorf("map args unrendered: %v", task.Args["nested"])
	}
}

// A `{{ }}` inside a loop item used to reach the target as literal text; this
// wrote "innodb_buffer_pool_size = {{ pool }}M" into a live my.cnf.
func TestLoopItemIsRenderedBeforeSubstitution(t *testing.T) {
	task := &model.Task{
		Runner: "shell",
		Args:   model.TaskArgs{parser.DefaultArg: "echo {{ item }}"},
		Vars:   model.TaskVars{"item": "pool = {{ size }}M", "size": 250},
	}
	tr := Run(task, model.TaskVars{})
	if tr.Status != Success {
		t.Fatalf("status %d: %s", tr.Status, tr.Output)
	}
	if got := strings.TrimSpace(tr.Output); got != "pool = 250M" {
		t.Errorf("output = %q, want %q", got, "pool = 250M")
	}
}

// A guard naming a variable is the one place where getting templating wrong is
// invisible: an unrendered "{{ dir }}/lock" cannot exist, so the guard never
// fires and the task runs on every converge while reporting success.
func TestPathGuardsAreTemplated(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "lock")
	if err := os.WriteFile(lock, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	vars := model.TaskVars{"dir": dir}

	creates := &model.Task{
		Runner:  "shell",
		Args:    model.TaskArgs{"_args": "true"},
		Vars:    vars,
		Creates: "{{ dir }}/lock",
	}
	skip, why, err := shouldSkip(creates)
	if err != nil {
		t.Fatalf("creates guard errored: %v", err)
	}
	if !skip {
		t.Errorf("creates: %q exists, want skip, got run", lock)
	}
	if strings.Contains(why, "{{") {
		t.Errorf("creates: reason still holds a template: %q", why)
	}

	removes := &model.Task{
		Runner:  "shell",
		Args:    model.TaskArgs{"_args": "true"},
		Vars:    vars,
		Removes: "{{ dir }}/absent",
	}
	if skip, _, err = shouldSkip(removes); err != nil {
		t.Fatalf("removes guard errored: %v", err)
	}
	if !skip {
		t.Error("removes: path is already absent, want skip, got run")
	}

	// And the rendered path must be the one that decides, not any path.
	present := &model.Task{
		Runner:  "shell",
		Args:    model.TaskArgs{"_args": "true"},
		Vars:    vars,
		Creates: "{{ dir }}/not-there",
	}
	if skip, _, err = shouldSkip(present); err != nil {
		t.Fatalf("creates guard errored: %v", err)
	}
	if skip {
		t.Error("creates: path does not exist, want run, got skip")
	}
}
