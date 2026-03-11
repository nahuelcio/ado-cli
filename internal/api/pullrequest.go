package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ChangeType string

const (
	ChangeTypeAdd    ChangeType = "add"
	ChangeTypeEdit   ChangeType = "edit"
	ChangeTypeDelete ChangeType = "delete"
	ChangeTypeRename ChangeType = "rename"
	ChangeTypePerm   ChangeType = "permission"
)

type PullRequestStatus string

const (
	PullRequestStatusActive    PullRequestStatus = "active"
	PullRequestStatusAbandoned PullRequestStatus = "abandoned"
	PullRequestStatusCompleted PullRequestStatus = "completed"
	PullRequestStatusNotSet    PullRequestStatus = "notSet"
)

type ThreadStatus string

const (
	ThreadStatusActive  ThreadStatus = "active"
	ThreadStatusFixed   ThreadStatus = "fixed"
	ThreadStatusWontFix ThreadStatus = "wontFix"
	ThreadStatusClosed  ThreadStatus = "closed"
	ThreadStatusPending ThreadStatus = "pending"
)

type CommentType string

const (
	CommentTypeText       CommentType = "text"
	CommentTypeSuggestion CommentType = "suggestion"
)

type ThreadType string

const (
	ThreadTypeBug      ThreadType = "bug"
	ThreadTypeIssue    ThreadType = "issue"
	ThreadTypeQuestion ThreadType = "question"
	ThreadTypeGeneral  ThreadType = "general"
)

type IdentityRef struct {
	DisplayName string `json:"displayName"`
	UniqueName  string `json:"uniqueName"`
	ID          string `json:"id"`
	ImageURL    string `json:"imageUrl,omitempty"`
}

type GitRef struct {
	CommitID     string `json:"commitId"`
	RepositoryID string `json:"repositoryId,omitempty"`
	URL          string `json:"url,omitempty"`
}

type RepositoryRef struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	URL     string      `json:"url"`
	Project *ProjectRef `json:"project,omitempty"`
}

type ProjectRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type GitCommitRef struct {
	CommitID string       `json:"commitId"`
	URL      string       `json:"url,omitempty"`
	Author   *GitUserDate `json:"author,omitempty"`
	Comment  string       `json:"comment,omitempty"`
}

type GitUserDate struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Date  string `json:"date"`
}

type Reviewer struct {
	ReviewerURL *string      `json:"reviewerUrl,omitempty"`
	Vote        int          `json:"vote"`
	VotedBy     *IdentityRef `json:"votedBy,omitempty"`
	IsRequired  *bool        `json:"isRequired,omitempty"`
}

type PullRequest struct {
	PullRequestID         int                `json:"pullRequestId"`
	CodeReviewID          *int               `json:"codeReviewId,omitempty"`
	Status                PullRequestStatus  `json:"status"`
	CreatedBy             *IdentityRef       `json:"createdBy,omitempty"`
	CreationDate          *string            `json:"creationDate,omitempty"`
	Title                 string             `json:"title"`
	Description           *string            `json:"description,omitempty"`
	SourceRefName         *string            `json:"sourceRefName,omitempty"`
	TargetRefName         *string            `json:"targetRefName,omitempty"`
	MergeStatus           *string            `json:"mergeStatus,omitempty"`
	IsDraft               *bool              `json:"isDraft,omitempty"`
	MergeID               *string            `json:"mergeId,omitempty"`
	LastMergeSourceCommit *GitCommitRef      `json:"lastMergeSourceCommit,omitempty"`
	LastMergeTargetCommit *GitCommitRef      `json:"lastMergeTargetCommit,omitempty"`
	LastMergeCommit       *GitCommitRef      `json:"lastMergeCommit,omitempty"`
	Reviewers             []Reviewer         `json:"reviewers,omitempty"`
	URL                   *string            `json:"url,omitempty"`
	SupportsIterations    *bool              `json:"supportsIterations,omitempty"`
	Repository            *RepositoryRef     `json:"repository,omitempty"`
	Links                 map[string]LinkRef `json:"_links,omitempty"`
}

type GitChange struct {
	ChangeID         int                `json:"changeId"`
	ItemID           int                `json:"itemId"`
	Path             string             `json:"path"`
	OriginalPath     *string            `json:"originalPath,omitempty"`
	ChangeType       ChangeType         `json:"changeType"`
	SourceServerItem *string            `json:"sourceServerItem,omitempty"`
	TargetServerItem *string            `json:"targetServerItem,omitempty"`
	SourceVersion    *string            `json:"sourceVersion,omitempty"`
	TargetVersion    *string            `json:"targetVersion,omitempty"`
	SourceEncoding   *string            `json:"sourceEncoding,omitempty"`
	TargetEncoding   *string            `json:"targetEncoding,omitempty"`
	Links            map[string]LinkRef `json:"_links,omitempty"`
}

