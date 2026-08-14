package ssh

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kevinburke/ssh_config"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// the shape of the honeypot fleet: containers on a private bridge, reachable
// only through the edge host, plus a two hop chain to prove chains work
const sampleConfig = `
Host edge
	HostName 159.69.26.146
	User root
	Port 2222
	IdentityFile ~/.ssh/id_fleet

Host store-*
	ProxyJump edge
	User root

Host far
	ProxyJump first,second

Host noproxy
	ProxyJump none

Host locked
	IdentityFile ~/.ssh/id_fleet
	IdentitiesOnly yes
`

// emptyHome points HOME at a scratch dir so tests never see the developer's own
// keys, config or known_hosts.
func emptyHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ssh"), 0o700))
	return home
}

func Test_splitTarget(t *testing.T) {
	for _, tc := range []struct {
		target, user, host, port string
	}{
		{"host", "", "host", ""},
		{"root@host", "root", "host", ""},
		{"root@host:2211", "root", "host", "2211"},
		{"127.0.0.1:2211", "", "127.0.0.1", "2211"},
	} {
		user, host, port := splitTarget(tc.target)
		require.Equal(t, []string{tc.user, tc.host, tc.port}, []string{user, host, port}, tc.target)
	}
}

func Test_resolveHost(t *testing.T) {
	emptyHome(t)
	cfg, err := ssh_config.DecodeBytes([]byte(sampleConfig))
	require.NoError(t, err)

	for _, tc := range []struct {
		name, target     string
		user, host, port string
		jumps            []string
	}{
		{
			name:   "alias resolves HostName, User and Port",
			target: "edge", user: "root", host: "159.69.26.146", port: "2222",
		},
		{
			name:   "target overrides config, as on the ssh command line",
			target: "bob@edge:2022", user: "bob", host: "159.69.26.146", port: "2022",
		},
		{
			name:   "wildcard stanza gives User and ProxyJump, host stays as written",
			target: "store-1", user: "root", host: "store-1", port: "22",
			jumps: []string{"edge"},
		},
		{
			name:   "address not in the config is dialled directly",
			target: "root@10.66.0.11", user: "root", host: "10.66.0.11", port: "22",
		},
		{
			name:   "comma separated ProxyJump is a chain",
			target: "far", user: os.Getenv("USER"), host: "far", port: "22",
			jumps: []string{"first", "second"},
		},
		{
			name:   "ProxyJump none means direct",
			target: "noproxy", user: os.Getenv("USER"), host: "noproxy", port: "22",
		},
	} {
		hc := resolveHost(cfg, tc.target)
		require.Equal(t, tc.user, hc.user, tc.name)
		require.Equal(t, tc.host, hc.host, tc.name)
		require.Equal(t, tc.port, hc.port, tc.name)
		require.Equal(t, tc.jumps, hc.jumps, tc.name)
	}
}

// A jump host is resolved with the same code path as any other target, so its
// own HostName/User/Port/IdentityFile stanza applies. That is the fleet's shape:
// hosts: [root@10.66.0.11] with a ProxyJump to an alias.
func Test_resolveHost_jumpResolvesThroughItsOwnStanza(t *testing.T) {
	home := emptyHome(t)
	key := filepath.Join(home, ".ssh", "id_fleet")
	require.NoError(t, os.WriteFile(key, []byte("x"), 0o600))

	cfg, err := ssh_config.DecodeBytes([]byte(sampleConfig))
	require.NoError(t, err)

	target := resolveHost(cfg, "store-2")
	require.Equal(t, []string{"edge"}, target.jumps)

	jump := resolveHost(cfg, target.jumps[0])
	require.Equal(t, "159.69.26.146", jump.host)
	require.Equal(t, "root", jump.user)
	require.Equal(t, "2222", jump.port)
	require.Equal(t, []string{key}, jump.identities)
}

// IdentitiesOnly is what our own ssh_config ships, and offering the defaults on
// top of the named key is how a host with MaxAuthTries 6 starts refusing us.
func Test_resolveHost_identitiesOnly(t *testing.T) {
	home := emptyHome(t)
	fleet := filepath.Join(home, ".ssh", "id_fleet")
	for _, n := range []string{"id_fleet", "id_ed25519"} {
		require.NoError(t, os.WriteFile(filepath.Join(home, ".ssh", n), []byte("x"), 0o600))
	}
	cfg, err := ssh_config.DecodeBytes([]byte(sampleConfig))
	require.NoError(t, err)

	require.Equal(t, []string{fleet}, resolveHost(cfg, "locked").identities)
	// without it, the defaults are still tried
	require.Equal(t, []string{filepath.Join(home, ".ssh", "id_ed25519")},
		resolveHost(cfg, "store-1").identities)
}

