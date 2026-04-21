package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nahuelcio/ado-cli/internal/api"
	"github.com/nahuelcio/ado-cli/internal/config"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type OutputFormat string

const (
	FormatTable    OutputFormat = "table"
	FormatJSON     OutputFormat = "json"
	FormatYAML     OutputFormat = "yaml"
	FormatMarkdown OutputFormat = "markdown"
)

func (f *OutputFormat) String() string {
	if f == nil {
		return string(FormatTable)
	}
	return string(*f)
}

func (f *OutputFormat) Set(value string) error {
	switch OutputFormat(value) {
	case FormatTable, FormatJSON, FormatYAML, FormatMarkdown:
		*f = OutputFormat(value)
		return nil
	default:
		return fmt.Errorf("invalid format %q: use table, json, yaml, or markdown", value)
	}
}

func (f *OutputFormat) Type() string {
	return "output-format"
}

var format OutputFormat

func getWorkItemClient(cmd *cobra.Command) (api.WorkItemClient, string, *config.ConfigLoader, error) {
	cfg, authCfg, err := getConfigAndAuth(cmd)
	if err != nil {
		return nil, "", nil, err
	}

	if err := checkProfileScope(cfg, ScopeWorkItems); err != nil {
		return nil, "", nil, err
	}

	client, err := api.GetWorkItemClient(context.Background(), authCfg.Org, authCfg.Project, authCfg.PAT)
	if err != nil {
		return nil, "", nil, err
	}

	return client, authCfg.Project, cfg, nil
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
	case FormatMarkdown:
		printMarkdown(data)
	case FormatTable:
		printTable(data)
	default:
		output, err := yaml.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		fmt.Println(string(output))
	}
	return nil
}

func printMarkdown(data interface{}) {
	switch v := data.(type) {
	case []map[string]interface{}:
		printMarkdownList(v)
	case map[string]interface{}:
		if _, ok := v["by_parent"]; ok {
			printMarkdownDashboard(v)
		} else {
			printMarkdownSingle(v)
		}
	default:
		fmt.Printf("%+v\n", data)
	}
}

func printMarkdownList(items []map[string]interface{}) {
	if len(items) == 0 {
		fmt.Println("No work items found")
		return
	}

	fmt.Printf("# Work Items (%d)\n\n", len(items))
	for _, item := range items {
		id, _ := item["id"].(int)
		title, _ := item["title"].(string)
		state, _ := item["state"].(string)
		wiType, _ := item["type"].(string)
		assignedTo, _ := item["assigned_to"].(string)

		fmt.Printf("## #%d - %s\n", id, title)
		fmt.Printf("- **State:** %s\n", state)
		if wiType != "" {
			fmt.Printf("- **Type:** %s\n", wiType)
		}
		if assignedTo != "" {
			fmt.Printf("- **Assigned To:** %s\n", assignedTo)
		}
		if parentID := extractParentIDFromRelated(item); parentID > 0 {
			fmt.Printf("- **Parent:** #%d\n", parentID)
		}

		if desc, ok := item["description"].(string); ok && desc != "" {
			fmt.Printf("\n> %s\n", strings.ReplaceAll(desc, "\n", "\n> "))
		}

		if comments, ok := item["comments"].([]map[string]interface{}); ok && len(comments) > 0 {
			fmt.Printf("\n**Comments (%d):**\n", len(comments))
			for i, c := range comments {
				author, _ := c["author"].(string)
				date, _ := c["date"].(string)
				text, _ := c["text"].(string)
				fmt.Printf("%d. **%s** (%s): %s\n", i+1, author, date, text)
			}
		}

		fmt.Println("\n---")
	}
}

