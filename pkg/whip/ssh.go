package whip

/*
see https://github.com/sfreiberg/simplessh/blob/master/simplessh.go#L57
*/

import (
	"fmt"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	TCP = "tcp"
)

func DialSSH(target string) (*ssh.Client, error) {

	user, host, port := SplitTarget(target)

	if port == "" {
		port = "22"
	}

	if user == "" {
		user = os.Getenv("USER")
	}

	keyfile := os.ExpandEnv("$HOME/.ssh/id_rsa")

	key, err := os.ReadFile(keyfile)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		// ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error { return nil }),
	}
	addr := fmt.Sprintf("%s:%s", host, port)
	return ssh.Dial(TCP, addr, config)
}

func SplitTarget(target string) (user, host, port string) {
	tok := strings.Split(target, "@")
	if len(tok) != 1 {
		user = tok[0]
	}

	host, port, err := net.SplitHostPort(target)
	if err == nil {
		return user, host, port
	}
	return user, target, ""

}
