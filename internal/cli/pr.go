package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nahuelcio/ado-cli/internal/api"
	"github.com/nahuelcio/ado-cli/internal/config"

	"github.com/spf13/cobra"
)

var prFormat OutputFormat

func getPRClient(cmd *cobra.Command) (api.PullRequestClient, string, string, error) {
	cfg, authCfg, err := getConfigAndAuth(cmd)
	if err != nil {
		return nil, "", "", err
	}

	repo, err := getRepoFromFlags(cmd)
	if err != nil {
		return nil, "", "", err
	}

	_ = cfg

	client, err := api.GetPullRequestClient(context.Background(), authCfg.Org, authCfg.Project, authCfg.PAT)
	if err != nil {
		return nil, "", "", err
	}

	return client, authCfg.Project, repo, nil
}

func extractLLMPRData(pr *api.PullRequest) map[string]interface{} {
	result := map[string]interface{}{
		"id":     pr.PullRequestID,
		"title":  pr.Title,
		"status": string(pr.Status),
	}

	if pr.Description != nil && *pr.Description != "" {
		result["description"] = cleanHTML(*pr.Description)
	}

	if pr.SourceRefName != nil {
		result["source_branch"] = *pr.SourceRefName
	}
	if pr.TargetRefName != nil {
		result["target_branch"] = *pr.TargetRefName
	}
	if pr.CreatedBy != nil {
		result["author"] = pr.CreatedBy.DisplayName
	}
	if pr.MergeStatus != nil {
		result["merge_status"] = *pr.MergeStatus
	}
	if pr.IsDraft != nil && *pr.IsDraft {
		result["is_draft"] = true
	}

	if len(pr.Reviewers) > 0 {
		var reviewers []string
		for _, r := range pr.Reviewers {
			if r.VotedBy != nil {
				vote := ""
				switch r.Vote {
				case 10:
					vote = "✓"
				case -10:
					vote = "✗"
				case -5:
					vote = "⏳"
				}
				name := r.VotedBy.DisplayName
				if vote != "" {
					name = vote + " " + name
				}
				reviewers = append(reviewers, name)
			}
		}
		if len(reviewers) > 0 {
			result["reviewers"] = reviewers
		}
	}

	return result
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
	fmt.Printf("%-8s %-12s %-10s %-35s %-42s\n", "ID", "Status", "Comments", "File", "Comment")
	fmt.Println(strings.Repeat("-", 107))

	for _, thread := range threads {
		fmt.Printf("%-8s %-12s %-10s %-35s %-42s\n",
			strconv.Itoa(thread.ID),
			thread.Status,
			strconv.Itoa(thread.CommentCount),
			truncateString(thread.File, 33),
			truncateString(thread.Comment, 40),
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

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Manage pull requests",
	Long: `Pull requests: list, show, review, threads, diff.

Examples:
  ado pr list --repo myrepo
  ado pr show --repo myrepo --pr-id 456
  ado pr review --repo myrepo --pr-id 456 --status approved
  ado pr threads --repo myrepo --pr-id 456`,
}

var prListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pull requests",
	Long:  `List PRs. Filters: --status (active/completed/abandoned/all).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, project, repo, err := getPRClient(cmd)
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
		case "all":
			status = api.PullRequestStatusAll
		case "":
			status = api.PullRequestStatusActive
		default:
			return fmt.Errorf("invalid status: %s", statusStr)
		}

		prs, err := client.ListPullRequests(context.Background(), project, repo, status)
		if err != nil {
			return fmt.Errorf("failed to list pull requests: %w", err)
		}

		full, _ := cmd.Flags().GetBool("full")
		if prFormat == FormatTable {
			printPRTable(prs)
			return nil
		}

		if full {
			return printOutput(prs, prFormat)
		}

		var llmData []map[string]interface{}
		for _, pr := range prs {
			llmData = append(llmData, extractLLMPRData(&pr))
		}

		if prFormat == "" {
			prFormat = FormatYAML
		}
		return printOutput(llmData, prFormat)
	},
}

var prShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show a pull request",
	Long:  `Show details of a specific pull request.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, project, repo, err := getPRClient(cmd)
		if err != nil {
			return err
		}

		prID, _ := cmd.Flags().GetInt("pr-id")
		if prID == 0 {
			return fmt.Errorf("pull request ID is required (--pr-id)")
		}

		pr, err := client.GetPullRequest(context.Background(), project, repo, prID)
		if err != nil {
			return fmt.Errorf("failed to get pull request: %w", err)
		}

		full, _ := cmd.Flags().GetBool("full")
		if full {
			if prFormat == FormatTable {
				return printOutput(pr, FormatJSON)
			}
			return printOutput(pr, prFormat)
		}

		output := extractLLMPRData(pr)
		if prFormat == "" {
			prFormat = FormatYAML
		}
		return printOutput(output, prFormat)
	},
}

