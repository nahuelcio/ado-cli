package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxWorkItemsBatchSize = 200

type WorkItemFields map[string]interface{}

type WorkItemComment struct {
	WorkItemID   int          `json:"workItemId,omitempty"`
	CommentId    int          `json:"commentId,omitempty"`
	Text         string       `json:"text"`
	CreatedDate  string       `json:"createdDate,omitempty"`
	CreatedBy    *IdentityRef `json:"createdBy,omitempty"`
	ModifiedDate string       `json:"modifiedDate,omitempty"`
	ModifiedBy   *IdentityRef `json:"modifiedBy,omitempty"`
	Version      int          `json:"version,omitempty"`
	IsDeleted    bool         `json:"isDeleted,omitempty"`
	URL          string       `json:"url,omitempty"`
}

type WorkItem struct {
	ID        int                `json:"id"`
	Rev       int                `json:"rev"`
	Fields    WorkItemFields     `json:"fields"`
	Relations []WorkItemRelation `json:"relations,omitempty"`
	Links     map[string]LinkRef `json:"_links,omitempty"`
	URL       string             `json:"url"`
}

type WorkItemRelation struct {
	Rel        string                 `json:"rel"`
	URL        string                 `json:"url"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type LinkRef struct {
	HRef string `json:"href"`
}

type WorkItemCommentsResponse struct {
	Count             int               `json:"count"`
	Comments          []WorkItemComment `json:"comments"`
	TotalCount        int               `json:"totalCount"`
	ContinuationToken string            `json:"continuationToken,omitempty"`
	NextPage          string            `json:"nextPage,omitempty"`
}

type WorkItemFilters struct {
	State    string
	Assignee string
	Mine     bool
	Type     string
	Limit    int
}

type WorkItemStateTransition struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason,omitempty"`
}

type WorkItemTypeInfo struct {
	Name        string                    `json:"name"`
	States      []string                  `json:"states"`
	Transitions []WorkItemStateTransition `json:"transitions,omitempty"`
}

type WiqlQueryResult struct {
	QueryResultType string            `json:"queryResultType"`
	WorkItems       []WiqlWorkItemRef `json:"workItems"`
	Columns         []WiqlColumnRef   `json:"columns"`
}

type WiqlWorkItemRef struct {
	ID  int    `json:"id"`
	URL string `json:"url"`
}

type WiqlColumnRef struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type WorkItemClient interface {
	GetWorkItem(ctx context.Context, project string, id int, expand *bool) (*WorkItem, error)
	ListWorkItems(ctx context.Context, project string, filters WorkItemFilters) ([]WorkItem, error)
	CreateWorkItem(ctx context.Context, project string, workItemType string, fields map[string]interface{}) (*WorkItem, error)
	UpdateWorkItem(ctx context.Context, project string, id int, updates []map[string]interface{}) (*WorkItem, error)
	GetComments(ctx context.Context, project string, workItemID int) ([]WorkItemComment, error)
	AddComment(ctx context.Context, project string, workItemID int, text string) (*WorkItemComment, error)
	QueryByWiql(ctx context.Context, project string, wiqlQuery string, limit int) ([]WorkItem, error)
	GetWorkItemsBatch(ctx context.Context, project string, ids []int) ([]WorkItem, error)
	GetValidStates(ctx context.Context, project string, workItemID int) ([]string, error)
}

type workItemClient struct {
	client *AzureDevOpsClient
}

func (c *workItemClient) resolveProject(project string) string {
	if strings.TrimSpace(project) != "" {
		return project
	}
	return c.client.Config.Project
}

func NewWorkItemClient(client *AzureDevOpsClient) WorkItemClient {
	return &workItemClient{
		client: client,
	}
}

func (c *workItemClient) GetWorkItem(ctx context.Context, project string, id int, expand *bool) (*WorkItem, error) {
	project = c.resolveProject(project)
	url := fmt.Sprintf("%s/%s/_apis/wit/workitems/%d?api-version=7.1", c.client.Config.BaseURL, project, id)
	if expand != nil && *expand {
		url += "&$expand=all"
	}

	resp, err := c.client.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get work item %d: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get work item %d: status %d, body: %s", id, resp.StatusCode, string(body))
	}

	var result WorkItem
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode work item response: %w", err)
	}

	return &result, nil
}

