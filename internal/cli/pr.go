package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/nahuelcio/ado-cli/internal/api"
	"github.com/nahuelcio/ado-cli/internal/config"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	prFormat    OutputFormat
	prProfile   string
	prRepo      string
)

func getPRClient(cmd *cobra.Command) (api.PullRequestClient, *config.ConfigLoader, error) {
	profileFlag, _ := cmd.Flags().GetString("profile")
	if profileFlag != "" {
		prProfile = profileFlag
	}

	repoFlag, _ := cmd.Flags().GetString("repo")
	if repoFlag != "" {
		prRepo = repoFlag
	}

	cfg := config.NewConfigLoader("")
	if prProfile != "" {
		cfg.SetActiveProfile(prProfile)
	}

	cfg.Load()

	org := cfg.GetOrganization()
	proj := cfg.GetProject()
	auth := cfg.GetAuth()

	if org == "" {
		return nil, nil, fmt.Errorf("organization not configured. Use --profile or set AZURE_DEVOPS_ORG")
	}
	if proj == "" {
		return nil, nil, fmt.Errorf("project not configured. Use --profile or set AZURE_DEVOPS_PROJECT")
	}
	if prRepo == "" {
		return nil, nil, fmt.Errorf("repository not configured. Use --repo flag or set AZURE_DEVOPS_REPO")
	}

	token := auth.PAT
	if token == "" {
		return nil, nil, fmt.Errorf("PAT not configured. Use --profile or set AZURE_DEVOPS_PAT")
	}

	client, err := api.GetPullRequestClient(context.Background(), org, proj, token)
	if err != nil {
		return nil, nil, err
	}

	return client, cfg, nil
}

func printPRTable(data interface{}) {
	prs, ok := data.([]api.PullRequest)
	if !ok {
		fmt.Printf("%+v\n", data)
		return
	}

	if len(prs) == 0 {
		fmt.Println("No pull requests found")
		return
	}

	fmt.Println("Pull Requests")
	fmt.Println()
	fmt.Printf("%-8s %-45s %-12s %-35s %-35s %-22s\n", "ID", "Title", "Status", "Source", "Target", "Author")
	fmt.Println(strings.Repeat("-", 157))

	for _, pr := range prs {
		source := ""
		if pr.SourceRefName != nil {
			source = *pr.SourceRefName
		}
		target := ""
		if pr.TargetRefName != nil {
			target = *pr.TargetRefName
		}
		author := ""
		if pr.CreatedBy != nil {
			author = pr.CreatedBy.DisplayName
		}

		fmt.Printf("%-8s %-45s %-12s %-35s %-35s %-22s\n",
			strconv.Itoa(pr.PullRequestID),
			truncateString(pr.Title, 43),
			string(pr.Status),
			truncateString(source, 33),
			truncateString(target, 33),
			truncateString(author, 20),
		)
	}
}

func printChangesTable(data interface{}) {
	changes, ok := data.([]api.GitChange)
	if !ok {
		fmt.Printf("%+v\n", data)
		return
	}

	if len(changes) == 0 {
		fmt.Println("No changes found")
		return
	}

	fmt.Println("Pull Request Changes")
	fmt.Println()
	fmt.Printf("%-60s %-15s\n", "Path", "Change Type")
	fmt.Println(strings.Repeat("-", 75))

	for _, change := range changes {
		fmt.Printf("%-60s %-15s\n", change.Path, string(change.ChangeType))
	}
}

func printThreadsTable(data interface{}) {
	threads, ok := data.([]api.PullRequestThreadSummary)
	if !ok {
		fmt.Printf("%+v\n", data)
		return
	}

	if len(threads) == 0 {
		fmt.Println("No threads found")
		return
	}

	fmt.Println("Pull Request Threads")
	fmt.Println()
	fmt.Printf("%-8s %-12s %-10s %-35s %-42s\n", "ID", "Status", "Comments", "File", "First Comment")
	fmt.Println(strings.Repeat("-", 107))

	for _, thread := range threads {
		filePath := ""
		if thread.ThreadContext != nil && thread.ThreadContext.FilePath != nil {
			filePath = *thread.ThreadContext.FilePath
		}
		firstComment := ""
		if thread.FirstComment != nil {
			firstComment = truncateString(thread.FirstComment.Content, 40)
		}

		fmt.Printf("%-8s %-12s %-10s %-35s %-42s\n",
			strconv.Itoa(thread.ID),
			string(thread.Status),
			strconv.Itoa(thread.CommentCount),
			truncateString(filePath, 33),
			firstComment,
		)
	}
}