var prChangesCmd = &cobra.Command{
	Use:   "changes",
	Short: "Show pull request changes",
	Long:  `Show changes in a specific pull request.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, project, repo, err := getPRClient(cmd)
		if err != nil {
			return err
		}

		prID, _ := cmd.Flags().GetInt("pr-id")
		if prID == 0 {
			return fmt.Errorf("pull request ID is required (--pr-id)")
		}

		changes, err := client.GetPullRequestChanges(context.Background(), project, repo, prID)
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

var prDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show pull request diff with file metadata",
	Long: `Show the diff/changes of a pull request with file metadata for code review.

This command retrieves the list of changed files with metadata useful for LLM code review:
- File paths and change types (add/edit/delete/rename)
- Original path (for renames)
- Change statistics (additions/deletions)
- Binary file detection

Note: Azure DevOps API returns file metadata but not full diff content. 
For viewing actual code changes, use the web interface or fetch individual files.

Examples:
  # Show diff for a PR (all files)
  ado pr diff --repo myrepo --pr-id 123

  # Show diff limited to first 5 files
  ado pr diff --repo myrepo --pr-id 123 --max-files 5

  # Show diff in JSON format
  ado pr diff --repo myrepo --pr-id 123 --format json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, project, repo, err := getPRClient(cmd)
		if err != nil {
			return err
		}

		prID, _ := cmd.Flags().GetInt("pr-id")
		if prID == 0 {
			return fmt.Errorf("pull request ID is required (--pr-id)")
		}

		maxFiles, _ := cmd.Flags().GetInt("max-files")

		diff, err := client.GetPullRequestDiff(context.Background(), project, repo, prID, maxFiles)
		if err != nil {
			return fmt.Errorf("failed to get pull request diff: %w", err)
		}

		return printOutput(diff, prFormat)
	},
}

