package main

import (
	"fmt"
	"os"

	log "github.com/gwillem/go-simplelog"
	flags "github.com/jessevdk/go-flags"

	"github.com/gwillem/whip/internal/update"
	"github.com/gwillem/whip/internal/vault"
)

type opts struct {
	Verbose []bool `short:"v" long:"verbose" description:"verbose output"`
	Version bool   `long:"version" description:"print version and exit"`

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
	runWhip(playbookArg, verbosity)
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