func Test_identityFiles(t *testing.T) {
	home := emptyHome(t)
	sshdir := filepath.Join(home, ".ssh")
	// no id_ecdsa: absent keys must be skipped, not offered
	for _, n := range []string{"id_rsa", "id_ed25519", "id_fleet"} {
		require.NoError(t, os.WriteFile(filepath.Join(sshdir, n), []byte("x"), 0o600))
	}

	t.Run("configured first, then ed25519 before rsa", func(t *testing.T) {
		require.Equal(t, []string{
			filepath.Join(sshdir, "id_fleet"),
			filepath.Join(sshdir, "id_ed25519"),
			filepath.Join(sshdir, "id_rsa"),
		}, identityFiles([]string{"~/.ssh/id_fleet"}, false))
	})

	t.Run("defaults only", func(t *testing.T) {
		require.Equal(t, []string{
			filepath.Join(sshdir, "id_ed25519"),
			filepath.Join(sshdir, "id_rsa"),
		}, identityFiles(nil, false))
	})

	t.Run("absent and duplicate entries are dropped", func(t *testing.T) {
		got := identityFiles([]string{"~/.ssh/id_rsa", "$HOME/.ssh/id_rsa", "~/.ssh/nosuchkey"}, false)
		require.Equal(t, []string{
			filepath.Join(sshdir, "id_rsa"),
			filepath.Join(sshdir, "id_ed25519"),
		}, got)
	})

	t.Run("IdentitiesOnly stops the defaults being appended", func(t *testing.T) {
		require.Equal(t, []string{filepath.Join(sshdir, "id_fleet")},
			identityFiles([]string{"~/.ssh/id_fleet"}, true))
	})

	t.Run("IdentitiesOnly with a missing key still falls back", func(t *testing.T) {
		require.Equal(t, []string{
			filepath.Join(sshdir, "id_ed25519"),
			filepath.Join(sshdir, "id_rsa"),
		}, identityFiles([]string{"~/.ssh/nosuchkey"}, true))
	})
}

func testHostKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	return signer.PublicKey()
}

func Test_knownHostsCallback(t *testing.T) {
	const host = "10.66.0.11:22"
	remote := &testAddr{"10.66.0.11:22"}
	key := testHostKey(t)

	t.Run("unknown host is accepted and recorded", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "sub", "known_hosts")
		cb := knownHostsCallback(path)
		require.NoError(t, cb(host, remote, key))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Contains(t, string(data), "10.66.0.11")
		require.Contains(t, string(data), key.Type())

		// second contact verifies against the recorded key without appending
		require.NoError(t, cb(host, remote, key))
		again, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, data, again)
	})

	t.Run("changed key is refused, naming host and fingerprint", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "known_hosts")
		cb := knownHostsCallback(path)
		require.NoError(t, cb(host, remote, key))

		other := testHostKey(t)
		err := cb(host, remote, other)
		require.Error(t, err)
		require.Contains(t, err.Error(), "10.66.0.11")
		require.Contains(t, err.Error(), ssh.FingerprintSHA256(other))
		require.Contains(t, err.Error(), ssh.FingerprintSHA256(key))
		require.Contains(t, err.Error(), path)
	})

	t.Run("unwritable database does not fail the connection", func(t *testing.T) {
		// a file where a directory should be: recording cannot work here
		blocker := filepath.Join(t.TempDir(), "notadir")
		require.NoError(t, os.WriteFile(blocker, nil, 0o600))
		require.NoError(t, knownHostsCallback(filepath.Join(blocker, "known_hosts"))(host, remote, key))
	})

	t.Run("insecure skips verification and records nothing", func(t *testing.T) {
		home := emptyHome(t)
		require.NoError(t, hostKeyCallback(Options{Insecure: true})(host, remote, key))
		_, err := os.Stat(filepath.Join(home, ".ssh", "known_hosts"))
		require.True(t, os.IsNotExist(err))
	})
}

type testAddr struct{ addr string }

func (a *testAddr) Network() string { return tcp }
func (a *testAddr) String() string  { return a.addr }

