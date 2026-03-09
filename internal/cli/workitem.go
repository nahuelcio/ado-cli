package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/your-org/azure-devops-cli/internal/api"
	"github.com/your-org/azure-devops-cli/internal/config"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatYAML  OutputFormat = "yaml"
)

var (
	format    OutputFormat
	profile   string
	profileName string
)

func getWorkItemClient(cmd *cobra.Command) (api.WorkItemClient, *config.ConfigLoader, error) {
	profileFlag, _ := cmd.Flags().GetString("profile")
	if profileFlag != "" {
		profileName = profileFlag
	}

	cfg := config.NewConfigLoader("")
	if profileName != "" {
		cfg.SetActiveProfile(profileName)
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

	token := auth.PAT
	if token == "" {
		return nil, nil, fmt.Errorf("PAT not configured. Use --profile or set AZURE_DEVOPS_PAT")
	}

	client, err := api.GetWorkItemClient(context.Background(), org, proj, token)
	if err != nil {
		return nil, nil, err
	}

	return client, cfg, nil
}

func printOutput(data interface{}, format OutputFormat) error {
	switch format {
	case FormatJSON:
		output, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(output))
	case FormatYAML:
		output, err := yaml.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		fmt.Println(string(output))
	case FormatTable:
		printTable(data)
	}
	return nil
}

func printTable(data interface{}) {
	workItems, ok := data.([]api.WorkItem)
	if !ok {
		fmt.Printf("%+v\n", data)
		return
	}

	if len(workItems) == 0 {
		fmt.Println("No work items found")
		return
	}

	fmt.Println("Work Items")
	fmt.Println()
	fmt.Printf("%-10s %-40s %-15s %-15s %-25s\n", "ID", "Title", "State", "Type", "Assigned To")
	fmt.Println(strings.Repeat("-", 105))

	for _, wi := range workItems {
		id := strconv.Itoa(wi.ID)
		title := truncateString(getStringField(wi.Fields, "System.Title"), 38)
		state := getStringField(wi.Fields, "System.State")
		workItemType := getStringField(wi.Fields, "System.WorkItemType")
		assignedTo := getStringField(wi.Fields, "System.AssignedTo")

		if assignedTo != "" {
			if id, ok := wi.Fields["System.AssignedTo"].(map[string]interface{}); ok {
				assignedTo = truncateString(getStringField(id, "displayName"), 23)
			}
		}

		fmt.Printf("%-10s %-40s %-15s %-15s %-25s\n", id, title, state, workItemType, assignedTo)
	}
}

