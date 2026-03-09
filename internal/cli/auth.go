package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/your-org/azure-devops-cli/internal/auth"
	"github.com/your-org/azure-devops-cli/internal/config"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long:  `Commands to manage Azure DevOps authentication (login, logout, test).`,
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
			fmt.Scanln(&input)
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
			fmt.Println("No credentials found. Please run 'ado auth login' first.")
			os.Exit(1)
		}

		fmt.Printf("Connection test successful for profile '%s'\n", profileName)
		fmt.Printf("Organization: %s\n", profile.Organization)
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

	rootCmd.AddCommand(authCmd)
}