type PullRequestIteration struct {
	ID              int          `json:"id"`
	FirstCommitID   *string      `json:"firstCommitId,omitempty"`
	LastCommitID    *string      `json:"lastCommitId,omitempty"`
	Author          *IdentityRef `json:"author,omitempty"`
	CreatedDate     *string      `json:"createdDate,omitempty"`
	Description     *string      `json:"description,omitempty"`
	SourceRefCommit *GitRef      `json:"sourceRefCommit,omitempty"`
	TargetRefCommit *GitRef      `json:"targetRefCommit,omitempty"`
	HasMoreChanges  *bool        `json:"hasMoreChanges,omitempty"`
}

type CommentPosition struct {
	Line   int  `json:"line"`
	Offset *int `json:"offset,omitempty"`
}

type ThreadContext struct {
	FilePath         *string           `json:"filePath,omitempty"`
	RightFileStart   *CommentPosition  `json:"rightFileStart,omitempty"`
	RightFileEnd     *CommentPosition  `json:"rightFileEnd,omitempty"`
	LeftFileStart    *CommentPosition  `json:"leftFileStart,omitempty"`
	LeftFileEnd      *CommentPosition  `json:"leftFileEnd,omitempty"`
	TrackingCriteria *TrackingCriteria `json:"trackingCriteria,omitempty"`
}

type TrackingCriteria struct {
	FirstLine *int `json:"firstLine,omitempty"`
	LastLine  *int `json:"lastLine,omitempty"`
}

type SuggestedChange struct {
	OriginalContent *string `json:"originalContent,omitempty"`
	NewContent      *string `json:"newContent,omitempty"`
	Description     *string `json:"description,omitempty"`
}

type ThreadComment struct {
	ID              *int          `json:"id,omitempty"`
	Content         string        `json:"content"`
	CommentType     *CommentType  `json:"commentType,omitempty"`
	Author          *IdentityRef  `json:"author,omitempty"`
	CreatedDate     *string       `json:"createdDate,omitempty"`
	ModifiedDate    *string       `json:"modifiedDate,omitempty"`
	IsDeleted       *bool         `json:"isDeleted,omitempty"`
	ParentCommentID *int          `json:"parentCommentId,omitempty"`
	Reactions       []Reaction    `json:"reactions,omitempty"`
	Likes           []IdentityRef `json:"likes,omitempty"`
}

