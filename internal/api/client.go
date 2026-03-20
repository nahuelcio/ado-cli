package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type ConnectionConfig struct {
	Organization string
	Project      string
	BaseURL      string
	Token        string
}

type RateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
}

type AzureDevOpsClient struct {
	Config      ConnectionConfig
	RateLimiter *RateLimiter
	httpClient  *http.Client
	authHeader  string
}

type Project struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	State          string `json:"state"`
	Revision       int    `json:"revision"`
	Visibility     string `json:"visibility,omitempty"`
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
}

type ProjectsResponse struct {
	Count int       `json:"count"`
	Value []Project `json:"value"`
}

func NewConnectionConfig(organization, project, token string) ConnectionConfig {
	organization = strings.TrimSpace(strings.TrimSuffix(organization, "/"))
	baseURL := fmt.Sprintf("https://dev.azure.com/%s", organization)

	if parsedURL, err := url.Parse(organization); err == nil && parsedURL.Host != "" {
		host := strings.ToLower(parsedURL.Hostname())
		path := strings.Trim(parsedURL.Path, "/")

		switch {
		case host == "dev.azure.com":
			parts := strings.Split(path, "/")
			if len(parts) > 0 && parts[0] != "" {
				organization = parts[0]
				baseURL = fmt.Sprintf("https://dev.azure.com/%s", organization)
			}
			if len(parts) > 1 && parts[1] != "" && project == "" {
				project = parts[1]
			}
		case strings.HasSuffix(host, ".visualstudio.com"):
			organization = strings.TrimSuffix(host, ".visualstudio.com")
			baseURL = "https://" + host
			if path != "" && project == "" {
				project = strings.Split(path, "/")[0]
			}
		}
	} else {
		org, proj, err := parseAzureDevOpsURL(organization)
		if err == nil && org != "" {
			organization = org
			baseURL = fmt.Sprintf("https://dev.azure.com/%s", organization)
			if proj != "" && project == "" {
				project = proj
			}
		}
	}

	return ConnectionConfig{
		Organization: organization,
		Project:      project,
		BaseURL:      baseURL,
		Token:        token,
	}
}

func NewAzureDevOpsClient(config ConnectionConfig) (*AzureDevOpsClient, error) {
	client := &AzureDevOpsClient{
		Config:      config,
		RateLimiter: NewRateLimiter(100, 100),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		authHeader: encodePAT(config.Token),
	}

	return client, nil
}

func (c *AzureDevOpsClient) GetWorkItemClient() WorkItemClient {
	return NewWorkItemClient(c)
}

func (c *AzureDevOpsClient) GetPullRequestClient() PullRequestClient {
	return NewPullRequestClient(c)
}

func (c *AzureDevOpsClient) GetRepositoryClient() RepositoryClient {
	return NewRepositoryClient(c)
}

func (c *AzureDevOpsClient) GetProjects(ctx context.Context) ([]Project, error) {
	url := fmt.Sprintf("%s/_apis/projects?api-version=7.1", c.Config.BaseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Basic "+c.authHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get projects: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get projects: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result ProjectsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode projects response: %w", err)
	}

	return result.Value, nil
}

func (c *AzureDevOpsClient) GetProject(ctx context.Context, projectName string) (*Project, error) {
	url := fmt.Sprintf("%s/_apis/projects/%s?api-version=7.1", c.Config.BaseURL, projectName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Basic "+c.authHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", projectName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get project %s: status %d, body: %s", projectName, resp.StatusCode, string(body))
	}

	var result Project
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode project response: %w", err)
	}

	return &result, nil
}

func (c *AzureDevOpsClient) ValidateConnection(ctx context.Context) error {
	_, err := c.GetProjects(ctx)
	if err != nil {
		return fmt.Errorf("connection validation failed: %w", err)
	}
	return nil
}

