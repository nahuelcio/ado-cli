package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/99designs/keyring"
)

const (
	ServicePAT     = "azure-devops-cli-pat"
	ServiceAAD     = "azure-devops-cli-aad"
	ServiceRefresh = "azure-devops-cli-refresh"
)

type CredentialManager struct {
	ring        keyring.Keyring
	storageDir  string
	storageFile string
	backend     string
}

type CredentialEntry struct {
	Service    string `json:"service"`
	Account    string `json:"account"`
	Credential string `json:"credential"`
	CreatedAt  int64  `json:"createdAt"`
}

func NewCredentialManager(storageDir string) (*CredentialManager, error) {
	if storageDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = os.Getenv("USERPROFILE")
		}
		storageDir = filepath.Join(homeDir, ".azure-devops-cli")
	}

	storageFile := filepath.Join(storageDir, "credentials.json")

	ring, err := keyring.Open(keyring.Config{
		ServiceName:             "azure-devops-cli",
		KeychainAccessibility:   keyring.WhenUnlockedThisDeviceOnly,
		FileDir:                 storageDir,
		FilePasswordFunc:        func(s string) (string, error) { return "", nil },
		LibSecretCollectionName: "azure-devops-cli",
	})

	backend := "keyring"
	if err != nil {
		backend = "file"
		fmt.Printf("[CredentialManager] Platform credential manager unavailable. Falling back to file-based storage.\n")
		fmt.Printf("[CredentialManager] Warning: File-based storage is less secure.\n")
	}

	return &CredentialManager{
		ring:        ring,
		storageDir:  storageDir,
		storageFile: storageFile,
		backend:     backend,
	}, nil
}

func (c *CredentialManager) SavePAT(service, account, pat string) error {
	key := c.makeKey(service, account)
	if c.backend == "keyring" {
		err := c.ring.Set(keyring.Item{
			Key:  key,
			Data: []byte(pat),
			Label: fmt.Sprintf("Azure DevOps CLI - %s", account),
		})
		if err != nil {
			return fmt.Errorf("failed to save credential via keyring: %w", err)
		}
		return nil
	}
	return c.saveToFileStorage(service, account, pat)
}

func (c *CredentialManager) GetPAT(service, account string) (string, error) {
	key := c.makeKey(service, account)
	if c.backend == "keyring" {
		item, err := c.ring.Get(key)
		if err != nil {
			if err == keyring.ErrItemNotFound {
				return "", nil
			}
			return "", fmt.Errorf("failed to get credential via keyring: %w", err)
		}
		return string(item.Data), nil
	}
	return c.getFromFileStorage(service, account)
}

func (c *CredentialManager) DeletePAT(service, account string) error {
	key := c.makeKey(service, account)
	if c.backend == "keyring" {
		err := c.ring.Remove(key)
		if err != nil && err != keyring.ErrItemNotFound {
			return fmt.Errorf("failed to delete credential via keyring: %w", err)
		}
		return nil
	}
	return c.deleteFromFileStorage(service, account)
}

func (c *CredentialManager) makeKey(service, account string) string {
	return fmt.Sprintf("%s::%s", service, account)
}

func (c *CredentialManager) saveToFileStorage(service, account, credential string) error {
	entries, err := c.loadFileStorage()
	if err != nil {
		entries = []CredentialEntry{}
	}

	entry := CredentialEntry{
		Service:    service,
		Account:    account,
		Credential: credential,
		CreatedAt:  0,
	}

	found := false
	for i, e := range entries {
		if e.Service == service && e.Account == account {
			entries[i] = entry
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, entry)
	}

	if err := os.MkdirAll(c.storageDir, 0o700); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	data, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	if err := os.WriteFile(c.storageFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

func (c *CredentialManager) getFromFileStorage(service, account string) (string, error) {
	entries, err := c.loadFileStorage()
	if err != nil {
		return "", nil
	}

	for _, e := range entries {
		if e.Service == service && e.Account == account {
			return e.Credential, nil
		}
	}

	return "", nil
}

func (c *CredentialManager) deleteFromFileStorage(service, account string) error {
	entries, err := c.loadFileStorage()
	if err != nil {
		return nil
	}

	newEntries := []CredentialEntry{}
	for _, e := range entries {
		if !(e.Service == service && e.Account == account) {
			newEntries = append(newEntries, e)
		}
	}

	if len(newEntries) == 0 {
		if err := os.Remove(c.storageFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove credentials file: %w", err)
		}
		return nil
	}

	data, err := json.Marshal(newEntries)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	if err := os.WriteFile(c.storageFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

func (c *CredentialManager) loadFileStorage() ([]CredentialEntry, error) {
	data, err := os.ReadFile(c.storageFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []CredentialEntry{}, nil
		}
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	var entries []CredentialEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	return entries, nil
}

func (c *CredentialManager) GetBackend() string {
	return c.backend
}

func (c *CredentialManager) IsPlatformManagerAvailable() bool {
	return c.backend == "keyring"
}

func (c *CredentialManager) GetStoragePath() string {
	return c.storageFile
}
