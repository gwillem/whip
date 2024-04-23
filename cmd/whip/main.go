package main

import (
	"os"

	log "github.com/gwillem/go-simplelog"

	"github.com/gwillem/whip/internal/vault"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "whip <playbook>",
		Short: "A fast and simple configuration manager",
		Long: `Chief Whip is a fast and simple configuration manager.
It aims to be stand-in replacement for Ansible for 90% of use cases.`,
		CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
		Args:              cobra.ExactArgs(1),
		Run:               runWhip,
	}
	vaultEditCmd = &cobra.Command{
		Use:   "edit",
		Short: "Encrypt and decrypt secrets",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			if err := vault.LaunchEditor(args[0]); err != nil {
				log.Fatal(err)
			}
		},
	}
	vaultConvertCmd = &cobra.Command{
		Use:   "convert",
		Short: "Convert secrets from Ansible Vault to Whip (Age)",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			if err := vault.ConvertAnsibleToWhip(args[0]); err != nil {
				log.Fatal(err)
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(vaultEditCmd, vaultConvertCmd)
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.PersistentFlags().CountP("verbose", "v", "verbose output")
}

func main() {
	if e := rootCmd.Execute(); e != nil {
		os.Exit(1)
	}
}
