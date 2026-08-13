package ssh

/*

Inspired by https://github.com/sfreiberg/simplessh/blob/master/simplessh.go

Goal: mimic basic ssh cli behaviour as much as possible: ~/.ssh/config is read
for HostName, User, Port, IdentityFile, IdentitiesOnly and ProxyJump, keys are
looked up the way ssh-keygen writes them, and host keys are verified against
~/.ssh/known_hosts with accept-new semantics.

*/

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/karrick/gobls"
	"github.com/kevinburke/ssh_config"

	// "github.com/klauspost/compress/zstd"
	log "github.com/gwillem/go-simplelog"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	tcp         = "tcp"
	unix        = "unix"
	agentSock   = "SSH_AUTH_SOCK"
	sshTimeout  = 3 * time.Second
	defaultPort = "22"
	// a ProxyJump loop in ~/.ssh/config would otherwise recurse forever
	maxJumps = 10
	// how much remote stderr to quote in an error; enough for a stack of shell
	// diagnostics, little enough to keep a progress line readable
	maxStderr = 4 << 10
)

// defaultKeyNames are tried in this order when ~/.ssh/config names no
// IdentityFile. Ed25519 first: it has been ssh-keygen's default output for
// years, so on a current machine id_rsa is often the one key that does not
// exist.
var defaultKeyNames = []string{"id_ed25519", "id_ecdsa", "id_rsa"}

// Insecure disables host key verification for every subsequent Connect, so a
// --insecure flag costs main() a single assignment.
var Insecure bool

type (
	// Options tunes a single connection. The zero value is the default policy:
	// verify the host key against ~/.ssh/known_hosts, recording hosts seen for
	// the first time.
	Options struct {
		Insecure bool // do not verify the host key at all
	}

	Client struct {
		cl *ssh.Client
		// jump hosts dialled on the way here, controller end first. Closed with
		// the client itself, so a ProxyJump chain does not leak connections.
		jumps []*ssh.Client
	}

	// hostConfig holds one target's effective settings after merging the target
	// as written by the user with ~/.ssh/config.
	hostConfig struct {
		user, host, port string
		identities       []string
		jumps            []string
	}
)

func (c *Client) Close() error {
	err := c.cl.Close()
	// unwind the chain from the target end back to the controller
	for i := len(c.jumps) - 1; i >= 0; i-- {
		if e := c.jumps[i].Close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}

func (c *Client) Run(cmd string) (string, error) {
	b, e := c.RunWriteRead(cmd, nil)
	return string(b), e
}

func (c *Client) RunWriteRead(cmd string, toWrite []byte) ([]byte, error) {
	sess, err := c.cl.NewSession()
	if err != nil {
		return nil, err
	}
	defer sess.Close() //nolint:errcheck
	if len(toWrite) > 0 {
		sess.Stdin = bytes.NewReader(toWrite)
	}
	out, err := sess.CombinedOutput(cmd)
	// the output is returned either way, but callers that only check the error
	// used to see a bare "Process exited with status 127"
	return out, remoteError(cmd, out, err)
}

// RunGobStreamer runs command over SSH, while parsing its stdout as gob stream.
// No method, because can't use generics on methods. Do we actually need
// generics to make the ssh package unaware of actual objects being passed?
func RunGobStreamer[T any](c *Client, cmd string, stdin io.Reader, callback func(T)) error {
	s, err := c.cl.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	s.Stdin = stdin
	var stderr bytes.Buffer
	s.Stderr = &stderr
	stdout, err := s.StdoutPipe()
	if err != nil {
		return err
	}
	if err := s.Start(cmd); err != nil {
		return remoteError(cmd, stderr.Bytes(), err)
	}

	dec := gob.NewDecoder(stdout)

	for {
		var obj T
		err := dec.Decode(&obj)
		if err == io.EOF {
			// End of the stream
			break
		} else if err != nil {
			log.Fatalf("error decoding GOB data: %v", err)
		}
		callback(obj)
	}
	// the buffer must be read after Wait, which flushes the stderr copy
	err = s.Wait()
	return remoteError(cmd, stderr.Bytes(), err)
}

func (c *Client) RunLineStreamer(cmd string, toWrite []byte, readCB func([]byte)) error {
	s, err := c.cl.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	if len(toWrite) > 0 {
		s.Stdin = bytes.NewReader(toWrite)
	}
	var stderr bytes.Buffer
	s.Stderr = &stderr
	stdout, err := s.StdoutPipe()
	if err != nil {
		return err
	}
	if err := s.Start(cmd); err != nil {
		return remoteError(cmd, stderr.Bytes(), err)
	}
	scanner := gobls.NewScanner(stdout)
	for scanner.Scan() {
		readCB(scanner.Bytes())
	}
	err = s.Wait()
	return remoteError(cmd, stderr.Bytes(), err)
}

func (c *Client) UploadBytes(data []byte, remote string, perm os.FileMode) error {
	client, err := sftp.NewClient(c.cl)
	if err != nil {
		return err
	}
	defer client.Close() //nolint:errcheck

	remoteFile, err := client.Create(remote)
	if err != nil {
		return err
	}

	_, err = remoteFile.Write(data)
	if err != nil {
		return err
	}

	return remoteFile.Chmod(perm)
}

func (c *Client) UploadBytesXZ(data []byte, remote string, perm os.FileMode) error {
	tempFile := fmt.Sprintf("%s.tmp", remote)
	// this needs xz on the target; a minimal image without xz-utils used to fail
	// with nothing but "status 127", so runCaptured quotes command and stderr
	cmd := fmt.Sprintf("xz -d > %s && chmod %o %s && mv -f %s %s", tempFile, perm, tempFile, tempFile, remote)
	return c.runCaptured(cmd, bytes.NewReader(data))
}

// runCaptured runs cmd, discarding its stdout but keeping stderr for the error.
func (c *Client) runCaptured(cmd string, stdin io.Reader) error {
	sess, err := c.cl.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close() //nolint:errcheck
	var stderr bytes.Buffer
	sess.Stderr = &stderr
	sess.Stdin = stdin
	err = sess.Run(cmd)
	return remoteError(cmd, stderr.Bytes(), err)
}

func (c *Client) UploadFile(local, remote string) error {
	client, err := sftp.NewClient(c.cl)
	if err != nil {
		return err
	}
	defer client.Close() //nolint:errcheck

	localFile, err := os.Open(local)
	if err != nil {
		return err
	}
	defer localFile.Close() //nolint:errcheck

	remoteFile, err := client.Create(remote)
	if err != nil {
		return err
	}

	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return err
	}

	localStat, err := localFile.Stat()
	if err != nil {
		return err
	}

	return remoteFile.Chmod(localStat.Mode())
}

