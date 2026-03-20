package cli

import (
	"fmt"
	"strings"

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
  add               Create a new profile with org URL and project
  list              Show all configured profiles
  show              Display details for a specific profile
  use               Set a profile as default (active)
  delete            Remove a profile
  set-permissions   Set the scopes/permissions for a profile
  show-permissions  Show the scopes for a profile

Examples:
  # Add a new profile
  ado profile add --name work --org https://dev.azure.com/mycompany --project myproject --default

  # List all profiles
  ado profile list

  # Set active profile
  ado profile use --name work

  # Show profile details
  ado profile show --name work

  # Set profile permissions (scopes)
  ado profile set-permissions --name yoizen-yflow --scopes repos,prs
  ado profile set-permissions --name yoizen-ysocial --scopes workitems,repos,prs

  # Show profile permissions
  ado profile show-permissions --name yoizen-yflow`,
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

var profileSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Check and sync PAT across profiles with same organization",
	Long: `Check which profiles share the same organization and PAT authentication.

This command shows:
- Profiles grouped by organization
- Which profiles will share the same PAT (auto-sync)
- PAT status for each organization

When multiple profiles use the same organization URL, they automatically
share the same PAT stored in the system keyring. No need to login multiple times.

Examples:
  # Check PAT sync status across all profiles
  ado profile sync

  # Sync PAT from one profile to others in same org
  ado profile sync --from yoizen --to yoizen-yflow`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fromProfile, _ := cmd.Flags().GetString("from")
		toProfile, _ := cmd.Flags().GetString("to")

		loader := config.NewConfigLoader("")
		cfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Group profiles by organization
		orgGroups := make(map[string][]string)
		profileDetails := make(map[string]config.Profile)

		for name, profile := range cfg.Profiles {
			orgKey := profile.Organization
			if orgKey == "" {
				orgKey = "(no organization)"
			}
			orgGroups[orgKey] = append(orgGroups[orgKey], name)
			profileDetails[name] = profile
		}

		fmt.Println("Profile Synchronization Status")
		fmt.Println("================================")
		fmt.Println()

		// Check if specific sync requested
		if fromProfile != "" && toProfile != "" {
			fromP, fromOk := profileDetails[fromProfile]
			toP, toOk := profileDetails[toProfile]

			if !fromOk {
				return fmt.Errorf("profile '%s' not found", fromProfile)
			}
			if !toOk {
				return fmt.Errorf("profile '%s' not found", toProfile)
			}

			if fromP.Organization != toP.Organization {
				return fmt.Errorf("profiles '%s' and '%s' have different organizations\n  %s: %s\n  %s: %s",
					fromProfile, toProfile, fromProfile, fromP.Organization, toProfile, toP.Organization)
			}

			fmt.Printf("✓ Profiles '%s' and '%s' share organization:\n", fromProfile, toProfile)
			fmt.Printf("  Organization: %s\n", fromP.Organization)
			fmt.Printf("  Project %s: %s\n", fromProfile, fromP.Project)
			fmt.Printf("  Project %s: %s\n", toProfile, toP.Project)
			fmt.Println()
			fmt.Println("These profiles automatically share the same PAT.")
			fmt.Println("No manual sync needed - PAT is retrieved by organization.")
			return nil
		}

		// Show all organization groups
		fmt.Printf("Found %d organization(s) with %d profile(s):\n\n", len(orgGroups), len(cfg.Profiles))

		for org, profiles := range orgGroups {
			fmt.Printf("📁 Organization: %s\n", org)
			fmt.Printf("   Profiles sharing PAT (%d):\n", len(profiles))

			for _, name := range profiles {
				p := profileDetails[name]
				marker := "  "
				if name == cfg.ActiveProfile {
					marker = "* "
				}
				fmt.Printf("   %s%s (project: %s)\n", marker, name, p.Project)
			}
			fmt.Println()
		}

		fmt.Println("💡 How it works:")
		fmt.Println("   - PAT is stored by organization in the system keyring")
		fmt.Println("   - All profiles with the same org automatically share the PAT")
		fmt.Println("   - No need to login multiple times for different projects!")
		fmt.Println()

		if len(orgGroups) > 1 {
			fmt.Printf("⚠ You have profiles in %d different organizations.\n", len(orgGroups))
			fmt.Println("   Each organization requires its own PAT.")
		}

		return nil
	},
}

var availableScopes = []string{"workitems", "repos", "prs"}

var profileSetPermissionsCmd = &cobra.Command{
	Use:   "set-permissions",
	Short: "Set the scopes/permissions for a profile",
	Long: `Set which scopes (workitems, repos, prs) a profile has access to.

Available scopes:
  workitems - Access to work item commands
  repos     - Access to repository commands
  prs       - Access to pull request commands

Examples:
  # Set profile to have only repos and prs scopes
  ado profile set-permissions --name yoizen-yflow --scopes repos,prs

  # Set profile to have all scopes
  ado profile set-permissions --name yoizen-ysocial --scopes workitems,repos,prs`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		scopesStr, _ := cmd.Flags().GetString("scopes")

		if name == "" {
			return fmt.Errorf("profile name is required (--name)")
		}
		if scopesStr == "" {
			return fmt.Errorf("scopes are required (--scopes)")
		}

		requestedScopes := strings.Split(scopesStr, ",")
		var validScopes []string
		for _, s := range requestedScopes {
			s = strings.TrimSpace(s)
			valid := false
			for _, allowed := range availableScopes {
				if s == allowed {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("invalid scope '%s'. Valid scopes are: %s", s, strings.Join(availableScopes, ", "))
			}
			validScopes = append(validScopes, s)
		}

		loader := config.NewConfigLoader("")
		cfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		profile, ok := cfg.Profiles[name]
		if !ok {
			return fmt.Errorf("profile '%s' does not exist", name)
		}

		profile.Scopes = validScopes
		loader.SetProfile(name, profile)

		if err := loader.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Profile '%s' scopes updated to: %s\n", name, strings.Join(validScopes, ", "))
		return nil
	},
}

var profileShowPermissionsCmd = &cobra.Command{
	Use:   "show-permissions",
	Short: "Show the scopes for a profile",
	Long: `Show which scopes (workitems, repos, prs) a profile has access to.

Examples:
  # Show permissions for a profile
  ado profile show-permissions --name yoizen-yflow`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")

		if name == "" {
			return fmt.Errorf("profile name is required (--name)")
		}

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
		if len(profile.Scopes) == 0 {
			fmt.Println("Scopes: (none)")
		} else {
			fmt.Printf("Scopes: %s\n", strings.Join(profile.Scopes, ", "))
		}

		fmt.Println("\nAllowed commands:")
		for _, scope := range availableScopes {
			allowed := "❌"
			if profile.HasScope(scope) {
				allowed = "✓"
			}
			fmt.Printf("  %s %s\n", allowed, scope)
		}

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

	profileSyncCmd.Flags().String("from", "", "Source profile for sync check")
	profileSyncCmd.Flags().String("to", "", "Target profile for sync check")

	profileSetPermissionsCmd.Flags().String("name", "", "Profile name")
	profileSetPermissionsCmd.Flags().String("scopes", "", "Comma-separated list of scopes (workitems, repos, prs)")
	_ = profileSetPermissionsCmd.MarkFlagRequired("name")
	_ = profileSetPermissionsCmd.MarkFlagRequired("scopes")

	profileShowPermissionsCmd.Flags().String("name", "", "Profile name")
	_ = profileShowPermissionsCmd.MarkFlagRequired("name")

	profileCmd.AddCommand(profileAddCmd)
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileDeleteCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileSyncCmd)
	profileCmd.AddCommand(profileSetPermissionsCmd)
	profileCmd.AddCommand(profileShowPermissionsCmd)
}
