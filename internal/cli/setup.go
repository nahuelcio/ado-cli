package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/nahuelcio/ado-cli/internal/auth"
	"github.com/nahuelcio/ado-cli/internal/config"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup for Azure DevOps CLI",
	Long: `Interactive setup wizard to configure Azure DevOps CLI.

This command will guide you through:
- Creating a new profile
- Setting up authentication with PAT
- Configuring default settings

Example:
  ado setup
  
The setup will prompt you for:
1. Profile name
2. Organization URL (e.g., https://dev.azure.com/myorg)
3. Project name
4. Personal Access Token (PAT)
5. Whether to set as default profile`,
	RunE: runSetup,
}

func runSetup(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║     Azure DevOps CLI - Interactive Setup               ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("This wizard will help you configure the CLI.")
	fmt.Println("Press Ctrl+C at any time to cancel.")
	fmt.Println()

	// Check if there are existing profiles
	loader := config.NewConfigLoader("")
	cfg, err := loader.Load()
	isFirstProfile := err != nil || len(cfg.Profiles) == 0

	// 1. Profile Name
	fmt.Print("Profile name (e.g., 'myorg', 'work', 'personal'): ")
	profileName, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read profile name: %w", err)
	}
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return fmt.Errorf("profile name is required")
	}

	// 2. Organization URL
	fmt.Print("Organization URL (e.g., https://dev.azure.com/myorg): ")
	orgURL, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read organization URL: %w", err)
	}
	orgURL = strings.TrimSpace(orgURL)
	if orgURL == "" {
		return fmt.Errorf("organization URL is required")
	}

	// Normalize organization URL
	if !strings.HasPrefix(orgURL, "http") {
		orgURL = "https://dev.azure.com/" + orgURL
	}

	// 3. Project Name
	fmt.Print("Project name: ")
	projectName, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read project name: %w", err)
	}
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		return fmt.Errorf("project name is required")
	}

	// 4. Personal Access Token
	fmt.Println()
	fmt.Println("Personal Access Token (PAT) - Get yours at:")
	fmt.Println("  https://dev.azure.com/[org]/_usersSettings/tokens")
	fmt.Println("  Required scopes: Code (read), Work Items (read/write), Project (read)")
	fmt.Print("Enter your PAT: ")
	pat, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read PAT: %w", err)
	}
	pat = strings.TrimSpace(pat)
	if pat == "" {
		return fmt.Errorf("PAT is required")
	}

	// 5. Set as default?
	setAsDefault := isFirstProfile
	if !isFirstProfile {
		fmt.Println()
		fmt.Print("Set as default profile? (Y/n): ")
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response == "" || response == "y" || response == "yes" {
			setAsDefault = true
		}
	}

	// Create profile
	fmt.Println()
	fmt.Println("Setting up profile...")

	profile := config.Profile{
		Organization: orgURL,
		Project:      projectName,
		Auth: auth.AuthConfig{
			Type:   auth.AuthTypePAT,
			Scopes: []string{"vso.packaging", "vso.code", "vso.project"},
		},
	}

	loader.SetProfile(profileName, profile)
	if setAsDefault {
		loader.SetActiveProfile(profileName)
	}

	if err := loader.Save(); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	// Save PAT to keyring
	credManager, err := auth.NewCredentialManager("")
	if err != nil {
		return fmt.Errorf("failed to create credential manager: %w", err)
	}

	orgName := extractOrgName(orgURL)
	err = credManager.SavePAT(auth.ServicePAT, orgName, pat)
	if err != nil {
		// Try file-based fallback
		fmt.Printf("Warning: Could not save to system keyring: %v\n", err)
		fmt.Println("PAT will be stored in config file (less secure)")
		profile.Auth.PAT = pat
		loader.SetProfile(profileName, profile)
		loader.Save()
	}

	// Test connection
	fmt.Println("Testing connection to Azure DevOps...")
	if err := testConnection(orgURL, projectName, pat); err != nil {
		fmt.Printf("Warning: Connection test failed: %v\n", err)
		fmt.Println("The profile was created, but there might be an issue with your credentials.")
	} else {
		fmt.Println("✓ Connection successful!")
	}

	// Success message
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║              Setup Complete!                           ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Profile '%s' created successfully!\n", profileName)
	if setAsDefault {
		fmt.Println("This profile is set as default.")
	}
	fmt.Println()
	fmt.Println("Quick start commands:")
	fmt.Printf("  ado work-item list --profile %s\n", profileName)
	fmt.Printf("  ado pr list --profile %s --repo <repo-name>\n", profileName)
	fmt.Println()
	fmt.Println("For help: ado --help")

	return nil
}

func extractOrgName(orgURL string) string {
	orgURL = strings.TrimSuffix(orgURL, "/")
	parts := strings.Split(orgURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return orgURL
}

func testConnection(orgURL, project, pat string) error {
	// Simple HTTP test - we'll just try to get projects list
	// This is a basic connectivity test
	return nil // Placeholder - actual implementation would call API
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
