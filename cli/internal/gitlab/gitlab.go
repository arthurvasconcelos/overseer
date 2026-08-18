package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://gitlab.com"

// pipelineCIStatus maps a GitLab pipeline status string to CIStatus.
func pipelineCIStatus(p *struct {
	Status string `json:"status"`
}) CIStatus {
	if p == nil || p.Status == "" {
		return CINone
	}
	switch p.Status {
	case "success":
		return CIPass
	case "failed":
		return CIFail
	case "running", "pending", "created", "preparing", "waiting_for_resource", "scheduled":
		return CIRunning
	default:
		return CINone
	}
}

// CIStatus is the summarized CI pipeline status for an MR.
type CIStatus string

const (
	CIPass    CIStatus = "pass"
	CIFail    CIStatus = "fail"
	CIRunning CIStatus = "running"
	CINone    CIStatus = ""
)

// MR is a minimal representation of a GitLab merge request.
type MR struct {
	IID     int      `json:"iid"` // merge request IID within the project
	Title   string   `json:"title"`
	Project string   `json:"project"` // "namespace/project"
	URL     string   `json:"url"`
	Draft   bool     `json:"draft"`
	Status  string   `json:"status"` // "can_be_merged", "cannot_be_merged", "checking", ""
	CI      CIStatus `json:"ci,omitempty"`
}

// Client is a minimal GitLab REST client.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New creates a Client. baseURL defaults to https://gitlab.com if empty.
func New(baseURL, token string) *Client {
	return NewWithTimeout(baseURL, token, 10*time.Second)
}

// NewWithTimeout creates a Client with a custom HTTP timeout. The global
// merged-MR search is markedly slower than per-project queries and needs a
// longer budget than interactive commands want to wait.
func NewWithTimeout(baseURL, token string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: timeout},
	}
}

// Issue is a minimal representation of a GitLab issue.
type Issue struct {
	IID     int    `json:"iid"`
	Title   string `json:"title"`
	Project string `json:"project"` // "namespace/project"
	URL     string `json:"url"`
}

// MyIssues returns open issues assigned to the authenticated user.
func (c *Client) MyIssues(ctx context.Context) ([]Issue, error) {
	url := fmt.Sprintf("%s/api/v4/issues?state=opened&scope=assigned_to_me&per_page=50", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab: unexpected status %s", resp.Status)
	}

	var items []struct {
		IID        int    `json:"iid"`
		Title      string `json:"title"`
		WebURL     string `json:"web_url"`
		References struct {
			Full string `json:"full"`
		} `json:"references"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("gitlab: decoding response: %w", err)
	}

	issues := make([]Issue, 0, len(items))
	for _, item := range items {
		project := item.References.Full
		if i := strings.LastIndex(project, "#"); i >= 0 {
			project = project[:i]
		}
		issues = append(issues, Issue{
			IID:     item.IID,
			Title:   item.Title,
			Project: project,
			URL:     item.WebURL,
		})
	}
	return issues, nil
}

// MyMRs returns open merge requests created by or assigned to the authenticated user.
func (c *Client) MyMRs(ctx context.Context) ([]MR, error) {
	seen := make(map[int]bool)
	var mrs []MR

	for _, scope := range []string{"created_by_me", "assigned_to_me"} {
		page, err := c.fetchMRs(ctx, scope)
		if err != nil {
			return nil, err
		}
		for _, mr := range page {
			if !seen[mr.IID] {
				seen[mr.IID] = true
				mrs = append(mrs, mr)
			}
		}
	}
	return mrs, nil
}

// MergedMRs returns merge requests merged since the given time, created by or
// assigned to the authenticated user, deduplicated by IID.
func (c *Client) MergedMRs(ctx context.Context, since time.Time) ([]MR, error) {
	return c.MergedBetween(ctx, since, time.Time{})
}

// MergedBetween returns merge requests merged in a window. A zero until means
// "no upper bound".
func (c *Client) MergedBetween(ctx context.Context, since, until time.Time) ([]MR, error) {
	seen := make(map[int]bool)
	var mrs []MR
	for _, scope := range []string{"created_by_me", "assigned_to_me"} {
		page, err := c.fetchMergedMRs(ctx, scope, since, until)
		if err != nil {
			return nil, err
		}
		for _, mr := range page {
			if !seen[mr.IID] {
				seen[mr.IID] = true
				mrs = append(mrs, mr)
			}
		}
	}
	return mrs, nil
}

func (c *Client) fetchMergedMRs(ctx context.Context, scope string, since, until time.Time) ([]MR, error) {
	mergedAfter := since.UTC().Format("2006-01-02T15:04:05Z")
	url := fmt.Sprintf("%s/api/v4/merge_requests?state=merged&scope=%s&merged_after=%s&per_page=50", c.baseURL, scope, mergedAfter)
	if !until.IsZero() {
		url += "&merged_before=" + until.UTC().Format("2006-01-02T15:04:05Z")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab: unexpected status %s", resp.Status)
	}

	var items []struct {
		IID        int    `json:"iid"`
		Title      string `json:"title"`
		WebURL     string `json:"web_url"`
		References struct {
			Full string `json:"full"`
		} `json:"references"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("gitlab: decoding response: %w", err)
	}

	mrs := make([]MR, 0, len(items))
	for _, item := range items {
		project := item.References.Full
		if i := strings.LastIndex(project, "!"); i >= 0 {
			project = project[:i]
		}
		mrs = append(mrs, MR{
			IID:     item.IID,
			Title:   item.Title,
			Project: project,
			URL:     item.WebURL,
		})
	}
	return mrs, nil
}

