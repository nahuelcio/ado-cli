package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type GitRepository struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	URL           string      `json:"url"`
	RemoteURL     string      `json:"remoteUrl,omitempty"`
	SSHURL        string      `json:"sshUrl,omitempty"`
	WebURL        string      `json:"webUrl,omitempty"`
	DefaultBranch string      `json:"defaultBranch,omitempty"`
	Project       *ProjectRef `json:"project,omitempty"`
	Size          int64       `json:"size,omitempty"`
}

type RepositoryClient interface {
	ListRepositories(ctx context.Context, project string) ([]GitRepository, error)
	GetRepository(ctx context.Context, project, repoName string) (*GitRepository, error)
}

type repositoryClient struct {
	client *AzureDevOpsClient
}

func NewRepositoryClient(client *AzureDevOpsClient) RepositoryClient {
	return &repositoryClient{
		client: client,
	}
}

func (c *repositoryClient) ListRepositories(ctx context.Context, project string) ([]GitRepository, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories?api-version=7.0", c.client.Config.BaseURL, project)

	resp, err := c.client.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list repositories: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Count int             `json:"count"`
		Value []GitRepository `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode repositories response: %w", err)
	}

	return result.Value, nil
}

func (c *repositoryClient) GetRepository(ctx context.Context, project, repoName string) (*GitRepository, error) {
	url := fmt.Sprintf("%s/%s/_apis/git/repositories/%s?api-version=7.0", c.client.Config.BaseURL, project, repoName)

	resp, err := c.client.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository %s: %w", repoName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get repository %s: status %d, body: %s", repoName, resp.StatusCode, string(body))
	}

	var result GitRepository
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode repository response: %w", err)
	}

	return &result, nil
}
