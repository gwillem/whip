package main

import (
	"fmt"
	"os"

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
	vaultCmd = &cobra.Command{
		Use:   "vault",
		Short: "Encrypt and decrypt secrets",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("not implemented yet")
		},
	}
)

func init() {
	rootCmd.AddCommand(vaultCmd)
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
}

func main() {
	if e := rootCmd.Execute(); e != nil {
		os.Exit(1)
	}
}