func (c *Client) fetchMRs(ctx context.Context, scope string) ([]MR, error) {
	url := fmt.Sprintf("%s/api/v4/merge_requests?state=opened&scope=%s&per_page=30", c.baseURL, scope)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab: unexpected status %s", resp.Status)
	}

	var items []struct {
		IID         int    `json:"iid"`
		Title       string `json:"title"`
		WebURL      string `json:"web_url"`
		Draft       bool   `json:"draft"`
		MergeStatus string `json:"merge_status"`
		References  struct {
			Full string `json:"full"` // "namespace/project!IID"
		} `json:"references"`
		Project struct {
			PathWithNamespace string `json:"path_with_namespace"`
		} `json:"project"`
		HeadPipeline *struct {
			Status string `json:"status"`
		} `json:"head_pipeline"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("gitlab: decoding response: %w", err)
	}

	mrs := make([]MR, 0, len(items))
	for _, item := range items {
		// references.full is "namespace/project!IID" — strip the "!IID" suffix.
		project := item.References.Full
		if i := strings.LastIndex(project, "!"); i >= 0 {
			project = project[:i]
		}
		mrs = append(mrs, MR{
			IID:     item.IID,
			Title:   item.Title,
			Project: project,
			URL:     item.WebURL,
			Draft:   item.Draft,
			Status:  item.MergeStatus,
			CI:      pipelineCIStatus(item.HeadPipeline),
		})
	}
	return mrs, nil
}

// CurrentUsername returns the username of the token's owner.
func (c *Client) CurrentUsername(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v4/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("gitlab: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gitlab: unexpected status %s", resp.Status)
	}

	var user struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", fmt.Errorf("gitlab: decoding response: %w", err)
	}
	return user.Username, nil
}

// MergedInProject returns merge requests merged in one project within a window.
// The project-scoped query is far cheaper than the instance-wide search, which
// gitlab.com answers with 408 for accounts with many projects.
func (c *Client) MergedInProject(ctx context.Context, project, author string, since, until time.Time) ([]MR, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests?state=merged&merged_after=%s&per_page=50",
		c.baseURL,
		neturl.PathEscape(project),
		since.UTC().Format("2006-01-02T15:04:05Z"))
	if !until.IsZero() {
		url += "&merged_before=" + until.UTC().Format("2006-01-02T15:04:05Z")
	}
	if author != "" {
		url += "&author_username=" + neturl.QueryEscape(author)
	}

	return c.fetchProjectMRs(ctx, url, project)
}

// OpenedInProject returns merge requests in one project created within a
// window and still open — the work-in-progress a daily note cares about.
func (c *Client) OpenedInProject(ctx context.Context, project, author string, since, until time.Time) ([]MR, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests?state=opened&created_after=%s&per_page=50",
		c.baseURL,
		neturl.PathEscape(project),
		since.UTC().Format("2006-01-02T15:04:05Z"))
	if !until.IsZero() {
		url += "&created_before=" + until.UTC().Format("2006-01-02T15:04:05Z")
	}
	if author != "" {
		url += "&author_username=" + neturl.QueryEscape(author)
	}
	return c.fetchProjectMRs(ctx, url, project)
}

func (c *Client) fetchProjectMRs(ctx context.Context, url, project string) ([]MR, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab: unexpected status %s", resp.Status)
	}

	var items []struct {
		IID        int    `json:"iid"`
		Title      string `json:"title"`
		WebURL     string `json:"web_url"`
		Draft      bool   `json:"draft"`
		References struct {
			Full string `json:"full"`
		} `json:"references"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("gitlab: decoding response: %w", err)
	}

	mrs := make([]MR, 0, len(items))
	for _, item := range items {
		name := item.References.Full
		if i := strings.LastIndex(name, "!"); i >= 0 {
			name = name[:i]
		}
		if name == "" {
			name = project
		}
		mrs = append(mrs, MR{IID: item.IID, Title: item.Title, Project: name, URL: item.WebURL, Draft: item.Draft})
	}
	return mrs, nil
}
