package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/trakhimenok/hoston/internal/keychain"
)

var authCmd = &cobra.Command{
	Use:       "auth [provider]",
	Short:     "Authenticate with a provider",
	ValidArgs: []string{"namecheap", "cloudflare"},
	Args:      cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var provider string
		if len(args) == 1 {
			provider = args[0]
		} else {
			err := huh.NewSelect[string]().
				Title("Choose provider to authenticate").
				Options(
					huh.NewOption("NameCheap", "namecheap"),
					huh.NewOption("CloudFlare", "cloudflare"),
				).
				Value(&provider).
				Run()
			if err != nil {
				return err
			}
		}

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