// remoteError decorates a failed remote command with the command itself and
// whatever it said on stderr, because "Process exited with status 127" names
// neither the command nor the missing binary. The original error is wrapped, so
// errors.As(&ssh.ExitError{}) keeps working.
func remoteError(cmd string, output []byte, err error) error {
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(output))
	if len(msg) > maxStderr {
		// the tail is where the failure is
		msg = "..." + msg[len(msg)-maxStderr:]
	}
	if msg == "" {
		return fmt.Errorf("remote command failed: %s: %w", oneLine(cmd), err)
	}
	return fmt.Errorf("remote command failed: %s: %w: %s", oneLine(cmd), err, msg)
}

// oneLine folds a multi line command into something that fits an error message.
func oneLine(cmd string) string {
	return strings.Join(strings.Fields(cmd), " ")
}

func Connect(target string) (*Client, error) {
	return ConnectWith(target, Options{Insecure: Insecure})
}

// ConnectWith dials target, which may be a bare ~/.ssh/config alias, a
// user@host, or a user@host:port.
func ConnectWith(target string, o Options) (*Client, error) {
	cl, jumps, err := dial(loadSSHConfig(), target, o, nil, 0)
	if err != nil {
		return nil, err
	}
	return &Client{cl: cl, jumps: jumps}, nil
}

