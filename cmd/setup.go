package cmd

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
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

		// NameCheap API requires the caller's public IP for whitelisting.
		clientIP := getPublicIP(ctx)
		if clientIP == "" {
			var ip string
			if err := huh.NewInput().
				Title("Could not detect public IP — enter manually").
				Placeholder("203.0.113.1").
				Value(&ip).
				Run(); err != nil {
				return err
			}
			clientIP = ip
		}

		registrar := namecheap.NewClient(ncUser, ncKey, ncUser, clientIP, false)

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

func getPublicIP(ctx context.Context) string {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org", nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) != nil {
		return ip
	}
	return ""
}
