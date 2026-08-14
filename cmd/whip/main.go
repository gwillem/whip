package main

import (
	"fmt"
	"os"

	log "github.com/gwillem/go-simplelog"
	flags "github.com/jessevdk/go-flags"

	"github.com/gwillem/whip/internal/ssh"
	"github.com/gwillem/whip/internal/update"
	"github.com/gwillem/whip/internal/vault"
)

type opts struct {
	Verbose []bool `short:"v" long:"verbose" description:"verbose output"`
	Version bool   `long:"version" description:"print version and exit"`

	// ExtraVars is repeatable: -e docroot=/srv/web -e db_name=magento. It
	// overrides vars_files and the play's own vars, so one playbook can serve
	// several targets from a script.
	ExtraVars []string `short:"e" long:"extra-vars" description:"set a variable, overriding the playbook (key=value, repeatable)" value-name:"key=value"`

	// Insecure turns off host key verification. The default is
	// accept-new: an unknown host is recorded on first contact, and a key
	// that later changes is refused. That is right almost always, and wrong
	// for a target legitimately rebuilt under the same address, which is
	// exactly what a honeypot fleet does on every rotation.
	Insecure bool `long:"insecure" description:"do not verify SSH host keys"`

	Edit    editCmd    `command:"edit" description:"encrypt and decrypt secrets"`
	Encrypt encryptCmd `command:"encrypt" description:"encrypt stdin to stdout"`
	Decrypt decryptCmd `command:"decrypt" description:"decrypt stdin to stdout"`
	Convert convertCmd `command:"convert" description:"convert secrets from Ansible Vault to Whip (Age)"`
	Update  updateCmd  `command:"update" description:"update Whip to the latest version"`
}

type editCmd struct {
	Args struct {
		File string `positional-arg-name:"file" required:"true"`
	} `positional-args:"true"`
}

type encryptCmd struct{}

type decryptCmd struct{}

type convertCmd struct {
	Args struct {
		File string `positional-arg-name:"file" required:"true"`
	} `positional-args:"true"`
}

type updateCmd struct{}

func (c *editCmd) Execute(args []string) error {
	return vault.LaunchEditor(c.Args.File)
}

func (c *encryptCmd) Execute(args []string) error {
	return vault.EncryptStream(os.Stdin, os.Stdout)
}

func (c *decryptCmd) Execute(args []string) error {
	return vault.DecryptStream(os.Stdin, os.Stdout)
}

func (c *convertCmd) Execute(args []string) error {
	return vault.ConvertAnsibleToWhip(c.Args.File)
}

func (c *updateCmd) Execute(args []string) error {
	return update.Run(buildVersion)
}

func main() {
	var o opts
	parser := flags.NewParser(&o, flags.Default)
	parser.Name = "whip"
	parser.SubcommandsOptional = true

	args, err := parser.Parse()
	if err != nil {
		if flags.WroteHelp(err) {
			os.Exit(0)
		}
		os.Exit(1)
	}

	if o.Version {
		fmt.Println("whip", buildVersion)
		return
	}

	// If a subcommand was executed, we're done
	if parser.Active != nil {
		return
	}

	// Default action: run whip with optional playbook argument
	var playbookArg string
	if len(args) > 0 {
		playbookArg = args[0]
	}

	verbosity := setVerbosityLevel(len(o.Verbose))

	ssh.Insecure = o.Insecure

	extraVars, err := parseExtraVars(o.ExtraVars)
	if err != nil {
		log.Fatal(err)
	}
	runWhip(playbookArg, verbosity, extraVars)
}

func setVerbosityLevel(verbosity int) int {
	log.SetLevel(log.LevelError)
	if verbosity > 0 {
		log.SetLevel(log.LevelTask)
	}
	if verbosity > 1 {
		log.SetLevel(log.LevelDebug)
	}
	return verbosity
}