func Test_remoteError(t *testing.T) {
	exit := errors.New("Process exited with status 127")

	t.Run("success stays nil", func(t *testing.T) {
		require.NoError(t, remoteError("true", nil, nil))
	})

	t.Run("names the command and quotes the stderr", func(t *testing.T) {
		cmd := "xz -d > /tmp/deputy.tmp && chmod 755 /tmp/deputy.tmp"
		err := remoteError(cmd, []byte("bash: line 1: xz: command not found\n"), exit)
		require.ErrorIs(t, err, exit)
		require.Contains(t, err.Error(), cmd)
		require.Contains(t, err.Error(), "xz: command not found")
	})

	t.Run("multi line commands fold onto one line", func(t *testing.T) {
		err := remoteError("uname -sm;\n\tsha256sum ~/.cache/whip/deputy;\n", nil, exit)
		require.Contains(t, err.Error(), "uname -sm; sha256sum ~/.cache/whip/deputy;")
		require.NotContains(t, err.Error(), "\n")
	})

	t.Run("long output is trimmed to its tail", func(t *testing.T) {
		err := remoteError("cmd", []byte(strings.Repeat("x", maxStderr*2)+"the end"), exit)
		require.Contains(t, err.Error(), "the end")
		require.Less(t, len(err.Error()), maxStderr+200)
	})
}

func Test_authMethods_noKeysNoAgent(t *testing.T) {
	emptyHome(t)
	t.Setenv(agentSock, "")
	_, err := authMethods(hostConfig{})
	require.Error(t, err)
	// the message must say where we looked, or the user cannot fix it
	require.Contains(t, err.Error(), agentSock)
	require.Contains(t, err.Error(), "id_ed25519")
	require.Contains(t, err.Error(), sshDir())
}

func Test_loadSSHConfig_missingFileIsNotFatal(t *testing.T) {
	emptyHome(t)
	require.Nil(t, loadSSHConfig())

	// and a config that is present is honoured
	require.NoError(t, os.WriteFile(filepath.Join(sshDir(), "config"), []byte(sampleConfig), 0o600))
	cfg := loadSSHConfig()
	require.NotNil(t, cfg)
	require.Equal(t, "159.69.26.146", resolveHost(cfg, "edge").host)
}

// testServer is the smallest ssh server that can stand in for a host: it accepts
// any public key, answers an exec request with a fixed line, and forwards
// direct-tcpip channels so it can also play jump host. It runs on loopback, so
// the ProxyJump path can be proven without a network.
type testServer struct {
	ln     net.Listener
	addr   string
	banner string
	stderr string       // when set, written to the session's stderr instead
	status uint32       // exit status to report
	jumped atomic.Int32 // direct-tcpip channels served
}

func newTestServer(t *testing.T, banner string) *testServer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)

	conf := &ssh.ServerConfig{
		PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
	}
	conf.AddHostKey(signer)

	ln, err := net.Listen(tcp, "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck

	s := &testServer{ln: ln, addr: ln.Addr().String(), banner: banner}
	go func() {
		for {
			nc, err := ln.Accept()
			if err != nil {
				return
			}
			go s.handle(nc, conf)
		}
	}()
	return s
}

func (s *testServer) port() string {
	_, port, _ := net.SplitHostPort(s.addr)
	return port
}

func (s *testServer) handle(nc net.Conn, conf *ssh.ServerConfig) {
	sc, chans, reqs, err := ssh.NewServerConn(nc, conf)
	if err != nil {
		nc.Close() //nolint:errcheck
		return
	}
	defer sc.Close() //nolint:errcheck
	go ssh.DiscardRequests(reqs)
	for nch := range chans {
		switch nch.ChannelType() {
		case "session":
			ch, creqs, err := nch.Accept()
			if err != nil {
				continue
			}
			go s.session(ch, creqs)
		case "direct-tcpip":
			s.jumped.Add(1)
			s.forward(nch)
		default:
			nch.Reject(ssh.UnknownChannelType, nch.ChannelType()) //nolint:errcheck
		}
	}
}

func (s *testServer) session(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close() //nolint:errcheck
	for req := range reqs {
		if req.WantReply {
			req.Reply(req.Type == "exec", nil) //nolint:errcheck
		}
		if req.Type != "exec" {
			continue
		}
		if s.stderr != "" {
			fmt.Fprintln(ch.Stderr(), s.stderr)
		} else {
			fmt.Fprintln(ch, s.banner)
		}
		ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{s.status})) //nolint:errcheck
		return
	}
}

