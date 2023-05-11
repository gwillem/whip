package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "chief",
		Short: "A fast and simple configuration manager",
		Long: `Chief Whip is a fast and simple configuration manager.
It aims to be stand-in replacement for Ansible for 90% of use cases.`,
		CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
	}
	whipCmd = &cobra.Command{
		Use:   "whip <playbook.yml>",
		Short: "Apply playbook to targets listed in playbook",
		Args:  cobra.MinimumNArgs(1),
		Run:   runWhip,
	}
	vaultCmd = &cobra.Command{
		Use:   "vault",
		Short: "Encrypt and decrypt secrets",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("not implemented yet")
		},
	}
)

func init() {
	rootCmd.AddCommand(whipCmd, vaultCmd)
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
}
