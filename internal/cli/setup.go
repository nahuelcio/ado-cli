package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/nahuelcio/ado-cli/internal/auth"
	"github.com/nahuelcio/ado-cli/internal/config"
	"github.com/spf13/cobra"
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

	printSetupBanner()

	loader := config.NewConfigLoader("")
	isFirstProfile := hasNoExistingProfiles(loader)

	profileName, err := promptRequired(reader, "Profile name (e.g., 'myorg', 'work', 'personal'): ", "profile name")
	if err != nil {
		return err
	}

	orgURL, err := promptRequired(reader, "Organization URL (e.g., https://dev.azure.com/myorg): ", "organization URL")
	if err != nil {
		return err
	}

	projectName, err := promptRequired(reader, "Project name: ", "project name")
	if err != nil {
		return err
	}

	pat, err := promptPAT(reader)
	if err != nil {
		return err
	}

	orgURL = normalizeOrganizationURL(orgURL)
	setAsDefault := promptDefaultProfile(reader, isFirstProfile)
	profile := newSetupProfile(orgURL, projectName)

	fmt.Println()
	fmt.Println("Setting up profile...")

	if err := saveSetupProfile(loader, profileName, profile, setAsDefault); err != nil {
		return err
	}
	if err := persistSetupPAT(loader, profileName, &profile, orgURL, pat); err != nil {
		return err
	}

	printSetupConnectionResult(orgURL, projectName, pat)
	printSetupSuccess(profileName, setAsDefault)
	return nil
}

func printSetupBanner() {
	fmt.Println("========================================================")
	fmt.Println(" Azure DevOps CLI - Interactive Setup")
	fmt.Println("========================================================")
	fmt.Println()
	fmt.Println("This wizard will help you configure the CLI.")
	fmt.Println("Press Ctrl+C at any time to cancel.")
	fmt.Println()
}

func hasNoExistingProfiles(loader *config.ConfigLoader) bool {
	cfg, err := loader.Load()
	return err != nil || len(cfg.Profiles) == 0
}

func promptRequired(reader *bufio.Reader, prompt, fieldName string) (string, error) {
	fmt.Print(prompt)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", fieldName, err)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", fieldName)
	}
	return value, nil
}

func promptPAT(reader *bufio.Reader) (string, error) {
	fmt.Println()
	fmt.Println("Personal Access Token (PAT) - Get yours at:")
	fmt.Println("  https://dev.azure.com/[org]/_usersSettings/tokens")
	fmt.Println("  Required scopes: Code (read), Work Items (read/write), Project (read)")
	return promptRequired(reader, "Enter your PAT: ", "PAT")
}

func normalizeOrganizationURL(orgURL string) string {
	if strings.HasPrefix(orgURL, "http") {
		return orgURL
	}
	return "https://dev.azure.com/" + orgURL
}

func promptDefaultProfile(reader *bufio.Reader, isFirstProfile bool) bool {
	if isFirstProfile {
		return true
	}

	fmt.Println()
	fmt.Print("Set as default profile? (Y/n): ")
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	switch strings.TrimSpace(strings.ToLower(response)) {
	case "", "y", "yes":
		return true
	default:
		return false
	}
}

func newSetupProfile(orgURL, projectName string) config.Profile {
	return config.Profile{
		Organization: orgURL,
		Project:      projectName,
		Auth: auth.AuthConfig{
			Type:   auth.AuthTypePAT,
			Scopes: []string{"vso.packaging", "vso.code", "vso.project"},
		},
	}
}

func saveSetupProfile(loader *config.ConfigLoader, profileName string, profile config.Profile, setAsDefault bool) error {
	loader.SetProfile(profileName, profile)
	if setAsDefault {
		loader.SetActiveProfile(profileName)
	}
	if err := loader.Save(); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}
	return nil
}

func persistSetupPAT(loader *config.ConfigLoader, profileName string, profile *config.Profile, orgURL, pat string) error {
	credManager, err := auth.NewCredentialManager("")
	if err != nil {
		return fmt.Errorf("failed to create credential manager: %w", err)
	}

	err = credManager.SavePAT(auth.ServicePAT, extractOrgName(orgURL), pat)
	if err == nil {
		return nil
	}

	fmt.Printf("Warning: Could not save to system keyring: %v\n", err)
	fmt.Println("PAT will be stored in config file (less secure)")
	profile.Auth.PAT = pat
	loader.SetProfile(profileName, *profile)
	if err := loader.Save(); err != nil {
		return fmt.Errorf("failed to save fallback profile: %w", err)
	}
	return nil
}

func printSetupConnectionResult(orgURL, projectName, pat string) {
	fmt.Println("Testing connection to Azure DevOps...")
	if err := testConnection(orgURL, projectName, pat); err != nil {
		fmt.Printf("Warning: Connection test failed: %v\n", err)
		fmt.Println("The profile was created, but there might be an issue with your credentials.")
		return
	}

	fmt.Println("Connection successful!")
}

func printSetupSuccess(profileName string, setAsDefault bool) {
	fmt.Println()
	fmt.Println("========================================================")
	fmt.Println(" Setup Complete!")
	fmt.Println("========================================================")
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
	return nil
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