func (s *testServer) forward(nch ssh.NewChannel) {
	var req struct {
		Host     string
		Port     uint32
		OrigHost string
		OrigPort uint32
	}
	if err := ssh.Unmarshal(nch.ExtraData(), &req); err != nil {
		nch.Reject(ssh.Prohibited, err.Error()) //nolint:errcheck
		return
	}
	up, err := net.Dial(tcp, net.JoinHostPort(req.Host, fmt.Sprint(req.Port)))
	if err != nil {
		nch.Reject(ssh.ConnectionFailed, err.Error()) //nolint:errcheck
		return
	}
	ch, reqs, err := nch.Accept()
	if err != nil {
		up.Close() //nolint:errcheck
		return
	}
	go ssh.DiscardRequests(reqs)
	go func() {
		io.Copy(ch, up) //nolint:errcheck
		ch.Close()      //nolint:errcheck
	}()
	go func() {
		io.Copy(up, ch) //nolint:errcheck
		up.Close()      //nolint:errcheck
	}()
}

// writeTestKey drops a usable private key in ~/.ssh so authMethods has something
// to offer.
func writeTestKey(t *testing.T, name string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	blk, err := ssh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(sshDir(), name), pem.EncodeToMemory(blk), 0o600))
}

// The fleet's shape: the target is reachable only from the jump host, and the
// jump host is described by its own stanza. Before ProxyJump support this needed
// an out of band "ssh -N -L 2211:10.66.0.11:22" and a hard coded port.
func Test_ConnectWith_proxyJump(t *testing.T) {
	emptyHome(t)
	t.Setenv(agentSock, "")
	writeTestKey(t, "id_ed25519")

	jump := newTestServer(t, "jump")
	target := newTestServer(t, "target")

	conf := fmt.Sprintf(`
Host bastion
	HostName 127.0.0.1
	Port %s
	User root

Host store-1
	HostName 127.0.0.1
	Port %s
	User deputy
	ProxyJump bastion
`, jump.port(), target.port())
	require.NoError(t, os.WriteFile(filepath.Join(sshDir(), "config"), []byte(conf), 0o600))

	c, err := ConnectWith("store-1", Options{})
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck

	out, err := c.Run("echo hoi")
	require.NoError(t, err)
	require.Equal(t, "target\n", out, "the session must land on the target, not on the jump host")
	require.Equal(t, int32(1), jump.jumped.Load(), "the jump host must have forwarded one connection")

	// accept-new recorded both hosts on first contact
	kh, err := os.ReadFile(filepath.Join(sshDir(), "known_hosts"))
	require.NoError(t, err)
	require.Len(t, strings.Split(strings.TrimSpace(string(kh)), "\n"), 2)

	// and closing the client closes the jump chain with it
	require.Len(t, c.jumps, 1)
	require.NoError(t, c.Close())
	_, err = c.jumps[0].NewSession()
	require.Error(t, err)
}

// A direct target must keep working exactly as before: no config stanza, no jump.
func Test_ConnectWith_direct(t *testing.T) {
	emptyHome(t)
	t.Setenv(agentSock, "")
	writeTestKey(t, "id_ed25519")

	srv := newTestServer(t, "direct")
	c, err := ConnectWith("root@"+srv.addr, Options{})
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck

	out, err := c.Run("echo hoi")
	require.NoError(t, err)
	require.Equal(t, "direct\n", out)
	require.Empty(t, c.jumps)
	require.Equal(t, int32(0), srv.jumped.Load())
}

// Gap 14: a target without xz used to fail the whole run with nothing but
// "Process exited with status 127".
func Test_UploadBytesXZ_reportsRemoteStderr(t *testing.T) {
	emptyHome(t)
	t.Setenv(agentSock, "")
	writeTestKey(t, "id_ed25519")

	srv := newTestServer(t, "")
	srv.stderr = "bash: line 1: xz: command not found"
	srv.status = 127

	c, err := ConnectWith("root@"+srv.addr, Options{})
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck

	err = c.UploadBytesXZ([]byte("payload"), "/tmp/deputy", 0o755)
	require.Error(t, err)
	require.Contains(t, err.Error(), "xz -d > /tmp/deputy.tmp")
	require.Contains(t, err.Error(), "xz: command not found")
	require.Contains(t, err.Error(), "127")

	var exit *ssh.ExitError
	require.True(t, errors.As(err, &exit), "the ExitError must stay reachable")
}