func getStringField(fields map[string]interface{}, key string) string {
	if val, ok := fields[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

var workItemCmd = &cobra.Command{
	Use:   "work-item",
	Short: "Manage Azure DevOps work items (create, read, update, comment)",
	Long: `Manage work items in Azure DevOps.

This command group provides full CRUD operations for work items including:
- Listing work items with filters (state, type, assignee)
- Getting detailed information about specific work items
- Creating new work items (Tasks, Bugs, Features, etc.)
- Adding comments and discussions
- Updating fields and changing states

Common work item types: Task, Bug, Feature, Epic, User Story

Examples:
  # List active tasks
  ado work-item list --state Active --type Task

  # Get work item details in JSON format
  ado work-item get --id 123 --format json

  # Create a new bug
  ado work-item create --title "Login button not working" --type Bug

  # Add a comment
  ado work-item comment --id 123 --text "Fixed in commit abc123"

  # Change state to Resolved
  ado work-item state --id 123 --state Resolved`,
}

var workItemListCmd = &cobra.Command{
	Use:   "list",
	Short: "List work items",
	Long:  `List work items with optional filters.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := getWorkItemClient(cmd)
		if err != nil {
			return err
		}

		state, _ := cmd.Flags().GetString("state")
		workType, _ := cmd.Flags().GetString("type")
		assignedTo, _ := cmd.Flags().GetString("assigned-to")

		filters := api.WorkItemFilters{
			State:    state,
			Type:     workType,
			Assignee: assignedTo,
			Limit:    100,
		}

		workItems, err := client.ListWorkItems(context.Background(), "", filters)
		if err != nil {
			return fmt.Errorf("failed to list work items: %w", err)
		}

		return printOutput(workItems, format)
	},
}

var workItemGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a work item",
	Long:  `Get a specific work item by ID.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := getWorkItemClient(cmd)
		if err != nil {
			return err
		}

		id, _ := cmd.Flags().GetInt("id")
		if id == 0 {
			return fmt.Errorf("work item ID is required")
		}

		includeComments, _ := cmd.Flags().GetBool("include-comments")

		expand := false
		wi, err := client.GetWorkItem(context.Background(), "", id, &expand)
		if err != nil {
			return fmt.Errorf("failed to get work item: %w", err)
		}

		output := map[string]interface{}{
			"workItem": wi,
		}

		if includeComments {
			comments, err := client.GetComments(context.Background(), "", id)
			if err == nil {
				output["comments"] = comments
			}
		}

		return printOutput(output, format)
	},
}

var workItemCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a work item",
	Long:  `Create a new work item.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := getWorkItemClient(cmd)
		if err != nil {
			return err
		}

		title, _ := cmd.Flags().GetString("title")
		workType, _ := cmd.Flags().GetString("type")
		description, _ := cmd.Flags().GetString("description")
		assignTo, _ := cmd.Flags().GetString("assign-to")

		if title == "" {
			return fmt.Errorf("title is required")
		}
		if workType == "" {
			return fmt.Errorf("type is required")
		}

		fields := map[string]interface{}{
			"System.Title": title,
		}

		if description != "" {
			fields["System.Description"] = description
		}
		if assignTo != "" {
			fields["System.AssignedTo"] = assignTo
		}

		wi, err := client.CreateWorkItem(context.Background(), "", workType, fields)
		if err != nil {
			return fmt.Errorf("failed to create work item: %w", err)
		}

		return printOutput(wi, format)
	},
}

var workItemCommentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Add a comment to a work item",
	Long:  `Add a comment to an existing work item.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := getWorkItemClient(cmd)
		if err != nil {
			return err
		}

		id, _ := cmd.Flags().GetInt("id")
		text, _ := cmd.Flags().GetString("text")

		if id == 0 {
			return fmt.Errorf("work item ID is required")
		}
		if text == "" {
			return fmt.Errorf("comment text is required")
		}

		comment, err := client.AddComment(context.Background(), "", id, text)
		if err != nil {
			return fmt.Errorf("failed to add comment: %w", err)
		}

		return printOutput(comment, format)
	},
}

var workItemFieldCmd = &cobra.Command{
	Use:   "field",
	Short: "Update a work item field",
	Long:  `Update a specific field on a work item.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := getWorkItemClient(cmd)
		if err != nil {
			return err
		}

		id, _ := cmd.Flags().GetInt("id")
		field, _ := cmd.Flags().GetString("field")
		value, _ := cmd.Flags().GetString("value")

		if id == 0 {
			return fmt.Errorf("work item ID is required")
		}
		if field == "" {
			return fmt.Errorf("field is required")
		}
		if value == "" {
			return fmt.Errorf("value is required")
		}

		updates := []map[string]interface{}{
			{
				"op":    "add",
				"path":  fmt.Sprintf("/fields/%s", field),
				"value": value,
			},
		}

		wi, err := client.UpdateWorkItem(context.Background(), "", id, updates)
		if err != nil {
			return fmt.Errorf("failed to update work item: %w", err)
		}

		return printOutput(wi, format)
	},
}

var workItemStateCmd = &cobra.Command{
	Use:   "state",
	Short: "Change work item state",
	Long:  `Change the state of a work item.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := getWorkItemClient(cmd)
		if err != nil {
			return err
		}

		id, _ := cmd.Flags().GetInt("id")
		state, _ := cmd.Flags().GetString("state")
		reason, _ := cmd.Flags().GetString("reason")

		if id == 0 {
			return fmt.Errorf("work item ID is required")
		}
		if state == "" {
			return fmt.Errorf("state is required")
		}

		updates := []map[string]interface{}{
			{
				"op":    "add",
				"path":  "/fields/System.State",
				"value": state,
			},
		}

		if reason != "" {
			updates = append(updates, map[string]interface{}{
				"op":    "add",
				"path":  "/fields/System.Reason",
				"value": reason,
			})
		}

		wi, err := client.UpdateWorkItem(context.Background(), "", id, updates)
		if err != nil {
			return fmt.Errorf("failed to update work item state: %w", err)
		}

		return printOutput(wi, format)
	},
}

var workItemUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a work item",
	Long:  `Update a work item (alias for field command).`,
	RunE:  workItemFieldCmd.RunE,
}

func init() {
	workItemListCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	workItemListCmd.Flags().String("state", "", "Filter by state")
	workItemListCmd.Flags().String("type", "", "Filter by work item type")
	workItemListCmd.Flags().String("assigned-to", "", "Filter by assignee")
	workItemListCmd.Flags().VarP(&format, "format", "f", "Output format (table/json/yaml)")

	workItemGetCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	workItemGetCmd.Flags().Int("id", 0, "Work item ID")
	workItemGetCmd.Flags().Bool("include-comments", false, "Include comments")
	workItemGetCmd.Flags().VarP(&format, "format", "f", "Output format (table/json/yaml)")

	workItemCreateCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	workItemCreateCmd.Flags().String("title", "", "Work item title (required)")
	workItemCreateCmd.Flags().String("type", "", "Work item type: Task, Bug, Feature (required)")
	workItemCreateCmd.Flags().String("description", "", "Work item description")
	workItemCreateCmd.Flags().String("assign-to", "", "Assignee")
	workItemCreateCmd.Flags().VarP(&format, "format", "f", "Output format (table/json/yaml)")

	workItemCommentCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	workItemCommentCmd.Flags().Int("id", 0, "Work item ID (required)")
	workItemCommentCmd.Flags().String("text", "", "Comment text (required)")
	workItemCommentCmd.Flags().VarP(&format, "format", "f", "Output format (table/json/yaml)")

	workItemFieldCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	workItemFieldCmd.Flags().Int("id", 0, "Work item ID (required)")
	workItemFieldCmd.Flags().String("field", "", "Field name (required)")
	workItemFieldCmd.Flags().String("value", "", "New value (required)")
	workItemFieldCmd.Flags().VarP(&format, "format", "f", "Output format (table/json/yaml)")

	workItemStateCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	workItemStateCmd.Flags().Int("id", 0, "Work item ID (required)")
	workItemStateCmd.Flags().String("state", "", "New state (required)")
	workItemStateCmd.Flags().String("reason", "", "Reason for state change")
	workItemStateCmd.Flags().VarP(&format, "format", "f", "Output format (table/json/yaml)")

	workItemUpdateCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	workItemUpdateCmd.Flags().Int("id", 0, "Work item ID (required)")
	workItemUpdateCmd.Flags().String("field", "", "Field name (required)")
	workItemUpdateCmd.Flags().String("value", "", "New value (required)")
	workItemUpdateCmd.Flags().VarP(&format, "format", "f", "Output format (table/json/yaml)")

	workItemCmd.AddCommand(workItemListCmd)
	workItemCmd.AddCommand(workItemGetCmd)
	workItemCmd.AddCommand(workItemCreateCmd)
	workItemCmd.AddCommand(workItemCommentCmd)
	workItemCmd.AddCommand(workItemFieldCmd)
	workItemCmd.AddCommand(workItemStateCmd)
	workItemCmd.AddCommand(workItemUpdateCmd)

	rootCmd.AddCommand(workItemCmd)
}

func init() {
	if format == "" {
		format = FormatTable
	}

	_ = os.Stderr
}
