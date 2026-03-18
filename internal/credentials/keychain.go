package credentials

import (
	"encoding/json"

	gokeychain "github.com/keybase/go-keychain"
)

const (
	serviceName = "hoston"
	accountName = "credentials" // single keychain item holds all credentials
)

// KeychainStore implements Store using a single macOS Keychain item that
// holds all credentials as a JSON map. One item = one password prompt.
type KeychainStore struct {
	cache map[string]string // nil until first load
}

var _ Store = (*KeychainStore)(nil)

// NewKeychainStore returns a credential store backed by the macOS Keychain.
func NewKeychainStore() *KeychainStore {
	return &KeychainStore{}
}

// load reads the single keychain item into the in-memory cache.
func (k *KeychainStore) load() error {
	if k.cache != nil {
		return nil
	}

	raw, err := readItem()
	if err != nil {
		return err
	}
	if raw == "" {
		// Try migrating legacy per-account items.
		k.cache = migrateLegacy()
		if len(k.cache) > 0 {
			return k.flush()
		}
		k.cache = make(map[string]string)
		return nil
	}

	m := make(map[string]string)
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return err
	}
	k.cache = m
	return nil
}

// flush writes the cache back to the single keychain item.
func (k *KeychainStore) flush() error {
	b, err := json.Marshal(k.cache)
	if err != nil {
		return err
	}
	return writeItem(string(b))
}

func (k *KeychainStore) GetAll() (map[string]string, error) {
	if err := k.load(); err != nil {
		return nil, err
	}
	cp := make(map[string]string, len(k.cache))
	for key, val := range k.cache {
		cp[key] = val
	}
	return cp, nil
}

func (k *KeychainStore) Get(account string) (string, error) {
	if err := k.load(); err != nil {
		return "", err
	}
	return k.cache[account], nil
}

func (k *KeychainStore) Set(account, value string) error {
	if err := k.load(); err != nil {
		return err
	}
	k.cache[account] = value
	return k.flush()
}

func (k *KeychainStore) Delete(account string) error {
	if err := k.load(); err != nil {
		return err
	}
	delete(k.cache, account)
	return k.flush()
}

func (k *KeychainStore) Has(account string) bool {
	if err := k.load(); err != nil {
		return false
	}
	return k.cache[account] != ""
}

// --- low-level keychain helpers (single item) ---

func readItem() (string, error) {
	query := gokeychain.NewItem()
	query.SetSecClass(gokeychain.SecClassGenericPassword)
	query.SetService(serviceName)
	query.SetAccount(accountName)
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

func writeItem(data string) error {
	// Delete then re-create to update.
	del := gokeychain.NewItem()
	del.SetSecClass(gokeychain.SecClassGenericPassword)
	del.SetService(serviceName)
	del.SetAccount(accountName)
	_ = gokeychain.DeleteItem(del)

	item := gokeychain.NewItem()
	item.SetSecClass(gokeychain.SecClassGenericPassword)
	item.SetService(serviceName)
	item.SetAccount(accountName)
	item.SetData([]byte(data))
	item.SetAccessible(gokeychain.AccessibleWhenUnlocked)
	item.SetSynchronizable(gokeychain.SynchronizableNo)
	return gokeychain.AddItem(item)
}

// --- legacy migration (old per-account items) ---

var legacyAccounts = []string{
	"cloudflare-api-token",
	"namecheap-username",
	"namecheap-api-key",
}

func migrateLegacy() map[string]string {
	m := make(map[string]string)
	for _, acct := range legacyAccounts {
		query := gokeychain.NewItem()
		query.SetSecClass(gokeychain.SecClassGenericPassword)
		query.SetService(serviceName)
		query.SetAccount(acct)
		query.SetMatchLimit(gokeychain.MatchLimitOne)
		query.SetReturnData(true)

		results, err := gokeychain.QueryItem(query)
		if err != nil || len(results) == 0 {
			continue
		}
		m[acct] = string(results[0].Data)

		// Remove old item after reading.
		del := gokeychain.NewItem()
		del.SetSecClass(gokeychain.SecClassGenericPassword)
		del.SetService(serviceName)
		del.SetAccount(acct)
		_ = gokeychain.DeleteItem(del)
	}
	return m
}
