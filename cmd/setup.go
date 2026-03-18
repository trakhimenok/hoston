package cmd

import (
	"github.com/spf13/cobra"
	cfclient "github.com/trakhimenok/hoston/internal/cloudflare"
	"github.com/trakhimenok/hoston/internal/firebase"
	ghpages "github.com/trakhimenok/hoston/internal/github"
	"github.com/trakhimenok/hoston/internal/keychain"
	"github.com/trakhimenok/hoston/internal/namecheap"
	"github.com/trakhimenok/hoston/internal/provider"
	"github.com/trakhimenok/hoston/internal/wizard"
)

var setupCmd = &cobra.Command{
	Use:   "setup [domain]",
	Short: "Set up a new domain with DNS and hosting",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		domain := args[0]
		ctx := cmd.Context()

		// Resolve DNS provider credentials and build client.
		cfToken, err := keychain.GetCloudflareToken()
		if err != nil {
			return err
		}
		dns, err := cfclient.NewClient(cfToken)
		if err != nil {
			return err
		}

		// Resolve registrar credentials and build client.
		ncUser, ncKey, err := keychain.GetNamecheapCredentials()
		if err != nil {
			return err
		}
		registrar := namecheap.NewClient(ncUser, ncKey, ncUser, "", false)

		// Collect available hosting providers.
		hostingProviders := []provider.HostingProvider{
			firebase.NewProvider(),
			ghpages.NewProvider(),
		}

		return wizard.RunSetup(ctx, wizard.SetupConfig{
			Domain:           domain,
			Verbose:          verbose,
			DNSProvider:      dns,
			Registrar:        registrar,
			HostingProviders: hostingProviders,
		})
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
