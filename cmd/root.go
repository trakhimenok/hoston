package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var verbose bool

var rootCmd = &cobra.Command{
	Use:     "hoston",
	Short:   "Domain & hosting setup CLI",
	Version: "0.1.0",
	Long: `hoston manages domain registration and hosting setup across multiple providers:
  - NameCheap  (domain registration & DNS)
  - CloudFlare (DNS & CDN)
  - Firebase   (hosting)
  - GitHub Pages (static hosting)`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}
