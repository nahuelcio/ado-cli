package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nahuelcio/ado-cli/internal/auth"
	"github.com/nahuelcio/ado-cli/internal/config"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication and credentials",
	Long: `Manage authentication with Azure DevOps.

This command group handles authentication using Personal Access Tokens (PAT).
Tokens are securely stored in the system keyring (or fallback to encrypted files).

Workflow:
  1. Login:    ado auth login --profile myorg
  2. Test:     ado auth test --profile myorg  
  3. Logout:   ado auth logout --profile myorg

Getting a PAT:
  Visit: https://dev.azure.com/[org]/_usersSettings/tokens
  Required scopes: Code (read), Work Items (read/write), Project (read)

Examples:
  ado auth login --profile myorg                    # Interactive login
  ado auth login --profile myorg --pat <token>      # Non-interactive login
  ado auth test --profile myorg                     # Verify connection
  ado auth logout --profile myorg                   # Remove credentials`,
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to Azure DevOps",
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName, _ := cmd.Flags().GetString("profile")
		pat, _ := cmd.Flags().GetString("pat")

		loader := config.NewConfigLoader("")

		if _, err := loader.Load(); err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if profileName == "" {
			profileName = loader.GetActiveProfileName()
		}

		profile, ok := loader.GetConfig().Profiles[profileName]
		if !ok {
			return fmt.Errorf("profile '%s' does not exist", profileName)
		}

		if pat == "" {
			fmt.Print("Enter your PAT: ")
			var input string
			if _, err := fmt.Scanln(&input); err != nil {
				return fmt.Errorf("failed to read PAT: %w", err)
			}
			pat = input
		}

		if pat == "" {
			return fmt.Errorf("PAT is required")
		}

		credManager, err := auth.NewCredentialManager("")
		if err != nil {
			return fmt.Errorf("failed to create credential manager: %w", err)
		}

		err = credManager.SavePAT(auth.ServicePAT, profile.Organization, pat)
		if err != nil {
			return fmt.Errorf("failed to save PAT: %w", err)
		}

		profile.Auth.PAT = "***"
		profile.Auth.Type = auth.AuthTypePAT
		loader.SetProfile(profileName, profile)

		if err := loader.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Successfully logged in to Azure DevOps (profile: %s)\n", profileName)
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from Azure DevOps",
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName, _ := cmd.Flags().GetString("profile")

		loader := config.NewConfigLoader("")

		cfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if profileName == "" {
			profileName = cfg.ActiveProfile
		}

		profile, ok := cfg.Profiles[profileName]
		if !ok {
			return fmt.Errorf("profile '%s' does not exist", profileName)
		}

		credManager, err := auth.NewCredentialManager("")
		if err != nil {
			return fmt.Errorf("failed to create credential manager: %w", err)
		}

		err = credManager.DeletePAT(auth.ServicePAT, profile.Organization)
		if err != nil {
			return fmt.Errorf("failed to delete PAT: %w", err)
		}

		fmt.Printf("Successfully logged out from Azure DevOps (profile: %s)\n", profileName)
		return nil
	},
}

var authTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test connection to Azure DevOps",
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName, _ := cmd.Flags().GetString("profile")

		loader := config.NewConfigLoader("")

		cfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if profileName == "" {
			profileName = cfg.ActiveProfile
		}

		profile, ok := cfg.Profiles[profileName]
		if !ok {
			return fmt.Errorf("profile '%s' does not exist", profileName)
		}

		credManager, err := auth.NewCredentialManager("")
		if err != nil {
			return fmt.Errorf("failed to create credential manager: %w", err)
		}

		pat, err := credManager.GetPAT(auth.ServicePAT, profile.Organization)
		if err != nil {
			return fmt.Errorf("failed to get PAT: %w", err)
		}

		if pat == "" {
			if profile.Auth.PAT != "" && profile.Auth.PAT != "***" {
				fmt.Printf("Connection test successful for profile '%s'\n", profileName)
				fmt.Printf("Organization: %s\n", profile.Organization)
				fmt.Println("Credential backend: config")
				return nil
			} else {
				return fmt.Errorf("no credentials found for profile %q. Run 'ado auth login --profile %s' or set AZURE_DEVOPS_PAT", profileName, profileName)
			}
		}

		fmt.Printf("Connection test successful for profile '%s'\n", profileName)
		fmt.Printf("Organization: %s\n", profile.Organization)
		fmt.Printf("Credential backend: %s\n", credManager.GetBackend())
		return nil
	},
}

func init() {
	authLoginCmd.Flags().String("profile", "", "Profile name to use")
	authLoginCmd.Flags().String("pat", "", "Personal Access Token")

	authLogoutCmd.Flags().String("profile", "", "Profile name to use")

	authTestCmd.Flags().String("profile", "", "Profile name to use")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authTestCmd)
}