func (c *workItemClient) ListWorkItems(ctx context.Context, project string, filters WorkItemFilters) ([]WorkItem, error) {
	project = c.resolveProject(project)
	selectClause := "SELECT [System.Id], [System.Title], [System.State], [System.AssignedTo], [System.WorkItemType]"
	fromClause := "FROM WorkItems"
	whereClauses := []string{"[System.TeamProject] = @project"}

	if filters.State != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("[System.State] = '%s'", escapeWiqlString(filters.State)))
	}
	if filters.Mine {
		whereClauses = append(whereClauses, "[System.AssignedTo] = @Me")
	} else if filters.Assignee != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("[System.AssignedTo] = '%s'", escapeWiqlString(filters.Assignee)))
	}
	if filters.Type != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("[System.WorkItemType] = '%s'", escapeWiqlString(filters.Type)))
	}

	wiql := fmt.Sprintf("%s\n%s\nWHERE %s", selectClause, fromClause, strings.Join(whereClauses, " AND "))

	workItems, err := c.QueryByWiql(ctx, project, wiql, filters.Limit)
	if err != nil {
		return nil, err
	}

	if filters.Limit > 0 && len(workItems) > filters.Limit {
		workItems = workItems[:filters.Limit]
	}

	return workItems, nil
}

func (c *workItemClient) CreateWorkItem(ctx context.Context, project string, workItemType string, fields map[string]interface{}) (*WorkItem, error) {
	project = c.resolveProject(project)
	url := fmt.Sprintf("%s/%s/_apis/wit/workitems/$%s?api-version=7.1", c.client.Config.BaseURL, project, workItemType)

	document := make([]JsonPatchOperation, 0, len(fields))
	for key, value := range fields {
		if value != nil && value != "" {
			document = append(document, JsonPatchOperation{
				Op:    "add",
				Path:  fmt.Sprintf("/fields/%s", key),
				Value: value,
			})
		}
	}

	resp, err := c.client.doRequest(ctx, http.MethodPost, url, document)
	if err != nil {
		return nil, fmt.Errorf("failed to create work item: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create work item: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result WorkItem
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode work item response: %w", err)
	}

	return &result, nil
}

func (c *workItemClient) UpdateWorkItem(ctx context.Context, project string, id int, updates []map[string]interface{}) (*WorkItem, error) {
	project = c.resolveProject(project)
	url := fmt.Sprintf("%s/%s/_apis/wit/workitems/%d?api-version=7.1", c.client.Config.BaseURL, project, id)

	document := make([]JsonPatchOperation, len(updates))
	for i, update := range updates {
		document[i] = JsonPatchOperation{
			Op:    update["op"].(string),
			Path:  update["path"].(string),
			Value: update["value"],
		}
	}

	resp, err := c.client.doRequest(ctx, http.MethodPatch, url, document)
	if err != nil {
		return nil, fmt.Errorf("failed to update work item %d: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to update work item %d: status %d, body: %s", id, resp.StatusCode, string(body))
	}

	var result WorkItem
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode work item response: %w", err)
	}

	return &result, nil
}

func (c *workItemClient) GetComments(ctx context.Context, project string, workItemID int) ([]WorkItemComment, error) {
	project = c.resolveProject(project)
	allComments := []WorkItemComment{}
	continuationToken := ""

	for {
		url := fmt.Sprintf("%s/%s/_apis/wit/workitems/%d/comments?api-version=7.1-preview.4", c.client.Config.BaseURL, project, workItemID)
		if continuationToken != "" {
			url = fmt.Sprintf("%s&continuationToken=%s", url, continuationToken)
		}

		resp, err := c.client.doRequest(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get comments for work item %d: %w", workItemID, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("failed to get comments for work item %d: status %d, body: %s", workItemID, resp.StatusCode, string(body))
		}

		var result WorkItemCommentsResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode comments response: %w", err)
		}

		allComments = append(allComments, result.Comments...)

		if result.ContinuationToken == "" {
			break
		}
		continuationToken = result.ContinuationToken
	}

	return allComments, nil
}

