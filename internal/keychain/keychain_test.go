// Package keychain provides macOS Keychain integration.
//
// Tests in this file only verify compile-time constants and package-level
// invariants. They deliberately avoid any interaction with the macOS Security
// framework (Keychain Services) because that requires a logged-in user session
// and may display interactive prompts — making it unsuitable for CI.
package keychain

import "testing"

// TestServiceName verifies the Keychain service identifier is set to the
// expected application name.
func TestServiceName(t *testing.T) {
	if ServiceName != "hostme" {
		t.Errorf("ServiceName = %q; want %q", ServiceName, "hostme")
	}
}

// TestAccountConstants verifies that every exported account-name constant is
// non-empty.  Empty constants would silently corrupt Keychain lookups.
func TestAccountConstants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value string
	}{
		{"AccountNamecheapAPIUser", AccountNamecheapAPIUser},
		{"AccountNamecheapAPIKey", AccountNamecheapAPIKey},
		{"AccountNamecheapUsername", AccountNamecheapUsername},
		{"AccountCloudflareToken", AccountCloudflareToken},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.value == "" {
				t.Errorf("constant %s must not be empty", tc.name)
			}
		})
	}
}

// TestAccountConstantsAreUnique verifies that no two account-name constants
// share the same value, which would cause one credential to overwrite another.
func TestAccountConstantsAreUnique(t *testing.T) {
	t.Parallel()

	constants := map[string]string{
		"AccountNamecheapAPIUser":  AccountNamecheapAPIUser,
		"AccountNamecheapAPIKey":   AccountNamecheapAPIKey,
		"AccountNamecheapUsername": AccountNamecheapUsername,
		"AccountCloudflareToken":   AccountCloudflareToken,
	}

	seen := make(map[string]string, len(constants)) // value → first name that used it
	for name, value := range constants {
		if first, exists := seen[value]; exists {
			t.Errorf("constant %s has the same value %q as %s; all account names must be unique",
				name, value, first)
		}
		seen[value] = name
	}
}
