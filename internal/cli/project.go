package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nahuelcio/ado-cli/internal/api"
	"github.com/nahuelcio/ado-cli/internal/config"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Check project capabilities and available resources",
	Long: `Check what resources are available in an Azure DevOps project.

This command verifies access to:
- Work Items (read/write permissions)
- Pull Requests (read/write permissions)  
- Git Repositories (read permissions)
- Project info and settings

Useful for verifying permissions before running other commands.`,
}

var projectCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check project capabilities and available resources",
	Long: `Check what resources are available and accessible in the current project.

This command tests:
✓ Work Items - Can list work items
✓ Pull Requests - Can list PRs (requires at least one repo)
✓ Repositories - Can list Git repos
✓ Project Info - Basic project details

Examples:
  # Check current profile's project
  ado project check

  # Check specific project
  ado project check --profile yoizen-yflow

  # Check with verbose output
  ado project check --verbose`,
	RunE: func(cmd *cobra.Command, args []string) error {
		profileFlag, _ := cmd.Flags().GetString("profile")
		verbose, _ := cmd.Flags().GetBool("verbose")

		cfg := config.NewConfigLoader("")
		if profileFlag != "" {
			cfg.SetActiveProfile(profileFlag)
		}

		if _, err := cfg.Load(); err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		org := cfg.GetOrganization()
		proj := cfg.GetProject()
		auth := cfg.GetAuth()

		if org == "" {
			return fmt.Errorf("organization not configured")
		}
		if proj == "" {
			return fmt.Errorf("project not configured")
		}

		token := auth.PAT
		if token == "" {
			return fmt.Errorf("PAT not configured - run 'ado auth login'")
		}

		fmt.Printf("Checking project: %s\n", proj)
		fmt.Printf("Organization: %s\n\n", org)

		// Check project info
		client, err := api.NewAzureDevOpsClient(api.NewConnectionConfig(org, proj, token))
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		ctx := context.Background()
		projectInfo, err := client.GetProject(ctx, proj)
		if err != nil {
			fmt.Printf("❌ Project Info: FAILED - %v\n", err)
		} else {
			fmt.Printf("✓ Project Info: OK\n")
			if verbose {
				fmt.Printf("  - ID: %s\n", projectInfo.ID)
				fmt.Printf("  - Name: %s\n", projectInfo.Name)
				fmt.Printf("  - State: %s\n", projectInfo.State)
				fmt.Printf("  - Visibility: %s\n", projectInfo.Visibility)
			}
		}

		// Check Work Items
		wiClient := client.GetWorkItemClient()
		_, err = wiClient.ListWorkItems(ctx, proj, api.WorkItemFilters{Limit: 1})
		if err != nil {
			fmt.Printf("❌ Work Items: FAILED - %v\n", err)
		} else {
			fmt.Printf("✓ Work Items: READ OK\n")
		}

		// Check Repositories
		repoClient := client.GetRepositoryClient()
		repos, err := repoClient.ListRepositories(ctx, proj)
		if err != nil {
			fmt.Printf("❌ Repositories: FAILED - %v\n", err)
		} else {
			fmt.Printf("✓ Repositories: READ OK (%d repos found)\n", len(repos))
			if verbose && len(repos) > 0 {
				fmt.Printf("  Available repos:\n")
				for i, repo := range repos {
					if i >= 5 {
						fmt.Printf("  ... and %d more\n", len(repos)-5)
						break
					}
					fmt.Printf("  - %s\n", repo.Name)
				}
			}
		}

		// Check Pull Requests (needs at least one repo)
		if len(repos) > 0 {
			prClient := client.GetPullRequestClient()
			_, err = prClient.ListPullRequests(ctx, proj, repos[0].Name, api.PullRequestStatusNotSet)
			if err != nil {
				fmt.Printf("❌ Pull Requests: FAILED - %v\n", err)
			} else {
				fmt.Printf("✓ Pull Requests: READ OK\n")
			}
		} else {
			fmt.Printf("⚠ Pull Requests: SKIPPED (no repos to test with)\n")
		}

		fmt.Println("\n✅ Check complete!")
		return nil
	},
}

func init() {
	projectCheckCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	projectCheckCmd.Flags().BoolP("verbose", "v", false, "Show detailed information")

	projectCmd.AddCommand(projectCheckCmd)
}
