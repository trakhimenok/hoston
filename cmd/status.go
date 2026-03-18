package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [domain]",
	Short: "Check domain status across providers",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Status check not yet implemented for %s\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
