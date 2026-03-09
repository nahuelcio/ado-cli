package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nahuelcio/ado-cli/internal/auth"
	"github.com/nahuelcio/ado-cli/internal/config"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage Azure DevOps profiles and configurations",
	Long: `Manage profiles for different Azure DevOps organizations.

Profiles store organization URLs, project names, and authentication settings.
This allows you to work with multiple Azure DevOps instances seamlessly.

Commands:
  add     Create a new profile with org URL and project
  list    Show all configured profiles
  show    Display details for a specific profile
  use     Set a profile as default (active)
  delete  Remove a profile

Examples:
  # Add a new profile
  ado profile add --name work --org https://dev.azure.com/mycompany --project myproject --default

  # List all profiles
  ado profile list

  # Set active profile
  ado profile use --name work

  # Show profile details
  ado profile show --name work`,
}

var profileAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		org, _ := cmd.Flags().GetString("org")
		project, _ := cmd.Flags().GetString("project")
		isDefault, _ := cmd.Flags().GetBool("default")

		loader := config.NewConfigLoader("")

		if _, err := loader.Load(); err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		profile := config.Profile{
			Organization: org,
			Project:      project,
			Auth: auth.AuthConfig{
				Type:   auth.AuthTypePAT,
				Scopes: []string{"vso.packaging", "vso.code", "vso.project"},
			},
		}

		loader.SetProfile(name, profile)

		if isDefault {
			loader.SetActiveProfile(name)
		}

		if err := loader.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Profile '%s' added successfully\n", name)
		if isDefault {
			fmt.Printf("Profile '%s' set as default\n", name)
		}
		return nil
	},
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		loader := config.NewConfigLoader("")

		cfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		activeProfile := cfg.ActiveProfile

		fmt.Println("Profiles:")
		for name, profile := range cfg.Profiles {
			marker := " "
			if name == activeProfile {
				marker = "*"
			}
			fmt.Printf("  %s %s (org: %s, project: %s)\n", marker, name, profile.Organization, profile.Project)
		}
		return nil
	},
}

var profileDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")

		loader := config.NewConfigLoader("")

		cfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if _, ok := cfg.Profiles[name]; !ok {
			return fmt.Errorf("profile '%s' does not exist", name)
		}

		delete(cfg.Profiles, name)

		if cfg.ActiveProfile == name {
			if len(cfg.Profiles) > 0 {
				for newActive := range cfg.Profiles {
					cfg.ActiveProfile = newActive
					break
				}
			} else {
				cfg.ActiveProfile = ""
			}
		}

		loader.Set("profiles", cfg.Profiles)
		loader.SetActiveProfile(cfg.ActiveProfile)

		if err := loader.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Profile '%s' deleted successfully\n", name)
		return nil
	},
}

var profileShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show profile details",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")

		loader := config.NewConfigLoader("")

		cfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		profile, ok := cfg.Profiles[name]
		if !ok {
			return fmt.Errorf("profile '%s' does not exist", name)
		}

		fmt.Printf("Profile: %s\n", name)
		if name == cfg.ActiveProfile {
			fmt.Println("Status: Active (default)")
		}
		fmt.Printf("Organization: %s\n", profile.Organization)
		fmt.Printf("Project: %s\n", profile.Project)
		fmt.Printf("Auth Type: %s\n", profile.Auth.Type)
		return nil
	},
}

var profileUseCmd = &cobra.Command{
	Use:   "use",
	Short: "Set a profile as default",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")

		loader := config.NewConfigLoader("")

		cfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if _, ok := cfg.Profiles[name]; !ok {
			return fmt.Errorf("profile '%s' does not exist", name)
		}

		loader.SetActiveProfile(name)

		if err := loader.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Profile '%s' set as default\n", name)
		return nil
	},
}

func init() {
	profileAddCmd.Flags().String("name", "", "Profile name")
	profileAddCmd.Flags().String("org", "", "Azure DevOps organization URL")
	profileAddCmd.Flags().String("project", "", "Azure DevOps project name")
	profileAddCmd.Flags().Bool("default", false, "Set as default profile")
	_ = profileAddCmd.MarkFlagRequired("name")
	_ = profileAddCmd.MarkFlagRequired("org")

	profileDeleteCmd.Flags().String("name", "", "Profile name to delete")
	_ = profileDeleteCmd.MarkFlagRequired("name")

	profileShowCmd.Flags().String("name", "", "Profile name to show")
	_ = profileShowCmd.MarkFlagRequired("name")

	profileUseCmd.Flags().String("name", "", "Profile name to use")
	_ = profileUseCmd.MarkFlagRequired("name")

	profileCmd.AddCommand(profileAddCmd)
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileDeleteCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileUseCmd)

	rootCmd.AddCommand(profileCmd)
}
