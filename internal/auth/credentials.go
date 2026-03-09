package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
		FileDir:                 storageDir,
		FilePasswordFunc:        func(_ string) (string, error) { return "", nil },
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
	account = CredentialAccount(account)
	key := c.makeKey(service, account)
	if c.backend == "keyring" {
		err := c.ring.Set(keyring.Item{
			Key:   key,
			Data:  []byte(pat),
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
	account = CredentialAccount(account)
	key := c.makeKey(service, account)
	if c.backend == "keyring" {
		item, err := c.ring.Get(key)
		if err != nil {
			if legacyPAT, legacyErr := c.getLegacyPAT(service, account); legacyErr == nil && legacyPAT != "" {
				return legacyPAT, nil
			}
			if isKeyringNotFoundError(err) {
				return "", nil
			}
			return "", fmt.Errorf("failed to get credential via keyring: %w", err)
		}
		return string(item.Data), nil
	}

	pat, err := c.getFromFileStorage(service, account)
	if err != nil || pat != "" {
		return pat, err
	}
	return c.getLegacyPAT(service, account)
}

func (c *CredentialManager) DeletePAT(service, account string) error {
	normalizedAccount := CredentialAccount(account)
	key := c.makeKey(service, normalizedAccount)
	if c.backend == "keyring" {
		err := c.ring.Remove(key)
		if err != nil && !isKeyringNotFoundError(err) {
			return fmt.Errorf("failed to delete credential via keyring: %w", err)
		}

		legacyKey := c.makeKey(service, account)
		if legacyKey != key {
			_ = c.ring.Remove(legacyKey)
		}
		return nil
	}

	if err := c.deleteFromFileStorage(service, normalizedAccount); err != nil {
		return err
	}
	if account != normalizedAccount {
		return c.deleteFromFileStorage(service, account)
	}
	return nil
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

func isKeyringNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	var keyNotFoundError interface{ Error() string }
	if errors.As(err, &keyNotFoundError) {
		msg := strings.ToLower(keyNotFoundError.Error())
		return strings.Contains(msg, "not found")
	}

	return false
}

func CredentialAccount(account string) string {
	account = strings.TrimSpace(account)
	account = strings.TrimSuffix(account, "/")
	if account == "" {
		return ""
	}

	if strings.HasPrefix(account, "http://") || strings.HasPrefix(account, "https://") {
		parsed, err := url.Parse(account)
		if err == nil {
			host := strings.ToLower(parsed.Hostname())
			path := strings.Trim(parsed.Path, "/")

			if host == "dev.azure.com" && path != "" {
				parts := strings.Split(path, "/")
				return parts[0]
			}

			if strings.HasSuffix(host, ".visualstudio.com") {
				return strings.TrimSuffix(host, ".visualstudio.com")
			}
		}
	}

	return account
}

func (c *CredentialManager) getLegacyPAT(service, account string) (string, error) {
	if account == "" {
		return "", nil
	}

	legacyAccounts := []string{
		account,
		"https://dev.azure.com/" + account,
		"https://" + account + ".visualstudio.com",
		"https://" + account + ".visualstudio.com/",
	}

	for _, legacyAccount := range legacyAccounts {
		if legacyAccount == CredentialAccount(legacyAccount) {
			continue
		}

		key := c.makeKey(service, legacyAccount)
		if c.backend == "keyring" {
			item, err := c.ring.Get(key)
			if err == nil {
				return string(item.Data), nil
			}
			if err != nil && !isKeyringNotFoundError(err) {
				return "", fmt.Errorf("failed to get legacy credential via keyring: %w", err)
			}
			continue
		}

		pat, err := c.getFromFileStorage(service, legacyAccount)
		if err != nil {
			return "", err
		}
		if pat != "" {
			return pat, nil
		}
	}

	return "", nil
}