var prThreadsCmd = &cobra.Command{
	Use:   "threads",
	Short: "Show pull request threads",
	Long:  `Show comment threads of a specific pull request.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, project, repo, err := getPRClient(cmd)
		if err != nil {
			return err
		}

		prID, _ := cmd.Flags().GetInt("pr-id")
		if prID == 0 {
			return fmt.Errorf("pull request ID is required (--pr-id)")
		}

		threads, err := client.GetThreads(context.Background(), project, repo, prID)
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
		client, project, repo, err := getPRClient(cmd)
		if err != nil {
			return err
		}

		prID, _ := cmd.Flags().GetInt("pr-id")
		if prID == 0 {
			return fmt.Errorf("pull request ID is required (--pr-id)")
		}

		summary, err := client.GetPullRequestSummary(context.Background(), project, repo, prID)
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
	Long: `Create a review, add a comment, or vote on a pull request.

Use --status to vote (approved/rejected/waiting)
Use --comment to add a review comment
Use both to vote AND comment

Examples:
  # Approve a PR
  ado pr review --repo myrepo --pr-id 123 --status approved

  # Reject a PR
  ado pr review --repo myrepo --pr-id 123 --status rejected

  # Add a comment
  ado pr review --repo myrepo --pr-id 123 --comment "LGTM!"

  # Vote and comment
  ado pr review --repo myrepo --pr-id 123 --status approved --comment "Nice work!"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, project, repo, err := getPRClient(cmd)
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

		if status != "" {
			var vote int
			switch status {
			case "approved":
				vote = 10
			case "rejected":
				vote = -10
			case "waiting":
				vote = -5
			case "noVote", "none":
				vote = 0
			default:
				return fmt.Errorf("invalid status: %s (use approved/rejected/waiting/noVote)", status)
			}

			err = client.VoteReviewer(context.Background(), project, repo, prID, "@me", vote)
			if err != nil {
				return fmt.Errorf("failed to vote on pull request: %w", err)
			}
		}

		if comment != "" {
			threadStatus := api.ThreadStatusPending
			thread := &api.PullRequestThread{
				Comments: []api.ThreadComment{
					{
						Content:     comment,
						CommentType: func() *api.CommentType { t := api.CommentTypeText; return &t }(),
					},
				},
				Status: &threadStatus,
			}

			result, err := client.CreateThread(context.Background(), project, repo, prID, thread)
			if err != nil {
				return fmt.Errorf("failed to create review comment: %w", err)
			}

			return printOutput(result, prFormat)
		}

		fmt.Println("Vote submitted successfully")
		return nil
	},
}

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage Azure DevOps Git repositories",
	Long:  `List and view Git repositories in your Azure DevOps project.`,
}

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Git repositories",
	Long:  `List all Git repositories in the current project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		profileFlag, _ := cmd.Flags().GetString("profile")

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
			return fmt.Errorf("organization not configured. Use --profile or set AZURE_DEVOPS_ORG")
		}
		if proj == "" {
			return fmt.Errorf("project not configured. Use --profile or set AZURE_DEVOPS_PROJECT")
		}

		token := auth.PAT
		if token == "" {
			return fmt.Errorf("PAT not configured. Use --profile or set AZURE_DEVOPS_PAT")
		}

		client, err := api.GetRepositoryClient(context.Background(), org, proj, token)
		if err != nil {
			return err
		}

		repos, err := client.ListRepositories(context.Background(), proj)
		if err != nil {
			return fmt.Errorf("failed to list repositories: %w", err)
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "" {
			format = "yaml"
		}

		// LLM-optimized output
		var llmData []map[string]interface{}
		for _, repo := range repos {
			item := map[string]interface{}{
				"name": repo.Name,
				"id":   repo.ID,
			}
			if repo.DefaultBranch != "" {
				item["default_branch"] = repo.DefaultBranch
			}
			if repo.RemoteURL != "" {
				item["url"] = repo.RemoteURL
			}
			if repo.Size > 0 {
				item["size_mb"] = repo.Size / 1024 / 1024
			}
			llmData = append(llmData, item)
		}

		outputFormat := OutputFormat(format)
		return printOutput(llmData, outputFormat)
	},
}

func init() {
	prListCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	prListCmd.Flags().String("repo", "", "Repository name (required)")
	prListCmd.Flags().String("status", "active", "Filter by status (active/completed/abandoned/all)")
	prListCmd.Flags().Bool("full", false, "Show all fields (default: LLM-optimized view)")
	prListCmd.Flags().VarP(&prFormat, "format", "f", "Output format (table/json/yaml) - default: yaml")

	prShowCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	prShowCmd.Flags().String("repo", "", "Repository name (required)")
	prShowCmd.Flags().Int("pr-id", 0, "Pull request ID (required)")
	prShowCmd.Flags().Bool("full", false, "Show all fields (default: LLM-optimized view)")
	prShowCmd.Flags().VarP(&prFormat, "format", "f", "Output format (table/json/yaml) - default: yaml")

	prChangesCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	prChangesCmd.Flags().String("repo", "", "Repository name (required)")
	prChangesCmd.Flags().Int("pr-id", 0, "Pull request ID (required)")
	prChangesCmd.Flags().VarP(&prFormat, "format", "f", "Output format (table/json/yaml)")

	prDiffCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	prDiffCmd.Flags().String("repo", "", "Repository name (required)")
	prDiffCmd.Flags().Int("pr-id", 0, "Pull request ID (required)")
	prDiffCmd.Flags().Int("max-files", 0, "Maximum number of files to show (0 = all)")
	prDiffCmd.Flags().VarP(&prFormat, "format", "f", "Output format (yaml/json)")

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

	repoListCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	repoListCmd.Flags().StringP("format", "f", "yaml", "Output format (yaml/json)")

	prCmd.AddCommand(prListCmd)
	prCmd.AddCommand(prShowCmd)
	prCmd.AddCommand(prChangesCmd)
	prCmd.AddCommand(prDiffCmd)
	prCmd.AddCommand(prThreadsCmd)
	prCmd.AddCommand(prSummaryCmd)
	prCmd.AddCommand(prReviewCmd)

	repoCmd.AddCommand(repoListCmd)
}

func init() {
	if prFormat == "" {
		prFormat = FormatYAML
	}
}
