package credentials

import (
	gokeychain "github.com/keybase/go-keychain"
)

const serviceName = "hoston"

// KeychainStore implements Store using the macOS Keychain.
// After the first GetAll call, results are cached in memory so that
// subsequent Get calls never trigger another Keychain auth dialog.
type KeychainStore struct {
	cache map[string]string // nil until first GetAll
}

var _ Store = (*KeychainStore)(nil)

// NewKeychainStore returns a credential store backed by the macOS Keychain.
func NewKeychainStore() *KeychainStore {
	return &KeychainStore{}
}

// GetAll fetches every credential stored under the hoston service.
// It first lists account names (attributes only), then fetches each
// value individually. All results are cached so subsequent Get calls
// are served from memory with no further Keychain prompts.
func (k *KeychainStore) GetAll() (map[string]string, error) {
	// Step 1: list all account names (attributes only — no data).
	query := gokeychain.NewItem()
	query.SetSecClass(gokeychain.SecClassGenericPassword)
	query.SetService(serviceName)
	query.SetMatchLimit(gokeychain.MatchLimitAll)
	query.SetReturnAttributes(true)

	results, err := gokeychain.QueryItem(query)
	if err == gokeychain.ErrorItemNotFound {
		k.cache = make(map[string]string)
		return k.cache, nil
	}
	if err != nil {
		return nil, err
	}

	// Step 2: fetch data for each account individually.
	m := make(map[string]string, len(results))
	for _, r := range results {
		if r.Account == "" {
			continue
		}
		val, err := k.getSingle(r.Account)
		if err != nil {
			return nil, err
		}
		m[r.Account] = val
	}
	k.cache = m
	return m, nil
}

// getSingle fetches a single credential value (bypasses cache).
func (k *KeychainStore) getSingle(account string) (string, error) {
	query := gokeychain.NewItem()
	query.SetSecClass(gokeychain.SecClassGenericPassword)
	query.SetService(serviceName)
	query.SetAccount(account)
	query.SetMatchLimit(gokeychain.MatchLimitOne)
	query.SetReturnData(true)

	results, err := gokeychain.QueryItem(query)
	if err == gokeychain.ErrorItemNotFound || len(results) == 0 {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(results[0].Data), nil
}

func (k *KeychainStore) Get(account string) (string, error) {
	// Serve from cache when available (populated by GetAll / Preload).
	if k.cache != nil {
		return k.cache[account], nil
	}
	return k.getSingle(account)
}

func (k *KeychainStore) Set(account, value string) error {
	_ = k.Delete(account)

	item := gokeychain.NewItem()
	item.SetSecClass(gokeychain.SecClassGenericPassword)
	item.SetService(serviceName)
	item.SetAccount(account)
	item.SetData([]byte(value))
	item.SetAccessible(gokeychain.AccessibleWhenUnlocked)
	item.SetSynchronizable(gokeychain.SynchronizableNo)

	if err := gokeychain.AddItem(item); err != nil {
		return err
	}
	// Invalidate cache so next read picks up the new value.
	k.cache = nil
	return nil
}

func (k *KeychainStore) Delete(account string) error {
	item := gokeychain.NewItem()
	item.SetSecClass(gokeychain.SecClassGenericPassword)
	item.SetService(serviceName)
	item.SetAccount(account)
	err := gokeychain.DeleteItem(item)
	if err == gokeychain.ErrorItemNotFound {
		return nil
	}
	// Invalidate cache.
	k.cache = nil
	return err
}

func (k *KeychainStore) Has(account string) bool {
	v, err := k.Get(account)
	return err == nil && v != ""
}