func printMarkdownGrouped(groups []map[string]interface{}) {
	if len(groups) == 0 {
		fmt.Println("No work items found")
		return
	}

	fmt.Println("# Work Items by Parent")
	for _, group := range groups {
		parentLabel, _ := group["parent_label"].(string)
		parentState, _ := group["parent_state"].(string)
		items, _ := group["items"].([]map[string]interface{})

		if parentLabel == "" {
			parentLabel = "No Parent"
		}

		fmt.Printf("## %s\n", parentLabel)
		if parentState != "" {
			fmt.Printf("**State:** %s\n\n", parentState)
		}

		if parentComments, ok := group["parent_comments"].([]map[string]interface{}); ok && len(parentComments) > 0 {
			fmt.Println("**Parent Comments:**")
			for _, c := range parentComments {
				author, _ := c["author"].(string)
				text, _ := c["text"].(string)
				fmt.Printf("- **%s**: %s\n", author, text)
			}
			fmt.Println()
		}

		for _, item := range items {
			id, _ := item["id"].(int)
			title, _ := item["title"].(string)
			state, _ := item["state"].(string)
			wiType, _ := item["type"].(string)

			fmt.Printf("### #%d - %s\n", id, title)
			fmt.Printf("- **State:** %s\n", state)
			if wiType != "" {
				fmt.Printf("- **Type:** %s\n", wiType)
			}

			if desc, ok := item["description"].(string); ok && desc != "" {
				fmt.Printf("\n> %s\n", strings.ReplaceAll(desc, "\n", "\n> "))
			}

			if comments, ok := item["comments"].([]map[string]interface{}); ok && len(comments) > 0 {
				fmt.Printf("\n**Comments (%d):**\n", len(comments))
				for i, c := range comments {
					author, _ := c["author"].(string)
					text, _ := c["text"].(string)
					fmt.Printf("%d. **%s**: %s\n", i+1, author, text)
				}
			}

			fmt.Println()
		}

		fmt.Println("---")
	}
}

func printMarkdownSingle(item map[string]interface{}) {
	id, _ := item["id"].(int)
	title, _ := item["title"].(string)
	state, _ := item["state"].(string)

	fmt.Printf("# #%d - %s\n", id, title)
	fmt.Printf("- **State:** %s\n", state)
	for k, v := range item {
		switch k {
		case "id", "title", "state", "comments":
			continue
		}
		fmt.Printf("- **%s:** %v\n", k, v)
	}

	if comments, ok := item["comments"].([]map[string]interface{}); ok && len(comments) > 0 {
		fmt.Printf("\n## Comments\n")
		for i, c := range comments {
			author, _ := c["author"].(string)
			date, _ := c["date"].(string)
			text, _ := c["text"].(string)
			fmt.Printf("%d. **%s** (%s): %s\n", i+1, author, date, text)
		}
	}
}

