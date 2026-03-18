package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/trakhimenok/hoston/internal/keychain"
)

var authCmd = &cobra.Command{
	Use:       "auth [provider]",
	Short:     "Authenticate with a provider",
	ValidArgs: []string{"namecheap", "cloudflare"},
	Args:      cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := args[0]
		switch provider {
		case "namecheap":
			return keychain.AuthNamecheap()
		case "cloudflare":
			return keychain.AuthCloudflare()
		default:
			return fmt.Errorf("unknown provider %q: must be one of namecheap, cloudflare", provider)
		}
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
}
