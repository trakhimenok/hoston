// Package credentials defines an interface for credential storage and provides
// a default implementation backed by the macOS Keychain.
package credentials

// Store abstracts secure credential storage so implementations can vary
// by platform (macOS Keychain, Linux secret-service, env vars, etc.).
type Store interface {
	// Get retrieves a credential value. Returns ("", nil) if not found.
	Get(account string) (string, error)

	// GetAll retrieves all credentials for the service at once.
	// This allows implementations to minimise auth prompts (e.g. a single
	// macOS Keychain dialog instead of one per credential).
	GetAll() (map[string]string, error)

	// Set stores a credential value, replacing any existing value.
	Set(account, value string) error

	// Delete removes a credential. Not-found is not an error.
	Delete(account string) error

	// Has returns true if a non-empty credential exists for the account.
	Has(account string) bool
}