// A named IdentityFile must be offered even when an agent is running.
//
// The Go client spends "publickey" on the first method that uses it, so the
// agent's callback followed by a separate ssh.PublicKeys for the configured
// key meant the configured key was never sent. On a laptop with three
// unrelated keys in its agent, a host with IdentityFile in ssh_config was
// unreachable and sshd logged three failures for keys nobody had named.
func Test_authMethodsOffersConfiguredKeysAndAgentInOneMethod(t *testing.T) {
	emptyHome(t)
	t.Setenv(agentSock, "")
	writeTestKey(t, "id_named")
	keyPath := filepath.Join(sshDir(), "id_named")

	methods, err := authMethods(hostConfig{identities: []string{keyPath}})
	require.NoError(t, err)
	require.Len(t, methods, 1,
		"every key must travel in one publickey method, or the ones after the first are never offered")
}

// IdentitiesOnly means what it means in ssh(1): the agent is not consulted.
func Test_authMethodsHonoursIdentitiesOnly(t *testing.T) {
	emptyHome(t)
	writeTestKey(t, "id_named")
	keyPath := filepath.Join(sshDir(), "id_named")

	t.Setenv(agentSock, "/nonexistent/agent.sock")
	hc := hostConfig{identities: []string{keyPath}, identitiesOnly: true}
	methods, err := authMethods(hc)
	require.NoError(t, err)
	require.Len(t, methods, 1)
}

// A host with no key and no agent still fails, and says what it looked for.
func Test_authMethodsWithNothingToOfferSaysSo(t *testing.T) {
	t.Setenv(agentSock, "")
	_, err := authMethods(hostConfig{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no usable key")
}

// The end to end version of the bug: an agent holding a key the server will
// not accept, and an IdentityFile it will. The named key has to win.
//
// This is how it presented in the field: a laptop with three unrelated keys in
// its agent, an ssh_config naming a per fleet key, and a target that refused
// every connection. sshd logged three failures for the agent's keys and no
// attempt at the named one.
func Test_ConnectWith_namedIdentityBeatsACrowdedAgent(t *testing.T) {
	emptyHome(t)

	// the key the server accepts, named in ssh_config
	_, wanted, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	wantedSigner, err := ssh.NewSignerFromKey(wanted)
	require.NoError(t, err)
	blk, err := ssh.MarshalPrivateKey(wanted, "")
	require.NoError(t, err)
	keyPath := filepath.Join(sshDir(), "id_fleet")
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(blk), 0o600))

	// and a different one, in the agent, which the server refuses
	_, spare, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	startTestAgent(t, spare)

	srv := newTestServerAccepting(t, "fleet", wantedSigner.PublicKey())

	conf := fmt.Sprintf("Host store-1\n\tHostName 127.0.0.1\n\tPort %s\n\tUser root\n\tIdentityFile %s\n",
		srv.port(), keyPath)
	require.NoError(t, os.WriteFile(filepath.Join(sshDir(), "config"), []byte(conf), 0o600))

	c, err := ConnectWith("store-1", Options{})
	require.NoError(t, err, "the named key must be offered even though the agent answered first")
	defer c.Close() //nolint:errcheck

	out, err := c.Run("echo hoi")
	require.NoError(t, err)
	require.Equal(t, "fleet\n", out)
}

// newTestServerAccepting is newTestServer with a guest list of one.
func newTestServerAccepting(t *testing.T, banner string, allowed ssh.PublicKey) *testServer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)

	want := allowed.Marshal()
	conf := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
			if !bytes.Equal(k.Marshal(), want) {
				return nil, fmt.Errorf("key refused")
			}
			return &ssh.Permissions{}, nil
		},
	}
	conf.AddHostKey(signer)

	ln, err := net.Listen(tcp, "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck

	s := &testServer{ln: ln, addr: ln.Addr().String(), banner: banner}
	go func() {
		for {
			nc, err := ln.Accept()
			if err != nil {
				return
			}
			go s.handle(nc, conf)
		}
	}()
	return s
}

// startTestAgent runs an in process ssh-agent holding one key and points
// $SSH_AUTH_SOCK at it.
func startTestAgent(t *testing.T, key ed25519.PrivateKey) {
	t.Helper()
	keyring := agent.NewKeyring()
	require.NoError(t, keyring.Add(agent.AddedKey{PrivateKey: &key}))

	// macOS caps unix socket paths near 104 bytes; t.TempDir() is longer
	dir, err := os.MkdirTemp("", "wa")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) }) //nolint:errcheck

	sock := filepath.Join(dir, "s")
	ln, err := net.Listen(unix, sock)
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go agent.ServeAgent(keyring, conn) //nolint:errcheck
		}
	}()
	t.Setenv(agentSock, sock)
}