func (c *AzureDevOpsClient) doRequest(ctx context.Context, method, url string, body interface{}) (*http.Response, error) {
	if err := c.RateLimiter.WaitForToken(ctx, 1); err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Basic "+c.authHeader)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

func NewRateLimiter(maxTokens float64, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (r *RateLimiter) Acquire(tokens float64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refill()

	if r.tokens >= tokens {
		r.tokens -= tokens
		return true
	}

	return false
}

func (r *RateLimiter) WaitForToken(ctx context.Context, tokens float64) error {
	for {
		if r.Acquire(tokens) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (r *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()

	refillAmount := elapsed * r.refillRate
	r.tokens = min(r.maxTokens, r.tokens+refillAmount)
	r.lastRefill = now
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

type Authenticator interface {
	Authenticate(ctx context.Context) (map[string]string, error)
}

type PersonalAccessTokenAuth struct {
	Token string
}

func NewPersonalAccessTokenAuth(token string) *PersonalAccessTokenAuth {
	return &PersonalAccessTokenAuth{
		Token: token,
	}
}

func (a *PersonalAccessTokenAuth) Authenticate(ctx context.Context) (map[string]string, error) {
	return map[string]string{
		"Authorization": "Basic " + encodePAT(a.Token),
	}, nil
}

func encodePAT(pat string) string {
	auth := ":" + pat
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

type ClientFactory struct {
	mu          sync.RWMutex
	clients     map[string]*AzureDevOpsClient
	rateLimiter *RateLimiter
}

var globalClientFactory *ClientFactory

func InitClientFactory() *ClientFactory {
	globalClientFactory = &ClientFactory{
		clients:     make(map[string]*AzureDevOpsClient),
		rateLimiter: NewRateLimiter(100, 100),
	}
	return globalClientFactory
}

func GetClientFactory() *ClientFactory {
	if globalClientFactory == nil {
		return InitClientFactory()
	}
	return globalClientFactory
}

func (f *ClientFactory) GetClient(org, project, token string) (*AzureDevOpsClient, error) {
	key := fmt.Sprintf("%s/%s", org, project)

	f.mu.RLock()
	if client, exists := f.clients[key]; exists {
		f.mu.RUnlock()
		return client, nil
	}
	f.mu.RUnlock()

	config := NewConnectionConfig(org, project, token)
	client, err := NewAzureDevOpsClient(config)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	f.clients[key] = client
	f.mu.Unlock()

	return client, nil
}

func (f *ClientFactory) ClearClients() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clients = make(map[string]*AzureDevOpsClient)
}

func GetWorkItemClient(ctx context.Context, organization, project, token string) (WorkItemClient, error) {
	factory := GetClientFactory()
	client, err := factory.GetClient(organization, project, token)
	if err != nil {
		return nil, err
	}
	return client.GetWorkItemClient(), nil
}

func GetPullRequestClient(ctx context.Context, organization, project, token string) (PullRequestClient, error) {
	factory := GetClientFactory()
	client, err := factory.GetClient(organization, project, token)
	if err != nil {
		return nil, err
	}
	return client.GetPullRequestClient(), nil
}

func GetRepositoryClient(ctx context.Context, organization, project, token string) (RepositoryClient, error) {
	factory := GetClientFactory()
	client, err := factory.GetClient(organization, project, token)
	if err != nil {
		return nil, err
	}
	return client.GetRepositoryClient(), nil
}

func parseAzureDevOpsURL(input string) (organization, project string, err error) {
	if strings.Contains(input, "/") && !strings.HasPrefix(input, "http") {
		parts := strings.SplitN(input, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}

	if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
		input = strings.TrimPrefix(input, "https://")
		input = strings.TrimPrefix(input, "http://")
		parts := strings.SplitN(input, "/", 3)
		if len(parts) >= 2 {
			if parts[0] == "dev.azure.com" {
				if len(parts) >= 3 {
					return parts[1], parts[2], nil
				}
				return parts[1], "", nil
			}
			if strings.HasSuffix(parts[0], ".visualstudio.com") {
				org := strings.TrimSuffix(parts[0], ".visualstudio.com")
				if len(parts) >= 2 {
					return org, parts[1], nil
				}
				return org, "", nil
			}
		}
	}

	return input, "", nil
}
