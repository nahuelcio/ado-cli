package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/nahuelcio/ado-cli/internal/auth"
)

const (
	DefaultConfigVersion  = 1
	DefaultProfileName    = "default"
	DefaultConfigDirName  = ".azure-devops-cli"
	DefaultConfigFileName = "config.yaml"
)

var (
	envOrg      = []string{"AZURE_DEVOPS_ORG", "AZURE_DEVOPS_ORGANIZATION"}
	envProject  = []string{"AZURE_DEVOPS_PROJECT"}
	envPAT      = []string{"AZURE_DEVOPS_PAT"}
	envAuthType = []string{"AZURE_DEVOPS_AUTH_TYPE"}
)

type Config struct {
	Version       int                `json:"version" yaml:"version"`
	ActiveProfile string             `json:"activeProfile" yaml:"activeProfile"`
	Profiles      map[string]Profile `json:"profiles" yaml:"profiles"`
}

type Profile struct {
	Organization string          `json:"organization" yaml:"organization"`
	Project      string          `json:"project" yaml:"project"`
	Auth         auth.AuthConfig `json:"auth" yaml:"auth"`
}

type ConfigLoader struct {
	config       Config
	configPath   string
	envOverrides map[string]string
}

func DefaultConfig() Config {
	return Config{
		Version:       DefaultConfigVersion,
		ActiveProfile: DefaultProfileName,
		Profiles: map[string]Profile{
			DefaultProfileName: {
				Organization: "",
				Project:      "",
				Auth: auth.AuthConfig{
					Type:   auth.AuthTypePAT,
					Scopes: []string{"vso.packaging", "vso.code", "vso.project"},
				},
			},
		},
	}
}

func NewConfigLoader(configPath string) *ConfigLoader {
	if configPath == "" {
		configPath = GetDefaultConfigPath()
	}

	loader := &ConfigLoader{
		config:       DefaultConfig(),
		configPath:   configPath,
		envOverrides: make(map[string]string),
	}

	loader.loadEnvOverrides()
	return loader
}

func GetDefaultConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = os.Getenv("USERPROFILE")
	}
	return filepath.Join(homeDir, DefaultConfigDirName, DefaultConfigFileName)
}

func (c *ConfigLoader) loadEnvOverrides() {
	for _, envVar := range envOrg {
		if value := os.Getenv(envVar); value != "" {
			c.envOverrides["organization"] = value
			break
		}
	}

	for _, envVar := range envProject {
		if value := os.Getenv(envVar); value != "" {
			c.envOverrides["project"] = value
			break
		}
	}

	for _, envVar := range envPAT {
		if value := os.Getenv(envVar); value != "" {
			c.envOverrides["pat"] = value
			break
		}
	}

	for _, envVar := range envAuthType {
		if value := os.Getenv(envVar); value != "" {
			c.envOverrides["authType"] = value
			break
		}
	}
}

func (c *ConfigLoader) IsEnvOverridden(key string) bool {
	_, exists := c.envOverrides[key]
	return exists
}

func (c *ConfigLoader) GetEnvOverridesInfo() map[string]map[string]string {
	info := make(map[string]map[string]string)

	for key, value := range c.envOverrides {
		source := ""
		displayValue := value

		switch key {
		case "organization":
			for _, envVar := range envOrg {
				if os.Getenv(envVar) != "" {
					source = envVar
					break
				}
			}
		case "project":
			source = envProject[0]
		case "pat":
			source = envPAT[0]
			displayValue = "***"
		case "authType":
			source = envAuthType[0]
		}

		info[key] = map[string]string{
			"value":  displayValue,
			"source": source,
		}
	}

	return info
}

func (c *ConfigLoader) Load() (Config, error) {
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return c.config, nil
		}
		return c.config, fmt.Errorf("failed to read config file: %w", err)
	}

	var loaded Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		return c.config, fmt.Errorf("failed to parse config file: %w", err)
	}

	c.config = c.mergeWithDefaults(loaded)
	return c.config, nil
}

