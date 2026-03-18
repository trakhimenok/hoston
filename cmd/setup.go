package cmd

import (
	"github.com/spf13/cobra"
	"github.com/trakhimenok/hostme/internal/wizard"
)

var setupCmd = &cobra.Command{
	Use:   "setup [domain]",
	Short: "Set up a new domain with DNS and hosting",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return wizard.RunSetup(cmd.Context(), args[0], verbose)
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