func printMarkdownDashboard(data map[string]interface{}) {
	assignedTo, _ := data["assigned_to"].(string)
	fmt.Printf("# Dashboard - %s\n\n", assignedTo)

	if totals, ok := data["totals"].(map[string]interface{}); ok {
		fmt.Println("## Totals")
		for state, count := range totals {
			fmt.Printf("- **%s:** %v\n", state, count)
		}
		fmt.Println()
	}

	if byParent, ok := data["by_parent"].([]map[string]interface{}); ok {
		fmt.Println("## By Parent")
		for _, group := range byParent {
			parentLabel, _ := group["parent_label"].(string)
			parentState, _ := group["parent_state"].(string)
			items, _ := group["items"].([]map[string]interface{})

			fmt.Printf("### %s", parentLabel)
			if parentState != "" {
				fmt.Printf(" (%s)", parentState)
			}
			fmt.Println()

			for _, item := range items {
				id, _ := item["id"].(int)
				title, _ := item["title"].(string)
				state, _ := item["state"].(string)
				fmt.Printf("- #%d **%s** [%s]\n", id, title, state)
			}
			fmt.Println()
		}
	}
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
		assignedTo := ""
		if assigneeData, ok := wi.Fields["System.AssignedTo"].(map[string]interface{}); ok {
			assignedTo = truncateString(getStringField(assigneeData, "displayName"), 23)
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
	Short: "Manage work items",
	Long: `Work items: list, get, create, comment, state.

Examples:
  ado work-item list --mine
  ado work-item get --id 123
  ado work-item create --title "Bug" --type Bug
  ado work-item state --id 123 --state Resolved`,
}

var workItemListCmd = &cobra.Command{
	Use:   "list",
	Short: "List work items",
	Long:  `List work items. Filters: --state, --type, --assigned-to, --mine, --all-states.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, project, _, err := getWorkItemClient(cmd)
		if err != nil {
			return err
		}

		state, _ := cmd.Flags().GetString("state")
		workType, _ := cmd.Flags().GetString("type")
		assignedTo, _ := cmd.Flags().GetString("assigned-to")
		mine, _ := cmd.Flags().GetBool("mine")
		full, _ := cmd.Flags().GetBool("full")
		withDetails, _ := cmd.Flags().GetBool("with-details")
		expandParents, _ := cmd.Flags().GetBool("expand-parents")
		groupBy, _ := cmd.Flags().GetString("group-by")

		if mine && assignedTo != "" {
			return fmt.Errorf("--mine cannot be used together with --assigned-to")
		}

		filters := api.WorkItemFilters{
			State:    state,
			Type:     workType,
			Assignee: assignedTo,
			Mine:     mine,
			Limit:    100,
		}

		workItems, err := client.ListWorkItems(context.Background(), project, filters)
		if err != nil {
			return fmt.Errorf("failed to list work items: %w", err)
		}

		if full {
			return printOutput(workItems, format)
		}

		var llmData []map[string]interface{}
		for _, wi := range workItems {
			llmData = append(llmData, extractLLMWorkItemData(&wi))
		}

		if withDetails {
			fetchCommentsForItems(client, project, llmData)
		}

		if expandParents {
			fetchAndInlineParents(client, project, llmData)
		}

		if format == "" {
			format = FormatYAML
		}

		if groupBy == "parent" {
			grouped := groupByParent(llmData)
			if format == FormatMarkdown {
				printMarkdownGrouped(grouped)
				return nil
			}
			return printOutput(grouped, format)
		}

		if format == FormatMarkdown {
			printMarkdownList(llmData)
			return nil
		}

		return printOutput(llmData, format)
	},
}

func extractLLMWorkItemData(wi *api.WorkItem) map[string]interface{} {
	result := map[string]interface{}{
		"id": wi.ID,
	}

	if title, ok := wi.Fields["System.Title"].(string); ok {
		result["title"] = title
	}
	if state, ok := wi.Fields["System.State"].(string); ok {
		result["state"] = state
	}
	if workItemType, ok := wi.Fields["System.WorkItemType"].(string); ok {
		result["type"] = workItemType
	}
	if assignee, ok := wi.Fields["System.AssignedTo"].(map[string]interface{}); ok {
		if displayName, ok := assignee["displayName"].(string); ok {
			result["assigned_to"] = displayName
		}
	}
	if desc, ok := wi.Fields["System.Description"].(string); ok {
		result["description"] = cleanHTML(desc)
	}

	if commentCount, ok := wi.Fields["System.CommentCount"].(float64); ok && commentCount > 0 {
		result["has_comments"] = true
		result["comment_count"] = int(commentCount)
	}

	if len(wi.Relations) > 0 {
		relationTypes := map[string]string{
			"System.LinkTypes.Hierarchy-Forward":  "child",
			"System.LinkTypes.Hierarchy-Reverse":  "parent",
			"System.LinkTypes.Related":            "related",
			"System.LinkTypes.Dependency-Forward": "successor",
			"System.LinkTypes.Dependency-Reverse": "predecessor",
		}

		var relatedItems []map[string]interface{}
		for _, rel := range wi.Relations {
			relType := rel.Rel
			if friendlyName, ok := relationTypes[rel.Rel]; ok {
				relType = friendlyName
			}

			relID := ""
			parts := strings.Split(rel.URL, "/")
			if len(parts) > 0 {
				relID = parts[len(parts)-1]
			}

			relatedItems = append(relatedItems, map[string]interface{}{
				"type": relType,
				"id":   relID,
			})
		}
		if len(relatedItems) > 0 {
			result["related"] = relatedItems
		}
	}

	return result
}

func extractRelatedIDs(relations []api.WorkItemRelation) []int {
	var ids []int
	for _, rel := range relations {
		if rel.Rel == "System.LinkTypes.Hierarchy-Forward" ||
			rel.Rel == "System.LinkTypes.Hierarchy-Reverse" ||
			rel.Rel == "System.LinkTypes.Related" {
			parts := strings.Split(rel.URL, "/")
			if len(parts) > 0 {
				idStr := parts[len(parts)-1]
				if id, err := strconv.Atoi(idStr); err == nil {
					ids = append(ids, id)
				}
			}
		}
	}
	return ids
}

func extractParentIDFromRelated(item map[string]interface{}) int {
	related, ok := item["related"].([]map[string]interface{})
	if !ok {
		return 0
	}
	for _, rel := range related {
		if relType, ok := rel["type"].(string); ok && relType == "parent" {
			if idStr, ok := rel["id"].(string); ok {
				if id, err := strconv.Atoi(idStr); err == nil {
					return id
				}
			}
		}
	}
	return 0
}

func fetchCommentsForItems(client api.WorkItemClient, project string, items []map[string]interface{}) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	var mu sync.Mutex

	for i, item := range items {
		id, ok := item["id"].(int)
		if !ok {
			continue
		}
		commentCount := 0
		if cc, ok := item["comment_count"].(int); ok {
			commentCount = cc
		}
		if commentCount == 0 {
			continue
		}

		wg.Add(1)
		go func(idx int, itemID int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			comments, err := client.GetComments(context.Background(), project, itemID)
			if err != nil {
				return
			}

			var simpleComments []map[string]interface{}
			for _, c := range comments {
				author := ""
				if c.CreatedBy != nil {
					author = c.CreatedBy.DisplayName
				}
				simpleComments = append(simpleComments, map[string]interface{}{
					"author": author,
					"date":   c.CreatedDate,
					"text":   cleanHTML(c.Text),
				})
			}

			mu.Lock()
			items[idx]["comments"] = simpleComments
			mu.Unlock()
		}(i, id)
	}
	wg.Wait()
}

func fetchAndInlineParents(client api.WorkItemClient, project string, items []map[string]interface{}) {
	parentIDSet := make(map[int]bool)
	for _, item := range items {
		if pid := extractParentIDFromRelated(item); pid > 0 {
			parentIDSet[pid] = true
		}
	}

	if len(parentIDSet) == 0 {
		return
	}

	parentIDs := make([]int, 0, len(parentIDSet))
	for id := range parentIDSet {
		parentIDs = append(parentIDs, id)
	}

	parentWIs, err := client.GetWorkItemsBatch(context.Background(), project, parentIDs)
	if err != nil {
		return
	}

	parentMap := make(map[int]*api.WorkItem)
	for i := range parentWIs {
		parentMap[parentWIs[i].ID] = &parentWIs[i]
	}

	parentComments := make(map[int][]map[string]interface{})
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	var mu sync.Mutex

	for pid, pw := range parentMap {
		if cc, ok := pw.Fields["System.CommentCount"].(float64); ok && cc > 0 {
			wg.Add(1)
			go func(parentID int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				comments, err := client.GetComments(context.Background(), project, parentID)
				if err != nil {
					return
				}
				var simpleComments []map[string]interface{}
				for _, c := range comments {
					author := ""
					if c.CreatedBy != nil {
						author = c.CreatedBy.DisplayName
					}
					simpleComments = append(simpleComments, map[string]interface{}{
						"author": author,
						"date":   c.CreatedDate,
						"text":   cleanHTML(c.Text),
					})
				}
				mu.Lock()
				parentComments[parentID] = simpleComments
				mu.Unlock()
			}(pid)
		}
	}
	wg.Wait()

	for _, item := range items {
		pid := extractParentIDFromRelated(item)
		if pid == 0 {
			continue
		}
		pw, ok := parentMap[pid]
		if !ok {
			continue
		}
		parentData := map[string]interface{}{
			"id":    pw.ID,
			"title": getStringField(pw.Fields, "System.Title"),
			"state": getStringField(pw.Fields, "System.State"),
		}
		if desc := getStringField(pw.Fields, "System.Description"); desc != "" {
			parentData["description"] = cleanHTML(desc)
		}
		if comments, ok := parentComments[pid]; ok && len(comments) > 0 {
			parentData["comments"] = comments
		}
		item["parent"] = parentData
	}
}

func groupByParent(items []map[string]interface{}) []map[string]interface{} {
	groups := make(map[string][]map[string]interface{})
	var groupOrder []string

	for _, item := range items {
		pid := extractParentIDFromRelated(item)
		var key string
		if pid > 0 {
			key = strconv.Itoa(pid)
			if parentData, ok := item["parent"].(map[string]interface{}); ok {
				if title, ok := parentData["title"].(string); ok {
					key = fmt.Sprintf("#%d %s", pid, title)
				}
			}
		} else {
			key = "__no_parent__"
		}

		if _, exists := groups[key]; !exists {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], item)
	}

	var result []map[string]interface{}
	for _, key := range groupOrder {
		group := map[string]interface{}{
			"items": groups[key],
		}
		if key == "__no_parent__" {
			group["parent_label"] = "No Parent"
		} else {
			group["parent_label"] = key
			if len(groups[key]) > 0 {
				if parentData, ok := groups[key][0]["parent"].(map[string]interface{}); ok {
					if state, ok := parentData["state"].(string); ok {
						group["parent_state"] = state
					}
					if comments, ok := parentData["comments"].([]map[string]interface{}); ok {
						group["parent_comments"] = comments
					}
				}
			}
		}
		result = append(result, group)
	}
	return result
}

var workItemGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a work item",
	Long:  `Get a specific work item by ID.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, project, _, err := getWorkItemClient(cmd)
		if err != nil {
			return err
		}

		id, _ := cmd.Flags().GetInt("id")
		if id == 0 {
			return fmt.Errorf("work item ID is required")
		}

		full, _ := cmd.Flags().GetBool("full")

		expand := true
		wi, err := client.GetWorkItem(context.Background(), project, id, &expand)
		if err != nil {
			return fmt.Errorf("failed to get work item: %w", err)
		}

		var output map[string]interface{}
		if full {
			output = map[string]interface{}{
				"workItem": *wi,
			}
		} else {
			output = extractLLMWorkItemData(wi)
		}

		if commentCount, ok := wi.Fields["System.CommentCount"].(float64); ok && commentCount > 0 {
			comments, err := client.GetComments(context.Background(), project, id)
			if err == nil && len(comments) > 0 {
				var simpleComments []map[string]interface{}
				for _, c := range comments {
					author := ""
					if c.CreatedBy != nil {
						author = c.CreatedBy.DisplayName
					}
					simpleComments = append(simpleComments, map[string]interface{}{
						"author": author,
						"date":   c.CreatedDate,
						"text":   cleanHTML(c.Text),
					})
				}
				output["comments"] = simpleComments
			}
		}

		relatedFull, _ := cmd.Flags().GetBool("related-full")
		if relatedFull && len(wi.Relations) > 0 {
			relatedIDs := extractRelatedIDs(wi.Relations)
			parentID := 0
			for _, rel := range wi.Relations {
				if rel.Rel == "System.LinkTypes.Hierarchy-Reverse" {
					parts := strings.Split(rel.URL, "/")
					if len(parts) > 0 {
						if id, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
							parentID = id
						}
					}
				}
			}

			if parentID > 0 {
				allIDs := append([]int{parentID}, relatedIDs...)
				relatedIDs = allIDs
			}

			if len(relatedIDs) > 0 {
				relatedWIs, err := client.GetWorkItemsBatch(context.Background(), project, relatedIDs)
				if err == nil {
					var qaFeedbacks []map[string]interface{}
					var parentData map[string]interface{}
					for _, relatedWI := range relatedWIs {
						wiType := getStringField(relatedWI.Fields, "System.WorkItemType")
						if relatedWI.ID == parentID {
							parentData = map[string]interface{}{
								"id":    relatedWI.ID,
								"title": getStringField(relatedWI.Fields, "System.Title"),
								"state": getStringField(relatedWI.Fields, "System.State"),
							}
							if desc := getStringField(relatedWI.Fields, "System.Description"); desc != "" {
								parentData["description"] = cleanHTML(desc)
							}
							if cc, ok := relatedWI.Fields["System.CommentCount"].(float64); ok && cc > 0 {
								comments, err := client.GetComments(context.Background(), project, parentID)
								if err == nil && len(comments) > 0 {
									var simpleComments []map[string]interface{}
									for _, c := range comments {
										author := ""
										if c.CreatedBy != nil {
											author = c.CreatedBy.DisplayName
										}
										simpleComments = append(simpleComments, map[string]interface{}{
											"author": author,
											"date":   c.CreatedDate,
											"text":   cleanHTML(c.Text),
										})
									}
									parentData["comments"] = simpleComments
								}
							}
						} else if wiType == "QA Feedback" {
							desc := getStringField(relatedWI.Fields, "System.Description")
							qaFeedbacks = append(qaFeedbacks, map[string]interface{}{
								"id":          relatedWI.ID,
								"title":       getStringField(relatedWI.Fields, "System.Title"),
								"state":       getStringField(relatedWI.Fields, "System.State"),
								"description": cleanHTML(desc),
								"url":         relatedWI.Links["html"].HRef,
							})
						}
					}
					if parentData != nil {
						output["parent"] = parentData
					}
					if len(qaFeedbacks) > 0 {
						output["qa_feedbacks_count"] = len(qaFeedbacks)
						output["qa_feedbacks"] = qaFeedbacks
					}
				}
			}
		}

		if format == "" {
			format = FormatYAML
		}

		return printOutput(output, format)
	},
}

var workItemCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a work item",
	Long:  `Create a new work item.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, project, _, err := getWorkItemClient(cmd)
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

		wi, err := client.CreateWorkItem(context.Background(), project, workType, fields)
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
		client, project, _, err := getWorkItemClient(cmd)
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

		comment, err := client.AddComment(context.Background(), project, id, text)
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
		client, project, _, err := getWorkItemClient(cmd)
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
				"op":    "replace",
				"path":  fmt.Sprintf("/fields/%s", field),
				"value": value,
			},
		}

		wi, err := client.UpdateWorkItem(context.Background(), project, id, updates)
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
		client, project, _, err := getWorkItemClient(cmd)
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
				"op":    "replace",
				"path":  "/fields/System.State",
				"value": state,
			},
		}

		if reason != "" {
			updates = append(updates, map[string]interface{}{
				"op":    "replace",
				"path":  "/fields/System.Reason",
				"value": reason,
			})
		}

		wi, err := client.UpdateWorkItem(context.Background(), project, id, updates)
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

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Show your work dashboard",
	Long: `Show a summary of all work items assigned to you, grouped by parent.
Includes totals by state and hierarchical view.

Aliases: ado my-work
Examples:
  ado dashboard
  ado dashboard --format markdown
  ado dashboard --format json`,
	Aliases: []string{"my-work"},
	RunE: func(cmd *cobra.Command, args []string) error {
		client, project, _, err := getWorkItemClient(cmd)
		if err != nil {
			return err
		}

		filters := api.WorkItemFilters{
			Mine:  true,
			Limit: 200,
		}

		workItems, err := client.ListWorkItems(context.Background(), project, filters)
		if err != nil {
			return fmt.Errorf("failed to list work items: %w", err)
		}

		var llmData []map[string]interface{}
		for _, wi := range workItems {
			llmData = append(llmData, extractLLMWorkItemData(&wi))
		}

		fetchCommentsForItems(client, project, llmData)
		fetchAndInlineParents(client, project, llmData)

		var assignedTo string
		for _, item := range llmData {
			if a, ok := item["assigned_to"].(string); ok && a != "" {
				assignedTo = a
				break
			}
		}

		totals := make(map[string]int)
		oneWeekAgo := time.Now().AddDate(0, 0, -7)
		closedThisWeek := 0
		for _, item := range llmData {
			state, _ := item["state"].(string)
			if state != "" {
				totals[state]++
			}
			if state == "Closed" {
				continue
			}
			_ = oneWeekAgo
			_ = closedThisWeek
		}
		totals["total"] = len(llmData)

		grouped := groupByParent(llmData)

		dashboard := map[string]interface{}{
			"assigned_to": assignedTo,
			"totals":      totals,
			"by_parent":   grouped,
		}

		if format == "" {
			format = FormatYAML
		}

		if format == FormatMarkdown {
			printMarkdownDashboard(dashboard)
			return nil
		}

		return printOutput(dashboard, format)
	},
}

func init() {
	workItemListCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	workItemListCmd.Flags().String("state", "", "Filter by state")
	workItemListCmd.Flags().String("type", "", "Filter by work item type")
	workItemListCmd.Flags().String("assigned-to", "", "Filter by assignee")
	workItemListCmd.Flags().Bool("mine", false, "Filter by work items assigned to the authenticated user")
	workItemListCmd.Flags().Bool("all-states", false, "Explicitly fetch all states (default behavior)")
	workItemListCmd.Flags().Bool("full", false, "Show all fields (default: LLM-optimized view)")
	workItemListCmd.Flags().Bool("with-details", false, "Fetch comments for each work item (batched)")
	workItemListCmd.Flags().Bool("expand-parents", false, "Inline parent work item data with comments")
	workItemListCmd.Flags().String("group-by", "", "Group results by field (parent)")
	workItemListCmd.Flags().VarP(&format, "format", "f", "Output format (table/json/yaml/markdown) - default: yaml")

	workItemGetCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	workItemGetCmd.Flags().Int("id", 0, "Work item ID (required)")
	workItemGetCmd.Flags().Bool("full", false, "Show all fields (default: LLM-optimized view)")
	workItemGetCmd.Flags().Bool("related-full", false, "Fetch and show parent, QA Feedbacks and related work items with full details")
	workItemGetCmd.Flags().VarP(&format, "format", "f", "Output format (table/json/yaml/markdown) - default: yaml")

	workItemCreateCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	workItemCreateCmd.Flags().String("title", "", "Work item title (required)")
	workItemCreateCmd.Flags().String("type", "", "Work item type: Task, Bug, Feature (required)")
	workItemCreateCmd.Flags().String("description", "", "Work item description")
	workItemCreateCmd.Flags().String("assign-to", "", "Assignee")
	workItemCreateCmd.Flags().VarP(&format, "format", "f", "Output format (table/json/yaml/markdown)")

	workItemCommentCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	workItemCommentCmd.Flags().Int("id", 0, "Work item ID (required)")
	workItemCommentCmd.Flags().String("text", "", "Comment text (required)")
	workItemCommentCmd.Flags().VarP(&format, "format", "f", "Output format (table/json/yaml/markdown)")

	workItemFieldCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	workItemFieldCmd.Flags().Int("id", 0, "Work item ID (required)")
	workItemFieldCmd.Flags().String("field", "", "Field name (required)")
	workItemFieldCmd.Flags().String("value", "", "New value (required)")
	workItemFieldCmd.Flags().VarP(&format, "format", "f", "Output format (table/json/yaml/markdown)")

	workItemStateCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	workItemStateCmd.Flags().Int("id", 0, "Work item ID (required)")
	workItemStateCmd.Flags().String("state", "", "New state (required)")
	workItemStateCmd.Flags().String("reason", "", "Reason for state change")
	workItemStateCmd.Flags().VarP(&format, "format", "f", "Output format (table/json/yaml/markdown)")

	workItemUpdateCmd.Flags().StringP("profile", "p", "", "Azure DevOps profile to use")
	workItemUpdateCmd.Flags().Int("id", 0, "Work item ID (required)")
	workItemUpdateCmd.Flags().String("field", "", "Field name (required)")
	workItemUpdateCmd.Flags().String("value", "", "New value (required)")
	workItemUpdateCmd.Flags().VarP(&format, "format", "f", "Output format (table/json/yaml/markdown)")

	workItemCmd.AddCommand(workItemListCmd)
	workItemCmd.AddCommand(workItemGetCmd)
	workItemCmd.AddCommand(workItemCreateCmd)
	workItemCmd.AddCommand(workItemCommentCmd)
	workItemCmd.AddCommand(workItemFieldCmd)
	workItemCmd.AddCommand(workItemStateCmd)
	workItemCmd.AddCommand(workItemUpdateCmd)
	workItemCmd.AddCommand(dashboardCmd)
}

func init() {
	if format == "" {
		format = FormatYAML
	}

	_ = os.Stderr
}
