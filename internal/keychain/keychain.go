package keychain

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	gokeychain "github.com/keybase/go-keychain"
	"golang.org/x/term"
)

// ServiceName is the Keychain service identifier for this application.
const ServiceName = "hostme"

// Credential account name constants.
const (
	AccountNamecheapAPIUser  = "namecheap-api-user"
	AccountNamecheapAPIKey   = "namecheap-api-key"
	AccountNamecheapUsername = "namecheap-username"
	AccountCloudflareToken   = "cloudflare-api-token"
)

// StoreCredential stores a credential value in the macOS Keychain.
// Any existing item for the same service/account is deleted first.
func StoreCredential(account, value string) error {
	// Delete any existing item; ignore not-found errors.
	_ = DeleteCredential(account)

	item := gokeychain.NewItem()
	item.SetSecClass(gokeychain.SecClassGenericPassword)
	item.SetService(ServiceName)
	item.SetAccount(account)
	item.SetData([]byte(value))
	item.SetAccessible(gokeychain.AccessibleWhenUnlocked)
	item.SetSynchronizable(gokeychain.SynchronizableNo)

	return gokeychain.AddItem(item)
}

// GetCredential retrieves a credential value from the macOS Keychain.
// Returns ("", nil) if the item does not exist.
func GetCredential(account string) (string, error) {
	query := gokeychain.NewItem()
	query.SetSecClass(gokeychain.SecClassGenericPassword)
	query.SetService(ServiceName)
	query.SetAccount(account)
	query.SetMatchLimit(gokeychain.MatchLimitOne)
	query.SetReturnData(true)

	results, err := gokeychain.QueryItem(query)
	if err == gokeychain.ErrorItemNotFound {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("keychain query failed for %s: %w", account, err)
	}
	if len(results) == 0 {
		return "", nil
	}
	return string(results[0].Data), nil
}

// DeleteCredential removes a credential from the macOS Keychain.
// Not-found errors are ignored.
func DeleteCredential(account string) error {
	item := gokeychain.NewItem()
	item.SetSecClass(gokeychain.SecClassGenericPassword)
	item.SetService(ServiceName)
	item.SetAccount(account)
	err := gokeychain.DeleteItem(item)
	if err == gokeychain.ErrorItemNotFound {
		return nil
	}
	return err
}

// HasCredential returns true if a non-empty credential exists for the given account.
func HasCredential(account string) bool {
	v, err := GetCredential(account)
	return err == nil && v != ""
}

// AuthNamecheap interactively collects NameCheap credentials and stores them in the Keychain.
func AuthNamecheap() error {
	fmt.Print("\n=== NameCheap Authentication ===\n")

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("NameCheap API User (your API-enabled account username): ")
	scanner.Scan()
	apiUser := strings.TrimSpace(scanner.Text())

	fmt.Print("NameCheap API Key: ")
	apiKeyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to read API key: %w", err)
	}
	apiKey := strings.TrimSpace(string(apiKeyBytes))

	fmt.Print("NameCheap Username (may be same as API user, press Enter to use API user): ")
	scanner.Scan()
	username := strings.TrimSpace(scanner.Text())
	if username == "" {
		username = apiUser
	}

	if err := StoreCredential(AccountNamecheapAPIUser, apiUser); err != nil {
		return fmt.Errorf("failed to store NameCheap API user: %w", err)
	}
	if err := StoreCredential(AccountNamecheapAPIKey, apiKey); err != nil {
		return fmt.Errorf("failed to store NameCheap API key: %w", err)
	}
	if err := StoreCredential(AccountNamecheapUsername, username); err != nil {
		return fmt.Errorf("failed to store NameCheap username: %w", err)
	}

	fmt.Print("\n✓ NameCheap credentials stored in Keychain.\n")
	return nil
}

// AuthCloudflare interactively collects a CloudFlare API token and stores it in the Keychain.
func AuthCloudflare() error {
	fmt.Print("\n=== CloudFlare Authentication ===\n")
	fmt.Print("\nTo create a CloudFlare API token:\n")
	fmt.Print("  1. Visit https://dash.cloudflare.com/profile/api-tokens\n")
	fmt.Print("  2. Click \"Create Token\"\n")
	fmt.Print("  3. Use the \"Edit zone DNS\" template\n")
	fmt.Print("  4. Scope it to the zones you need\n\n")

	fmt.Print("CloudFlare API Token: ")
	tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to read CloudFlare API token: %w", err)
	}
	token := strings.TrimSpace(string(tokenBytes))

	if err := StoreCredential(AccountCloudflareToken, token); err != nil {
		return fmt.Errorf("failed to store CloudFlare API token: %w", err)
	}

	fmt.Print("\n✓ CloudFlare API token stored in Keychain.\n")
	return nil
}

// GetNamecheapCredentials retrieves all three NameCheap credentials from the Keychain.
// Returns an error if any credential is missing.
func GetNamecheapCredentials() (apiUser, apiKey, username string, err error) {
	apiUser, err = GetCredential(AccountNamecheapAPIUser)
	if err != nil {
		return "", "", "", fmt.Errorf("missing NameCheap API user: %w", err)
	}
	if apiUser == "" {
		return "", "", "", fmt.Errorf("missing NameCheap API user")
	}

	apiKey, err = GetCredential(AccountNamecheapAPIKey)
	if err != nil {
		return "", "", "", fmt.Errorf("missing NameCheap API key: %w", err)
	}
	if apiKey == "" {
		return "", "", "", fmt.Errorf("missing NameCheap API key")
	}

	username, err = GetCredential(AccountNamecheapUsername)
	if err != nil {
		return "", "", "", fmt.Errorf("missing NameCheap username: %w", err)
	}
	if username == "" {
		return "", "", "", fmt.Errorf("missing NameCheap username")
	}

	return apiUser, apiKey, username, nil
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
