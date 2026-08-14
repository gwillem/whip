package playbook

import (
	"strings"
	"testing"
)

// A key whip does not recognise used to be logged at a level cmd/whip
// suppresses, so a misspelled field simply did nothing: `unles:` never
// guarded anything and the deploy reported success.
func TestUnknownFieldIsRefused(t *testing.T) {
	_, err := Parse([]byte(`
- hosts: [x]
  tasks:
    - name: typo
      shell: "true"
      unles: test -e /nope
`), t.TempDir())
	if err == nil {
		t.Fatal("expected an error for a misspelled task field")
	}
	if !strings.Contains(err.Error(), "unles") {
		t.Errorf("the error does not name the offending field: %v", err)
	}
}

// notify naming a handler that does not exist reads exactly like a handler
// that did not need to fire.
func TestNotifyMustNameAHandler(t *testing.T) {
	_, err := Parse([]byte(`
- hosts: [x]
  handlers:
    - name: reload nginx
      shell: "true"
  tasks:
    - name: ship config
      shell: "true"
      notify: [relaod nginx]
`), t.TempDir())
	if err == nil {
		t.Fatal("expected an error for a notify with no matching handler")
	}
	if !strings.Contains(err.Error(), "relaod nginx") {
		t.Errorf("the error does not name the missing handler: %v", err)
	}
}

func TestNotifyToARealHandlerIsAccepted(t *testing.T) {
	if _, err := Parse([]byte(`
- hosts: [x]
  handlers:
    - name: reload nginx
      shell: "true"
  tasks:
    - name: ship config
      shell: "true"
      notify: [reload nginx]
`), t.TempDir()); err != nil {
		t.Fatalf("a correct notify was refused: %v", err)
	}
}

// requiredArgs and optionalArgs were declared on the runners and read by
// nothing, so an argument name typo was silently dropped.
func TestUnknownRunnerArgumentIsRefused(t *testing.T) {
	_, err := Parse([]byte(`
- hosts: [x]
  tasks:
    - name: start it
      service:
        name: nginx
        stat: started
`), t.TempDir())
	if err == nil {
		t.Fatal("expected an error for an argument the service runner does not read")
	}
	if !strings.Contains(err.Error(), "stat") {
		t.Errorf("the error does not name the argument: %v", err)
	}
}

// Runners that declare no argument list keep their freedom; refusing what
// they were not told about would reject working playbooks.
func TestRunnersWithoutDeclaredArgsAreNotPoliced(t *testing.T) {
	if _, err := Parse([]byte(`
- hosts: [x]
  tasks:
    - shell:
        _args: "true"
        changed_when: "false"
`), t.TempDir()); err != nil {
		t.Fatalf("an undeclared-arg runner was policed: %v", err)
	}
}

// whip keeps Ansible's verbiage so a playbook can be moved across without a
// rewrite, and its own fixtures carry these keys.
func TestAnsibleNoOpKeysStillLoad(t *testing.T) {
	if _, err := Parse([]byte(`
- hosts: [x]
  remote_user: root
  gather_facts: false
  tasks:
    - shell: "true"
`), t.TempDir()); err != nil {
		t.Fatalf("an Ansible no-op keyword was refused: %v", err)
	}
}

// Ignoring these silently is the dangerous option: a play written with
// `become: true` and run without it does the wrong work as the wrong user and
// then reports success.
func TestAnsibleKeysThatChangeBehaviourAreRefused(t *testing.T) {
	for _, key := range []string{"become", "sudo", "when", "vars_prompt"} {
		src := "- hosts: [x]\n  " + key + ": true\n  tasks:\n    - shell: \"true\"\n"
		if key == "when" {
			src = "- hosts: [x]\n  tasks:\n    - shell: \"true\"\n      when: something\n"
		}
		_, err := Parse([]byte(src), t.TempDir())
		if err == nil {
			t.Errorf("%s was accepted and ignored", key)
			continue
		}
		if !strings.Contains(err.Error(), key) {
			t.Errorf("%s: error does not name the key: %v", key, err)
		}
	}
}
