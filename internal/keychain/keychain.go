package keychain

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/trakhimenok/hoston/internal/credentials"
)

// ServiceName is the Keychain service identifier for this application.
const ServiceName = "hoston"

// Credential account name constants.
const (
	AccountNamecheapAPIKey   = "namecheap-api-key"
	AccountNamecheapUsername = "namecheap-username"
	AccountCloudflareToken   = "cloudflare-api-token"
)

// store is the backing credential store. Defaults to macOS Keychain.
var store credentials.Store = credentials.NewKeychainStore()

// SetStore replaces the backing credential store (useful for testing or
// alternative platforms).
func SetStore(s credentials.Store) { store = s }

// StoreCredential stores a credential value.
func StoreCredential(account, value string) error {
	return store.Set(account, value)
}

// GetCredential retrieves a credential value.
func GetCredential(account string) (string, error) {
	return store.Get(account)
}

// DeleteCredential removes a credential.
func DeleteCredential(account string) error {
	return store.Delete(account)
}

// HasCredential returns true if a non-empty credential exists.
func HasCredential(account string) bool {
	return store.Has(account)
}

// AuthNamecheap interactively collects NameCheap credentials and stores them.
func AuthNamecheap() error {
	var username, apiKey string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("NameCheap Username").
				Value(&username).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("username is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("NameCheap API Key").
				EchoMode(huh.EchoModePassword).
				Value(&apiKey).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("API key is required")
					}
					return nil
				}),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}

	username = strings.TrimSpace(username)
	apiKey = strings.TrimSpace(apiKey)

	if err := StoreCredential(AccountNamecheapUsername, username); err != nil {
		return fmt.Errorf("failed to store NameCheap username: %w", err)
	}
	if err := StoreCredential(AccountNamecheapAPIKey, apiKey); err != nil {
		return fmt.Errorf("failed to store NameCheap API key: %w", err)
	}

	fmt.Println("✓ NameCheap credentials stored.")
	return nil
}

// AuthCloudflare interactively collects a CloudFlare API token and stores it.
func AuthCloudflare() error {
	var token string

	fmt.Println("Create an API token at:")
	fmt.Println("  https://dash.cloudflare.com/profile/api-tokens")
	fmt.Println()
	fmt.Println("Required permissions:")
	fmt.Println("  • Zone > Zone: Edit")
	fmt.Println("  • Zone > DNS:  Edit")
	fmt.Println()

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("CloudFlare API Token").
				EchoMode(huh.EchoModePassword).
				Value(&token).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("token is required")
					}
					return nil
				}),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}

	token = strings.TrimSpace(token)

	if err := StoreCredential(AccountCloudflareToken, token); err != nil {
		return fmt.Errorf("failed to store CloudFlare API token: %w", err)
	}

	fmt.Println("✓ CloudFlare API token stored.")
	return nil
}

// GetNamecheapCredentials retrieves NameCheap credentials from the Keychain.
// Returns an error if any credential is missing.
func GetNamecheapCredentials() (username, apiKey string, err error) {
	username, err = GetCredential(AccountNamecheapUsername)
	if err != nil {
		return "", "", fmt.Errorf("missing NameCheap username: %w", err)
	}
	if username == "" {
		return "", "", fmt.Errorf("missing NameCheap username")
	}

	apiKey, err = GetCredential(AccountNamecheapAPIKey)
	if err != nil {
		return "", "", fmt.Errorf("missing NameCheap API key: %w", err)
	}
	if apiKey == "" {
		return "", "", fmt.Errorf("missing NameCheap API key")
	}

	return username, apiKey, nil
}

// GetCloudflareToken retrieves the CloudFlare API token from the Keychain.
// Returns an error if the token is missing.
func GetCloudflareToken() (string, error) {
	token, err := GetCredential(AccountCloudflareToken)
	if err != nil {
		return "", fmt.Errorf("missing CloudFlare API token: %w", err)
	}
	if token == "" {
		return "", fmt.Errorf("missing CloudFlare API token")
	}
	return token, nil
}