func (c *workItemClient) AddComment(ctx context.Context, project string, workItemID int, text string) (*WorkItemComment, error) {
	project = c.resolveProject(project)
	url := fmt.Sprintf("%s/%s/_apis/wit/workitems/%d/comments?api-version=7.1-preview.4", c.client.Config.BaseURL, project, workItemID)

	comment := map[string]string{
		"text": text,
	}

	resp, err := c.client.doRequest(ctx, http.MethodPost, url, comment)
	if err != nil {
		return nil, fmt.Errorf("failed to add comment to work item %d: %w", workItemID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to add comment to work item %d: status %d, body: %s", workItemID, resp.StatusCode, string(body))
	}

	var result WorkItemComment
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode comment response: %w", err)
	}

	return &result, nil
}

func (c *workItemClient) QueryByWiql(ctx context.Context, project string, wiqlQuery string, limit int) ([]WorkItem, error) {
	project = c.resolveProject(project)
	url := fmt.Sprintf("%s/%s/_apis/wit/wiql?api-version=7.1", c.client.Config.BaseURL, project)
	if limit > 0 {
		url = fmt.Sprintf("%s&$top=%d", url, limit)
	}

	query := map[string]string{
		"query": wiqlQuery,
	}

	resp, err := c.client.doRequest(ctx, http.MethodPost, url, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute WIQL query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to execute WIQL query: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result WiqlQueryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode WIQL response: %w", err)
	}

	if len(result.WorkItems) == 0 {
		return []WorkItem{}, nil
	}

	ids := make([]int, len(result.WorkItems))
	for i, wi := range result.WorkItems {
		ids[i] = wi.ID
	}

	return c.GetWorkItemsBatch(ctx, project, ids)
}

func (c *workItemClient) GetWorkItemsBatch(ctx context.Context, project string, ids []int) ([]WorkItem, error) {
	project = c.resolveProject(project)
	if len(ids) == 0 {
		return []WorkItem{}, nil
	}

	url := fmt.Sprintf("%s/%s/_apis/wit/workitemsbatch?api-version=7.1", c.client.Config.BaseURL, project)
	workItems := make([]WorkItem, 0, len(ids))

	for start := 0; start < len(ids); start += maxWorkItemsBatchSize {
		end := start + maxWorkItemsBatchSize
		if end > len(ids) {
			end = len(ids)
		}

		batchRequest := map[string]interface{}{
			"ids":    ids[start:end],
			"fields": []string{"System.Id", "System.Title", "System.State", "System.AssignedTo", "System.WorkItemType"},
		}

		resp, err := c.client.doRequest(ctx, http.MethodPost, url, batchRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to get work items batch: %w", err)
		}

		var result struct {
			Count int        `json:"count"`
			Value []WorkItem `json:"value"`
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to get work items batch: status %d, body: %s", resp.StatusCode, string(body))
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode work items batch response: %w", err)
		}
		resp.Body.Close()

		workItems = append(workItems, result.Value...)
	}

	return workItems, nil
}

func (c *workItemClient) GetValidStates(ctx context.Context, project string, workItemID int) ([]string, error) {
	project = c.resolveProject(project)
	workItem, err := c.GetWorkItem(ctx, project, workItemID, nil)
	if err != nil {
		return nil, err
	}

	workItemType, ok := workItem.Fields["System.WorkItemType"].(string)
	if !ok || workItemType == "" {
		return nil, fmt.Errorf("work item type not found for work item %d", workItemID)
	}

	url := fmt.Sprintf("%s/%s/_apis/wit/workitemtypes/%s?$expand=all&api-version=7.1", c.client.Config.BaseURL, project, workItemType)

	resp, err := c.client.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get work item type definition: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get work item type definition: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		States []struct {
			Name string `json:"name"`
		} `json:"states"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode work item type response: %w", err)
	}

	states := make([]string, len(result.States))
	for i, state := range result.States {
		states[i] = state.Name
	}

	return states, nil
}

type JsonPatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func escapeWiqlString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