func printSummaryTable(data interface{}) {
	summary, ok := data.(*api.PRSummary)
	if !ok {
		fmt.Printf("%+v\n", data)
		return
	}

	fmt.Printf("Pull Request #%d - Summary\n", summary.PullRequestID)
	fmt.Println()
	fmt.Printf("%-20s %s\n", "Title:", summary.Title)
	fmt.Printf("%-20s %s\n", "Status:", summary.Status)
	fmt.Printf("%-20s %s\n", "Source Branch:", summary.SourceBranch)
	fmt.Printf("%-20s %s\n", "Target Branch:", summary.TargetBranch)
	fmt.Printf("%-20s %d\n", "Total Files Changed:", summary.TotalChanges)
	fmt.Println()

	if len(summary.Files) > 0 {
		fmt.Println("Changed Files")
		fmt.Println()
		fmt.Printf("%-60s %-15s\n", "Path", "Change Type")
		fmt.Println(strings.Repeat("-", 75))

		for _, file := range summary.Files {
			fmt.Printf("%-60s %-15s\n", file.Path, file.ChangeType)
		}
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Manage Azure DevOps pull requests (list, review, view changes)",
	Long: `Manage pull requests in Azure DevOps.

This command group provides comprehensive PR management including:
- Listing pull requests with status filters (active, completed, abandoned)
- Viewing detailed PR information and metadata
- Examining code changes and file diffs
- Reading and creating review threads/comments
- Getting PR summaries for quick overview

Examples:
  # List active pull requests
  ado pr list --repo myrepo --status active

  # Show PR details
  ado pr show --repo myrepo --pr-id 456

  # View all changes in a PR
  ado pr changes --repo myrepo --pr-id 456

  # View all discussion threads
  ado pr threads --repo myrepo --pr-id 456

  # Get a quick summary
  ado pr summary --repo myrepo --pr-id 456

  # Add a review comment
  ado pr review --repo myrepo --pr-id 456 --comment "LGTM!" --status approved`,
}

var prListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pull requests",
	Long:  `List pull requests with optional filters.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := getPRClient(cmd)
		if err != nil {
			return err
		}

		statusStr, _ := cmd.Flags().GetString("status")
		var status api.PullRequestStatus
		switch statusStr {
		case "active":
			status = api.PullRequestStatusActive
		case "completed":
			status = api.PullRequestStatusCompleted
		case "abandoned":
			status = api.PullRequestStatusAbandoned
		case "all", "":
			status = api.PullRequestStatusNotSet
		default:
			return fmt.Errorf("invalid status: %s", statusStr)
		}

		prs, err := client.ListPullRequests(context.Background(), "", prRepo, status)
		if err != nil {
			return fmt.Errorf("failed to list pull requests: %w", err)
		}

		if prFormat == FormatTable {
			printPRTable(prs)
			return nil
		}
		return printOutput(prs, prFormat)
	},
}

var prShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show a pull request",
	Long:  `Show details of a specific pull request.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := getPRClient(cmd)
		if err != nil {
			return err
		}

		prID, _ := cmd.Flags().GetInt("pr-id")
		if prID == 0 {
			return fmt.Errorf("pull request ID is required (--pr-id)")
		}

		pr, err := client.GetPullRequest(context.Background(), "", prRepo, prID)
		if err != nil {
			return fmt.Errorf("failed to get pull request: %w", err)
		}

		if prFormat == FormatTable {
			return printOutput(pr, FormatJSON)
		}
		return printOutput(pr, prFormat)
	},
}

var prChangesCmd = &cobra.Command{
	Use:   "changes",
	Short: "Show pull request changes",
	Long:  `Show changes in a specific pull request.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := getPRClient(cmd)
		if err != nil {
			return err
		}

		prID, _ := cmd.Flags().GetInt("pr-id")
		if prID == 0 {
			return fmt.Errorf("pull request ID is required (--pr-id)")
		}

		changes, err := client.GetPullRequestChanges(context.Background(), "", prRepo, prID)
		if err != nil {
			return fmt.Errorf("failed to get pull request changes: %w", err)
		}

		if prFormat == FormatTable {
			printChangesTable(changes)
			return nil
		}
		return printOutput(changes, prFormat)
	},
}

var prThreadsCmd = &cobra.Command{
	Use:   "threads",
	Short: "Show pull request threads",
	Long:  `Show comment threads of a specific pull request.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := getPRClient(cmd)
		if err != nil {
			return err
		}

		prID, _ := cmd.Flags().GetInt("pr-id")
		if prID == 0 {
			return fmt.Errorf("pull request ID is required (--pr-id)")
		}

		threads, err := client.GetThreads(context.Background(), "", prRepo, prID)
		if err != nil {
			return fmt.Errorf("failed to get pull request threads: %w", err)
		}

		if prFormat == FormatTable {
			printThreadsTable(threads)
			return nil
		}
		return printOutput(threads, prFormat)
	},
}

var prSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show pull request summary",
	Long:  `Show a summary of a specific pull request.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := getPRClient(cmd)
		if err != nil {
			return err
		}

		prID, _ := cmd.Flags().GetInt("pr-id")
		if prID == 0 {
			return fmt.Errorf("pull request ID is required (--pr-id)")
		}

		summary, err := client.GetPullRequestSummary(context.Background(), "", prRepo, prID)
		if err != nil {
			return fmt.Errorf("failed to get pull request summary: %w", err)
		}

		printSummaryTable(summary)
		return nil
	},
}

var prReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Create a pull request review",
	Long:  `Create a review or add a comment to a pull request.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := getPRClient(cmd)
		if err != nil {
			return err
		}

		prID, _ := cmd.Flags().GetInt("pr-id")
		if prID == 0 {
			return fmt.Errorf("pull request ID is required (--pr-id)")
		}

		comment, _ := cmd.Flags().GetString("comment")
		status, _ := cmd.Flags().GetString("status")

		if comment == "" && status == "" {
			return fmt.Errorf("either --comment or --status is required")
		}

		threadStatus := api.ThreadStatusPending
		if status != "" {
			switch status {
			case "approved":
				threadStatus = api.ThreadStatusClosed
			case "rejected":
				threadStatus = api.ThreadStatusWontFix
			case "waiting":
				threadStatus = api.ThreadStatusPending
			default:
				return fmt.Errorf("invalid status: %s (use approved/rejected/waiting)", status)
			}
		}

		commentType := api.CommentTypeText
		thread := &api.PullRequestThread{
			Comments: []api.ThreadComment{
				{
					Content:    comment,
					CommentType: &commentType,
				},
			},
			Status: &threadStatus,
		}

		result, err := client.CreateThread(context.Background(), "", prRepo, prID, thread)
		if err != nil {
			return fmt.Errorf("failed to create review: %w", err)
		}

		return printOutput(result, prFormat)
	},
}

func init() {
	prListCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	prListCmd.Flags().String("repo", "", "Repository name (required)")
	prListCmd.Flags().String("status", "active", "Filter by status (active/completed/abandoned/all)")
	prListCmd.Flags().VarP(&prFormat, "format", "f", "Output format (table/json/yaml)")

	prShowCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	prShowCmd.Flags().String("repo", "", "Repository name (required)")
	prShowCmd.Flags().Int("pr-id", 0, "Pull request ID (required)")
	prShowCmd.Flags().VarP(&prFormat, "format", "f", "Output format (table/json/yaml)")

	prChangesCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	prChangesCmd.Flags().String("repo", "", "Repository name (required)")
	prChangesCmd.Flags().Int("pr-id", 0, "Pull request ID (required)")
	prChangesCmd.Flags().VarP(&prFormat, "format", "f", "Output format (table/json/yaml)")

	prThreadsCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	prThreadsCmd.Flags().String("repo", "", "Repository name (required)")
	prThreadsCmd.Flags().Int("pr-id", 0, "Pull request ID (required)")
	prThreadsCmd.Flags().VarP(&prFormat, "format", "f", "Output format (table/json/yaml)")

	prSummaryCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	prSummaryCmd.Flags().String("repo", "", "Repository name (required)")
	prSummaryCmd.Flags().Int("pr-id", 0, "Pull request ID (required)")

	prReviewCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	prReviewCmd.Flags().String("repo", "", "Repository name (required)")
	prReviewCmd.Flags().Int("pr-id", 0, "Pull request ID (required)")
	prReviewCmd.Flags().String("comment", "", "Comment text")
	prReviewCmd.Flags().String("status", "", "Review status (approved/rejected/waiting)")
	prReviewCmd.Flags().VarP(&prFormat, "format", "f", "Output format (table/json/yaml)")

	prCmd.AddCommand(prListCmd)
	prCmd.AddCommand(prShowCmd)
	prCmd.AddCommand(prChangesCmd)
	prCmd.AddCommand(prThreadsCmd)
	prCmd.AddCommand(prSummaryCmd)
	prCmd.AddCommand(prReviewCmd)

	rootCmd.AddCommand(prCmd)
}

func init() {
	if prFormat == "" {
		prFormat = FormatTable
	}

	_ = os.Stderr
}