// dial connects to target, honouring its ProxyJump chain. via is an already
// established client to tunnel through, or nil to open a socket from here. It
// returns the target client plus every jump client opened on the way, so the
// caller can close them.
func dial(cfg *ssh_config.Config, target string, o Options, via *ssh.Client, depth int) (*ssh.Client, []*ssh.Client, error) {
	if depth > maxJumps {
		return nil, nil, fmt.Errorf("ssh: ProxyJump chain at %s exceeds %d hosts, is it a loop?", target, maxJumps)
	}

	hc := resolveHost(cfg, target)

	// Reach the jump hosts first and tunnel onwards from the last one. A jump
	// host may itself sit behind a jump, hence the recursion.
	var opened []*ssh.Client
	for _, j := range hc.jumps {
		jc, sub, err := dial(cfg, j, o, via, depth+1)
		if err != nil {
			closeAll(opened)
			return nil, nil, fmt.Errorf("ssh: via jump host %s: %w", j, err)
		}
		opened = append(append(opened, sub...), jc)
		via = jc
	}

	auth, err := authMethods(hc)
	if err != nil {
		closeAll(opened)
		return nil, nil, err
	}
	conf := &ssh.ClientConfig{
		User:            hc.user,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback(o),
		Timeout:         sshTimeout,
		// these ciphers were supposedly faster but I didn't measure any difference --WdG
		// Config:          ssh.Config{
		//			Ciphers: []string{"aes128-ctr", "aes192-ctr", "aes256-ctr", "aes128-gcm@openssh.com", "chacha20-poly1305@openssh.com"},
		// },
	}
	addr := net.JoinHostPort(hc.host, hc.port)

	if via == nil {
		cl, err := ssh.Dial(tcp, addr, conf)
		if err != nil {
			return nil, nil, fmt.Errorf("ssh: could not connect to %s as %s: %w", addr, hc.user, err)
		}
		return cl, opened, nil
	}

	// a plain TCP forward through the jump host, with an ssh handshake on top:
	// this is what ssh -J does
	sock, err := via.Dial(tcp, addr)
	if err != nil {
		closeAll(opened)
		return nil, nil, fmt.Errorf("ssh: could not reach %s through jump host: %w", addr, err)
	}
	conn, chans, reqs, err := ssh.NewClientConn(sock, addr, conf)
	if err != nil {
		sock.Close() //nolint:errcheck
		closeAll(opened)
		return nil, nil, fmt.Errorf("ssh: handshake with %s through jump host failed: %w", addr, err)
	}
	return ssh.NewClient(conn, chans, reqs), opened, nil
}

func closeAll(clients []*ssh.Client) {
	for i := len(clients) - 1; i >= 0; i-- {
		clients[i].Close() //nolint:errcheck
	}
}

// resolveHost merges the target as written with ~/.ssh/config. Anything spelled
// out in the target wins, as with ssh(1): the command line beats the file.
func resolveHost(cfg *ssh_config.Config, target string) hostConfig {
	user, alias, port := splitTarget(target)
	hc := hostConfig{user: user, host: alias, port: port}

	var (
		identities []string
		only       bool
	)
	if cfg != nil {
		if v := cfgGet(cfg, alias, "HostName"); v != "" {
			hc.host = v
		}
		if hc.user == "" {
			hc.user = cfgGet(cfg, alias, "User")
		}
		if hc.port == "" {
			hc.port = cfgGet(cfg, alias, "Port")
		}
		identities = cfgGetAll(cfg, alias, "IdentityFile")
		// with IdentitiesOnly the defaults are not appended: offering four keys
		// to a host with MaxAuthTries 6 is how "too many authentication
		// failures" happens
		only = strings.EqualFold(cfgGet(cfg, alias, "IdentitiesOnly"), "yes")
		for _, v := range cfgGetAll(cfg, alias, "ProxyJump") {
			// "ProxyJump a,b" is a chain: reach a, then b, then the target
			for _, j := range strings.Split(v, ",") {
				if j = strings.TrimSpace(j); j != "" && !strings.EqualFold(j, "none") {
					hc.jumps = append(hc.jumps, j)
				}
			}
		}
	}

	if hc.port == "" {
		hc.port = defaultPort
	}
	if hc.user == "" {
		hc.user = os.Getenv("USER")
	}
	hc.identities = identityFiles(identities, only)
	return hc
}

func cfgGet(cfg *ssh_config.Config, alias, key string) string {
	v, err := cfg.Get(alias, key)
	if err != nil {
		log.Debug("Could not read", key, "from ssh config:", err)
		return ""
	}
	return strings.TrimSpace(v)
}

func cfgGetAll(cfg *ssh_config.Config, alias, key string) []string {
	v, err := cfg.GetAll(alias, key)
	if err != nil {
		log.Debug("Could not read", key, "from ssh config:", err)
		return nil
	}
	return v
}

// loadSSHConfig reads ~/.ssh/config. A missing or broken file is not fatal:
// whip worked without one before and must keep doing so.
// SSHConfigEnv names an alternative ssh_config, so a repository can ship the
// stanzas its own playbooks need instead of asking every operator to edit
// ~/.ssh/config by hand. A fleet reached through a bastion is the ordinary
// case for that: the ProxyJump belongs with the playbooks, in review, not in
// each person's dotfiles.
const SSHConfigEnv = "WHIP_SSH_CONFIG"