type Reaction struct {
	ID    int    `json:"id"`
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type PullRequestThread struct {
	ID               int                `json:"id,omitempty"`
	Status           *ThreadStatus      `json:"status,omitempty"`
	ThreadContext    *ThreadContext     `json:"threadContext,omitempty"`
	Comments         []ThreadComment    `json:"comments"`
	IsDeleted        *bool              `json:"isDeleted,omitempty"`
	ThreadType       *ThreadType        `json:"threadType,omitempty"`
	Links            map[string]LinkRef `json:"_links,omitempty"`
	SuggestedChanges []SuggestedChange  `json:"suggestedChanges,omitempty"`
}

type PullRequestThreadSummary struct {
	ID              int            `json:"id"`
	Status          ThreadStatus   `json:"status"`
	ThreadContext   *ThreadContext `json:"threadContext,omitempty"`
	CommentCount    int            `json:"commentCount"`
	FirstComment    *ThreadComment `json:"firstComment,omitempty"`
	LastUpdatedDate *string        `json:"lastUpdatedDate,omitempty"`
	Participants    []IdentityRef  `json:"participants,omitempty"`
}

type PullRequestComment struct {
	ID              int          `json:"id"`
	Content         string       `json:"content"`
	CommentType     CommentType  `json:"commentType"`
	Author          *IdentityRef `json:"author,omitempty"`
	CreatedDate     string       `json:"createdDate"`
	ModifiedDate    *string      `json:"modifiedDate,omitempty"`
	IsDeleted       *bool        `json:"isDeleted,omitempty"`
	ParentCommentID *int         `json:"parentCommentId,omitempty"`
}

type ChangeDetails struct {
	Path         string     `json:"path"`
	ChangeType   ChangeType `json:"changeType"`
	AddedLines   int        `json:"addedLines"`
	RemovedLines int        `json:"removedLines"`
	Content      *struct {
		OriginalContent *string `json:"originalContent,omitempty"`
		NewContent      *string `json:"newContent,omitempty"`
		Diff            *string `json:"diff,omitempty"`
	} `json:"content,omitempty"`
}

type IterationChangeEntry struct {
	ChangeID         int        `json:"changeId"`
	ItemID           string     `json:"itemId"`
	Path             string     `json:"path"`
	OriginalPath     *string    `json:"originalPath,omitempty"`
	ChangeType       ChangeType `json:"changeType"`
	SourceServerItem *string    `json:"sourceServerItem,omitempty"`
	TargetServerItem *string    `json:"targetServerItem,omitempty"`
	SourceVersion    *string    `json:"sourceVersion,omitempty"`
	TargetVersion    *string    `json:"targetVersion,omitempty"`
	SourceEncoding   *string    `json:"sourceEncoding,omitempty"`
	TargetEncoding   *string    `json:"targetEncoding,omitempty"`
	IsBinary         *bool      `json:"isBinary,omitempty"`
	Content          *struct {
		OriginalContent *string `json:"originalContent,omitempty"`
		NewContent      *string `json:"newContent,omitempty"`
	} `json:"content,omitempty"`
	URL   *string            `json:"url,omitempty"`
	Links map[string]LinkRef `json:"_links,omitempty"`
}

type IterationChangesResponse struct {
	ChangeEntries          []IterationChangeEntry `json:"changeEntries"`
	CommonAncestorCommit   *string                `json:"commonAncestorCommit,omitempty"`
	CommonAncestorCommitID *string                `json:"commonAncestorCommitId,omitempty"`
	BaseIteration          *IterationRef          `json:"baseIteration,omitempty"`
	TargetIteration        *IterationRef          `json:"targetIteration,omitempty"`
	HasMoreChanges         *bool                  `json:"hasMoreChanges,omitempty"`
}

type IterationRef struct {
	ID            int     `json:"id"`
	FirstCommitID *string `json:"firstCommitId,omitempty"`
	LastCommitID  *string `json:"lastCommitId,omitempty"`
}

type DetailedChange struct {
	ChangeID      int          `json:"changeId"`
	ItemID        string       `json:"itemId"`
	Path          string       `json:"path"`
	OriginalPath  *string      `json:"originalPath,omitempty"`
	ChangeType    ChangeType   `json:"changeType"`
	AddedLines    int          `json:"addedLines"`
	RemovedLines  int          `json:"removedLines"`
	IsBinary      bool         `json:"isBinary"`
	IsTruncated   bool         `json:"isTruncated"`
	Diff          *DiffContent `json:"diff,omitempty"`
	SourceVersion *string      `json:"sourceVersion,omitempty"`
	TargetVersion *string      `json:"targetVersion,omitempty"`
}

type DiffContent struct {
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

type FileChangeSummary struct {
	Path         string `json:"path"`
	ChangeType   string `json:"changeType"`
	AddedLines   int    `json:"addedLines"`
	RemovedLines int    `json:"removedLines"`
}

type PRSummary struct {
	PullRequestID int                 `json:"pullRequestId"`
	Title         string              `json:"title"`
	Status        string              `json:"status"`
	SourceBranch  string              `json:"sourceBranch"`
	TargetBranch  string              `json:"targetBranch"`
	TotalChanges  int                 `json:"totalChanges"`
	Files         []FileChangeSummary `json:"files"`
	Summary       ChangeSummary       `json:"summary"`
}

type ChangeSummary struct {
	TotalAddedLines   int `json:"totalAddedLines"`
	TotalRemovedLines int `json:"totalRemovedLines"`
	FilesAdded        int `json:"filesAdded"`
	FilesEdited       int `json:"filesEdited"`
	FilesDeleted      int `json:"filesDeleted"`
}

type PullRequestClient interface {
	ListPullRequests(ctx context.Context, project, repo string, status PullRequestStatus) ([]PullRequest, error)
	GetPullRequest(ctx context.Context, project, repo string, prID int) (*PullRequest, error)
	GetPullRequestChanges(ctx context.Context, project, repo string, prID int) ([]GitChange, error)
	GetPullRequestIterations(ctx context.Context, project, repo string, prID int) ([]PullRequestIteration, error)
	GetIterationChanges(ctx context.Context, project, repo string, prID, iterationID int) ([]GitChange, error)
	GetThreads(ctx context.Context, project, repo string, prID int) ([]PullRequestThreadSummary, error)
	CreateThread(ctx context.Context, project, repo string, prID int, thread *PullRequestThread) (*PullRequestThread, error)
	PostComment(ctx context.Context, project, repo string, prID, threadID int, comment string) (*PullRequestComment, error)
	GetPullRequestSummary(ctx context.Context, project, repo string, prID int) (*PRSummary, error)
}

type pullRequestClient struct {
	client *AzureDevOpsClient
}

func NewPullRequestClient(client *AzureDevOpsClient) PullRequestClient {
	return &pullRequestClient{
		client: client,
	}
}

func (c *pullRequestClient) ListPullRequests(ctx context.Context, project, repo string, status PullRequestStatus) ([]PullRequest, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests?searchCriteria.status=%s&api-version=7.0", c.client.Config.BaseURL, project, repo, status)

	resp, err := c.client.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull requests: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get pull requests: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Count int           `json:"count"`
		Value []PullRequest `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode pull requests response: %w", err)
	}

	return result.Value, nil
}

func (c *pullRequestClient) GetPullRequest(ctx context.Context, project, repo string, prID int) (*PullRequest, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests/%d?api-version=7.0", c.client.Config.BaseURL, project, repo, prID)

	resp, err := c.client.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request %d: %w", prID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get pull request %d: status %d, body: %s", prID, resp.StatusCode, string(body))
	}

	var result PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode pull request response: %w", err)
	}

	return &result, nil
}

func (c *pullRequestClient) GetPullRequestChanges(ctx context.Context, project, repo string, prID int) ([]GitChange, error) {
	iterations, err := c.GetPullRequestIterations(ctx, project, repo, prID)
	if err != nil {
		return nil, err
	}

	if len(iterations) == 0 {
		return []GitChange{}, nil
	}

	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests/%d/iterations/%d/changes?api-version=7.0",
		c.client.Config.BaseURL, project, repo, prID, iterations[len(iterations)-1].ID)

	resp, err := c.client.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request changes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get pull request changes: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ChangeEntries []IterationChangeEntry `json:"changeEntries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode changes response: %w", err)
	}

	// Convert IterationChangeEntry to GitChange
	changes := make([]GitChange, len(result.ChangeEntries))
	for i, entry := range result.ChangeEntries {
		changes[i] = GitChange{
			ChangeID:         entry.ChangeID,
			Path:             entry.Path,
			OriginalPath:     entry.OriginalPath,
			ChangeType:       entry.ChangeType,
			SourceServerItem: entry.SourceServerItem,
			TargetServerItem: entry.TargetServerItem,
			SourceVersion:    entry.SourceVersion,
			TargetVersion:    entry.TargetVersion,
			SourceEncoding:   entry.SourceEncoding,
			TargetEncoding:   entry.TargetEncoding,
			Links:            entry.Links,
		}
		// ItemID from IterationChangeEntry is a string (ObjectID), GitChange expects int
		// We'll leave it as 0 since it's not a numeric ID
	}

	return changes, nil
}

