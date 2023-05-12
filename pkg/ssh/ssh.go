package ssh

/*

Inspired by https://github.com/sfreiberg/simplessh/blob/master/simplessh.go

Goal: mimic basic ssh cli behaviour as much as possible. Todo: parse .ssh/config

*/

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	TCP        = "tcp"
	UNIX       = "unix"
	AGENT_SOCK = "SSH_AUTH_SOCK"
)

var defaultKeyFile = os.ExpandEnv("$HOME/.ssh/id_rsa")

type (
	Client struct {
		cl *ssh.Client
	}
)

func (c *Client) Close() error {
	return c.cl.Close()
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
	defer sess.Close()
	if len(toWrite) > 0 {
		sess.Stdin = bytes.NewReader(toWrite)
	}
	return sess.CombinedOutput(cmd)
}

func (c *Client) RunLineStreamer(cmd string, toWrite []byte, readCB func([]byte)) error {
	s, err := c.cl.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	if len(toWrite) > 0 {
		s.Stdin = bytes.NewReader(toWrite)
	}
	stdout, err := s.StdoutPipe()
	if err != nil {
		return err
	}
	if err := s.Start(cmd); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		readCB(scanner.Bytes())
	}
	if err := s.Wait(); err != nil {
		return err
	}
	return nil
}

func (c *Client) UploadBytes(data []byte, remote string, perm os.FileMode) error {
	client, err := sftp.NewClient(c.cl)
	if err != nil {
		return err
	}
	defer client.Close()

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

func (c *Client) UploadFile(local, remote string) error {
	client, err := sftp.NewClient(c.cl)
	if err != nil {
		return err
	}
	defer client.Close()

	localFile, err := os.Open(local)
	if err != nil {
		return err
	}
	defer localFile.Close()

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

func Connect(target string) (*Client, error) {
	user, host, port := splitTarget(target)

	if port == "" {
		port = "22"
	}

	if user == "" {
		user = os.Getenv("USER")
	}

	// fmt.Println(user, host, port)

	authMethods := []ssh.AuthMethod{}

	// Try default key file?
	if key, err := os.ReadFile(defaultKeyFile); err == nil {
		signer, err := ssh.ParsePrivateKey(key)
		if err == nil {
			// fmt.Println("Adding default key auth")
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
	}

	// Try agent?
	if os.Getenv(AGENT_SOCK) != "" {
		if agentConn, err := net.Dial("unix", os.Getenv(AGENT_SOCK)); err == nil {
			// fmt.Println("Adding agent auth")
			authMethod := ssh.PublicKeysCallback(agent.NewClient(agentConn).Signers)
			authMethods = append(authMethods, authMethod)
		}
	}

	if len(authMethods) == 0 {
		log.Fatal("No auth methods available")
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	addr := fmt.Sprintf("%s:%s", host, port)
	cl, err := ssh.Dial(TCP, addr, config)
	return &Client{cl: cl}, err
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
