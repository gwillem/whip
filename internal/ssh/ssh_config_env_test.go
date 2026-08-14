package ssh

import (
	"os"
	"path/filepath"
	"testing"
)

// A repository that reaches its fleet through a bastion should be able to
// ship the ProxyJump with the playbooks, where it is reviewed, rather than
// asking every operator to edit their own dotfiles.
func TestSSHConfigEnvOverridesTheHomeConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ssh_config")
	body := "Host guest-a\n  HostName 10.66.0.11\n  User root\n  ProxyJump root@edge.example\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(SSHConfigEnv, path)

	cfg := loadSSHConfig()
	if cfg == nil {
		t.Fatal("the config named by " + SSHConfigEnv + " was not loaded")
	}
	got := resolveHost(cfg, "guest-a")
	if got.host != "10.66.0.11" || got.user != "root" {
		t.Errorf("resolved %+v, want 10.66.0.11 as root", got)
	}
	if len(got.jumps) != 1 || got.jumps[0] != "root@edge.example" {
		t.Errorf("jumps = %v, want [root@edge.example]", got.jumps)
	}
}