func (c *pullRequestClient) GetPullRequestIterations(ctx context.Context, project, repo string, prID int) ([]PullRequestIteration, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests/%d/iterations?api-version=7.0", c.client.Config.BaseURL, project, repo, prID)

	resp, err := c.client.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request iterations: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get pull request iterations: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Count int                    `json:"count"`
		Value []PullRequestIteration `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode iterations response: %w", err)
	}

	return result.Value, nil
}

func (c *pullRequestClient) GetIterationChanges(ctx context.Context, project, repo string, prID, iterationID int) ([]GitChange, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests/%d/iterations/%d/changes?api-version=7.0",
		c.client.Config.BaseURL, project, repo, prID, iterationID)

	resp, err := c.client.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get iteration changes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get iteration changes: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ChangeEntries []IterationChangeEntry `json:"changeEntries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode changes response: %w", err)
	}

	// Convert IterationChangeEntry to GitChange
	changes := make([]GitChange, len(result.ChangeEntries))
	for i, entry := range result.ChangeEntries {
		changes[i] = GitChange{
			ChangeID:         entry.ChangeID,
			Path:             entry.Path,
			OriginalPath:     entry.OriginalPath,
			ChangeType:       entry.ChangeType,
			SourceServerItem: entry.SourceServerItem,
			TargetServerItem: entry.TargetServerItem,
			SourceVersion:    entry.SourceVersion,
			TargetVersion:    entry.TargetVersion,
			SourceEncoding:   entry.SourceEncoding,
			TargetEncoding:   entry.TargetEncoding,
			Links:            entry.Links,
		}
	}

	return changes, nil
}

