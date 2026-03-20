package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nahuelcio/ado-cli/internal/config"
	"github.com/spf13/cobra"
)

type authConfig struct {
	Org     string
	Project string
	PAT     string
}

func truncateString(s string, maxLen int) string {
	if maxLen < 3 {
		if len(s) <= maxLen {
			return s
		}
		return s[:maxLen] + "..."
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func getConfigAndAuth(cmd *cobra.Command) (*config.ConfigLoader, *authConfig, error) {
	cfg := config.NewConfigLoader("")

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName != "" {
		cfg.SetActiveProfile(profileName)
	}

	if _, err := cfg.Load(); err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	org := cfg.GetOrganization()
	proj := cfg.GetProject()
	auth := cfg.GetAuth()

	if org == "" {
		return nil, nil, fmt.Errorf("organization not configured. Use --profile or set AZURE_DEVOPS_ORG")
	}
	if proj == "" {
		return nil, nil, fmt.Errorf("project not configured. Use --profile or set AZURE_DEVOPS_PROJECT")
	}

	pat := auth.PAT
	if pat == "" {
		return nil, nil, fmt.Errorf("PAT not configured. Use --profile or set AZURE_DEVOPS_PAT")
	}

	return cfg, &authConfig{
		Org:     org,
		Project: proj,
		PAT:     pat,
	}, nil
}

func getRepoFromFlags(cmd *cobra.Command) (string, error) {
	repo, _ := cmd.Flags().GetString("repo")
	if repo != "" {
		return repo, nil
	}

	envRepo := os.Getenv("AZURE_DEVOPS_REPO")
	if envRepo != "" {
		return envRepo, nil
	}

	activeProfile := ""
	if cfg := config.NewConfigLoader(""); cfg != nil {
		activeProfile = cfg.GetActiveProfileName()
	}

	return "", fmt.Errorf("repository not configured. Use --repo <repo-name>, set AZURE_DEVOPS_REPO environment variable, or configure repo in your profile. Example: ado pr list --profile %s --repo my-repo", activeProfile)
}

func getConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("USERPROFILE")
	}
	if homeDir == "" {
		homeDir = os.TempDir()
	}
	return filepath.Join(homeDir, ".azure-devops-cli")
}

func cleanHTML(input string) string {
	if input == "" {
		return ""
	}

	result := input
	result = strings.ReplaceAll(result, "<br>", "\n")
	result = strings.ReplaceAll(result, "<br/>", "\n")
	result = strings.ReplaceAll(result, "<br />", "\n")
	result = strings.ReplaceAll(result, "</div>", "\n")
	result = strings.ReplaceAll(result, "</p>", "\n")

	inTag := false
	var output strings.Builder
	for _, r := range result {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			output.WriteRune(r)
		}
	}

	cleaned := output.String()
	cleaned = strings.ReplaceAll(cleaned, "&nbsp;", " ")
	cleaned = strings.ReplaceAll(cleaned, "&lt;", "<")
	cleaned = strings.ReplaceAll(cleaned, "&gt;", ">")
	cleaned = strings.ReplaceAll(cleaned, "&amp;", "&")
	cleaned = strings.ReplaceAll(cleaned, "\n\n\n", "\n\n")
	cleaned = strings.TrimSpace(cleaned)

	return cleaned
}