func loadSSHConfig() *ssh_config.Config {
	path := os.Getenv(SSHConfigEnv)
	if path == "" {
		path = filepath.Join(sshDir(), "config")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Debug("No usable ssh config at", path, err)
		return nil
	}
	cfg, err := ssh_config.DecodeBytes(data)
	if err != nil {
		log.Debug("Could not parse", path, err)
		return nil
	}
	return cfg
}

// identityFiles returns the private keys to offer: those named by the config
// first, then the well known defaults, unless the config said IdentitiesOnly.
// Absent files are dropped, so a config listing keys for other hosts costs
// nothing here.
func identityFiles(configured []string, only bool) []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		p = expandPath(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, p)
		}
	}
	for _, p := range configured {
		add(p)
	}
	// if the named key is missing we still try the defaults: no keys at all
	// fails the connection outright, which is the worse outcome
	if only && len(out) > 0 {
		return out
	}
	for _, n := range defaultKeyNames {
		add(filepath.Join(sshDir(), n))
	}
	return out
}

// expandPath resolves the ~ and $VAR forms that ssh_config allows.
func expandPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		p = filepath.Join(home(), strings.TrimPrefix(p, "~"))
	}
	return os.ExpandEnv(p)
}

func home() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return os.Getenv("HOME")
}

func sshDir() string {
	return filepath.Join(home(), ".ssh")
}

func authMethods(hc hostConfig) ([]ssh.AuthMethod, error) {
	methods := []ssh.AuthMethod{}

	// the agent goes first: it is the only way to use an encrypted key
	if sock := os.Getenv(agentSock); sock != "" {
		if conn, err := net.Dial(unix, sock); err == nil {
			methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
		} else {
			log.Debug("Failed to connect to SSH agent:", sock, err)
		}
	}

	for _, f := range hc.identities {
		key, err := os.ReadFile(f)
		if err != nil {
			log.Debug("Could not read private key:", f, err)
			continue
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			// an encrypted key lands here; the agent is the answer for those
			log.Debug("Could not parse private key:", f, err)
			continue
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("no SSH auth methods available: no $%s and no usable key in %s (tried %s)",
			agentSock, sshDir(), strings.Join(defaultKeyNames, ", "))
	}
	return methods, nil
}

func hostKeyCallback(o Options) ssh.HostKeyCallback {
	if o.Insecure {
		return ssh.InsecureIgnoreHostKey()
	}
	return knownHostsCallback(filepath.Join(sshDir(), "known_hosts"))
}

// knownHostsCallback verifies the server key against path. A host met for the
// first time is accepted and recorded, as OpenSSH does with
// StrictHostKeyChecking=accept-new, because refusing it would break every first
// deploy. A key that changed is refused, naming host and fingerprint. Pass
// Options{Insecure: true} (or set ssh.Insecure) to skip all of this.
func knownHostsCallback(path string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if _, err := os.Stat(path); err == nil {
			cb, err := knownhosts.New(path)
			if err != nil {
				return fmt.Errorf("ssh: could not read %s: %w", path, err)
			}
			var ke *knownhosts.KeyError
			switch err := cb(hostname, remote, key); {
			case err == nil:
				return nil
			case errors.As(err, &ke) && len(ke.Want) == 0:
				// unknown host, recorded below
			case errors.As(err, &ke):
				return fmt.Errorf("ssh: host key mismatch for %s: server offers %s key %s, but %s line %d says %s. "+
					"If the host was rebuilt, remove that line; otherwise someone is between you and it",
					hostname, key.Type(), ssh.FingerprintSHA256(key), path, ke.Want[0].Line,
					ssh.FingerprintSHA256(ke.Want[0].Key))
			default:
				// revoked key, or an unreadable database
				return fmt.Errorf("ssh: host key rejected for %s: %w", hostname, err)
			}
		}

		// A read only ~/.ssh must not fail the deploy: we have already decided to
		// trust this key, and losing the record is the lesser problem.
		if err := appendKnownHost(path, hostname, key); err != nil {
			log.Debug("Could not record host key for", hostname, "in", path, err)
			return nil
		}
		log.Debug("Recorded new host key for", hostname, "in", path)
		return nil
	}
}

func appendKnownHost(path, hostname string, key ssh.PublicKey) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck
	_, err = fmt.Fprintln(f, knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key))
	return err
}

func splitTarget(target string) (user, host, port string) {
	tok := strings.Split(target, "@")
	if len(tok) != 1 {
		user = tok[0]
		target = tok[1]
	}

	host, port, err := net.SplitHostPort(target)
	if err == nil {
		return user, host, port
	}
	return user, target, ""
}