func (c *pullRequestClient) GetThreads(ctx context.Context, project, repo string, prID int) ([]PullRequestThreadSummary, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests/%d/threads?api-version=7.0", c.client.Config.BaseURL, project, repo, prID)

	resp, err := c.client.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request threads: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get pull request threads: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Count int                 `json:"count"`
		Value []PullRequestThread `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode threads response: %w", err)
	}

	summaries := make([]PullRequestThreadSummary, len(result.Value))
	for i, thread := range result.Value {
		summaries[i] = PullRequestThreadSummary{
			ID:            thread.ID,
			Status:        *thread.Status,
			ThreadContext: thread.ThreadContext,
			CommentCount:  len(thread.Comments),
		}
		if len(thread.Comments) > 0 {
			summaries[i].FirstComment = &thread.Comments[0]
		}
	}

	return summaries, nil
}

func (c *pullRequestClient) CreateThread(ctx context.Context, project, repo string, prID int, thread *PullRequestThread) (*PullRequestThread, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests/%d/threads?api-version=7.0", c.client.Config.BaseURL, project, repo, prID)

	if thread.ThreadContext != nil && thread.ThreadContext.FilePath != nil {
		fp := *thread.ThreadContext.FilePath
		if !strings.HasPrefix(fp, "/") {
			fp = "/" + fp
			thread.ThreadContext.FilePath = &fp
		}
	}

	comments := make([]map[string]interface{}, len(thread.Comments))
	for i, comment := range thread.Comments {
		comments[i] = map[string]interface{}{
			"content": comment.Content,
		}
		if comment.CommentType != nil {
			comments[i]["commentType"] = *comment.CommentType
		}
	}

	threadData := map[string]interface{}{
		"comments": comments,
	}
	if thread.Status != nil {
		threadData["status"] = *thread.Status
	}
	if thread.ThreadContext != nil {
		threadData["threadContext"] = thread.ThreadContext
	}

	resp, err := c.client.doRequest(ctx, http.MethodPost, url, threadData)
	if err != nil {
		return nil, fmt.Errorf("failed to create thread: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create thread: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result PullRequestThread
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode thread response: %w", err)
	}

	return &result, nil
}

func (c *pullRequestClient) PostComment(ctx context.Context, project, repo string, prID, threadID int, comment string) (*PullRequestComment, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests/%d/threads/%d/comments?api-version=7.0",
		c.client.Config.BaseURL, project, repo, prID, threadID)

	commentData := map[string]interface{}{
		"content":     comment,
		"commentType": CommentTypeText,
	}

	resp, err := c.client.doRequest(ctx, http.MethodPost, url, commentData)
	if err != nil {
		return nil, fmt.Errorf("failed to post comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to post comment: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result PullRequestComment
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode comment response: %w", err)
	}

	return &result, nil
}

func (c *pullRequestClient) GetPullRequestSummary(ctx context.Context, project, repo string, prID int) (*PRSummary, error) {
	pr, err := c.GetPullRequest(ctx, project, repo, prID)
	if err != nil {
		return nil, err
	}

	changes, err := c.GetPullRequestChanges(ctx, project, repo, prID)
	if err != nil {
		return nil, err
	}

	files := make([]FileChangeSummary, len(changes))
	filesAdded := 0
	filesEdited := 0
	filesDeleted := 0
	totalAdded := 0
	totalRemoved := 0

	for i, change := range changes {
		changeTypeStr := string(change.ChangeType)
		files[i] = FileChangeSummary{
			Path:         change.Path,
			ChangeType:   changeTypeStr,
			AddedLines:   0,
			RemovedLines: 0,
		}

		switch change.ChangeType {
		case ChangeTypeAdd:
			filesAdded++
		case ChangeTypeEdit:
			filesEdited++
		case ChangeTypeDelete:
			filesDeleted++
		}
	}

	sourceBranch := ""
	if pr.SourceRefName != nil {
		sourceBranch = *pr.SourceRefName
	}

	targetBranch := ""
	if pr.TargetRefName != nil {
		targetBranch = *pr.TargetRefName
	}

	return &PRSummary{
		PullRequestID: pr.PullRequestID,
		Title:         pr.Title,
		Status:        string(pr.Status),
		SourceBranch:  sourceBranch,
		TargetBranch:  targetBranch,
		TotalChanges:  len(changes),
		Files:         files,
		Summary: ChangeSummary{
			TotalAddedLines:   totalAdded,
			TotalRemovedLines: totalRemoved,
			FilesAdded:        filesAdded,
			FilesEdited:       filesEdited,
			FilesDeleted:      filesDeleted,
		},
	}, nil
}