func (c *ConfigLoader) Save() error {
	configDir := filepath.Dir(c.configPath)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(c.configPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *ConfigLoader) Get(key string) interface{} {
	keys := splitKey(key)
	value := interface{}(c.config)

	for _, k := range keys {
		if m, ok := value.(map[string]interface{}); ok {
			value = m[k]
		} else {
			return nil
		}
	}

	return value
}

func (c *ConfigLoader) Set(key string, value interface{}) {
	keys := splitKey(key)
	if len(keys) == 0 {
		return
	}

	lastKey := keys[len(keys)-1]
	keys = keys[:len(keys)-1]

	obj := interface{}(&c.config)
	for _, k := range keys {
		if m, ok := obj.(*map[string]interface{}); ok {
			if _, exists := (*m)[k]; !exists {
				(*m)[k] = make(map[string]interface{})
			}
			next := (*m)[k]
			obj = &next
		}
	}

	if m, ok := obj.(*map[string]interface{}); ok {
		(*m)[lastKey] = value
	}
}

func (c *ConfigLoader) GetOrganization() string {
	if value, ok := c.envOverrides["organization"]; ok {
		return value
	}
	profile := c.GetActiveProfile()
	if profile != nil {
		return profile.Organization
	}
	return ""
}

func (c *ConfigLoader) GetProject() string {
	if value, ok := c.envOverrides["project"]; ok {
		return value
	}
	profile := c.GetActiveProfile()
	if profile != nil {
		return profile.Project
	}
	return ""
}

func (c *ConfigLoader) GetAuth() *auth.AuthConfig {
	authConfig := &auth.AuthConfig{Type: auth.AuthTypePAT}

	if profile := c.GetActiveProfile(); profile != nil {
		authConfig = &profile.Auth
	}

	if value, ok := c.envOverrides["authType"]; ok {
		authType := auth.AuthType(value)
		if authType == auth.AuthTypePAT || authType == auth.AuthTypeOAuth ||
			authType == auth.AuthTypeManagedIdentity || authType == auth.AuthTypeSPN {
			authConfig.Type = authType
		}
	}

	if value, ok := c.envOverrides["pat"]; ok {
		authConfig.PAT = value
		authConfig.Type = auth.AuthTypePAT
	}

	return authConfig
}

func (c *ConfigLoader) GetActiveProfile() *Profile {
	profile, ok := c.config.Profiles[c.config.ActiveProfile]
	if !ok {
		return nil
	}
	return &profile
}

func (c *ConfigLoader) GetActiveProfileWithOverrides() *Profile {
	profile := c.GetActiveProfile()
	if profile == nil {
		return nil
	}

	return &Profile{
		Organization: c.GetOrganization(),
		Project:      c.GetProject(),
		Auth:         *c.GetAuth(),
	}
}

func (c *ConfigLoader) GetActiveProfileName() string {
	return c.config.ActiveProfile
}

func (c *ConfigLoader) SetActiveProfile(profileName string) {
	c.config.ActiveProfile = profileName
}

func (c *ConfigLoader) SetProfile(name string, profile Profile) {
	c.config.Profiles[name] = profile
}

func (c *ConfigLoader) GetProfileNames() []string {
	names := make([]string, 0, len(c.config.Profiles))
	for name := range c.config.Profiles {
		names = append(names, name)
	}
	return names
}

func (c *ConfigLoader) GetConfig() Config {
	return c.config
}

func (c *ConfigLoader) mergeWithDefaults(loaded Config) Config {
	merged := DefaultConfig()

	if loaded.Version != 0 {
		merged.Version = loaded.Version
	}
	if loaded.ActiveProfile != "" {
		merged.ActiveProfile = loaded.ActiveProfile
	}

	if loaded.Profiles != nil {
		for name, profile := range loaded.Profiles {
			merged.Profiles[name] = Profile{
				Organization: profile.Organization,
				Project:      profile.Project,
				Auth: auth.AuthConfig{
					Type:         profile.Auth.Type,
					PAT:          profile.Auth.PAT,
					ClientID:     profile.Auth.ClientID,
					ClientSecret: profile.Auth.ClientSecret,
					TenantID:     profile.Auth.TenantID,
					ManagedID:    profile.Auth.ManagedID,
					Scopes:       profile.Auth.Scopes,
				},
			}
		}
	}

	return merged
}

func splitKey(key string) []string {
	var keys []string
	var current []rune

	for _, r := range key {
		if r == '.' {
			if len(current) > 0 {
				keys = append(keys, string(current))
				current = nil
			}
		} else {
			current = append(current, r)
		}
	}

	if len(current) > 0 {
		keys = append(keys, string(current))
	}

	return keys
}

func GetConfigDir() string {
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = os.Getenv("USERPROFILE")
	}
	return filepath.Join(homeDir, DefaultConfigDirName)
}
