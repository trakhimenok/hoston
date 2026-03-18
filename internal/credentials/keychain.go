package credentials

import (
	gokeychain "github.com/keybase/go-keychain"
)

const serviceName = "hoston"

// KeychainStore implements Store using the macOS Keychain.
type KeychainStore struct{}

var _ Store = (*KeychainStore)(nil)

// NewKeychainStore returns a credential store backed by the macOS Keychain.
func NewKeychainStore() *KeychainStore {
	return &KeychainStore{}
}

func (k *KeychainStore) Get(account string) (string, error) {
	query := gokeychain.NewItem()
	query.SetSecClass(gokeychain.SecClassGenericPassword)
	query.SetService(serviceName)
	query.SetAccount(account)
	query.SetMatchLimit(gokeychain.MatchLimitOne)
	query.SetReturnData(true)

	results, err := gokeychain.QueryItem(query)
	if err == gokeychain.ErrorItemNotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", nil
	}
	return string(results[0].Data), nil
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

	return gokeychain.AddItem(item)
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
	return err
}

func (k *KeychainStore) Has(account string) bool {
	v, err := k.Get(account)
	return err == nil && v != ""
}
