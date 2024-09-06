package main

import (
	"fmt"
	"os"

	log "github.com/gwillem/go-simplelog"

	"github.com/gwillem/whip/internal/update"
	"github.com/gwillem/whip/internal/vault"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:               "whip [playbook]",
		Short:             "A fast and simple configuration manager",
		Long:              `A fast and simple configuration manager. Like a bloat-free Ansible.`,
		CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
		Args:              cobra.MaximumNArgs(1),
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
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Whip",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println("whip", buildVersion)
		},
	}
	updateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update Whip to the latest version",
		Run: func(_ *cobra.Command, _ []string) {
			if err := update.Run(buildVersion); err != nil {
				log.Fatal(err)
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(vaultEditCmd, vaultConvertCmd, versionCmd, updateCmd)
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.PersistentFlags().CountP("verbose", "v", "verbose output")
}

func main() {
	if e := rootCmd.Execute(); e != nil {
		os.Exit(1)
	}
}
