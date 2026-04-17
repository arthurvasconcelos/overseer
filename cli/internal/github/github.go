package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// CIStatus is the summarized CI check status for a PR.
type CIStatus string

const (
	CIPass    CIStatus = "pass"
	CIFail    CIStatus = "fail"
	CIRunning CIStatus = "running"
	CINone    CIStatus = ""
)

// PR is a minimal representation of a GitHub pull request.
type PR struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	Repo   string   `json:"repo"` // "owner/repo"
	URL    string   `json:"url"`
	Draft  bool     `json:"draft"`
	CI     CIStatus `json:"ci,omitempty"`
}

// Client is a minimal GitHub REST client.
type Client struct {
	token string
	http  *http.Client
}

// New creates a Client using a Personal Access Token.
func New(token string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{Timeout: 10 * time.Second},
	}
}

// MyPRs returns open pull requests that involve the authenticated user
// (authored, assigned, or review requested), sorted by last updated.
// CI status is fetched in parallel for each PR.
func (c *Client) MyPRs(ctx context.Context) ([]PR, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/search/issues?q=is:pr+is:open+involves:@me+archived:false&sort=updated&per_page=30",
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: unexpected status %s", resp.Status)
	}

	var result struct {
		Items []struct {
			Number        int    `json:"number"`
			Title         string `json:"title"`
			HTMLURL       string `json:"html_url"`
			RepositoryURL string `json:"repository_url"`
			Draft         bool   `json:"draft"`
			PullRequest   *struct {
				URL string `json:"url"`
			} `json:"pull_request"` // present on PRs, absent on issues
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("github: decoding response: %w", err)
	}

	prs := make([]PR, 0, len(result.Items))
	for _, item := range result.Items {
		if item.PullRequest == nil {
			continue // filter out plain issues
		}
		prs = append(prs, PR{
			Number: item.Number,
			Title:  item.Title,
			Repo:   repoFromURL(item.RepositoryURL),
			URL:    item.HTMLURL,
			Draft:  item.Draft,
		})
	}

	// Fetch CI status for each PR in parallel.
	var wg sync.WaitGroup
	for i := range prs {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			prs[i].CI = c.prCIStatus(ctx, prs[i].Repo, prs[i].Number)
		}()
	}
	wg.Wait()

	return prs, nil
}

// prCIStatus fetches the CI check status for a single PR.
// Returns CINone on any error so failures don't block the PR listing.
func (c *Client) prCIStatus(ctx context.Context, repo string, number int) CIStatus {
	// Step 1: fetch PR details to get head SHA.
	prURL := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d", repo, number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, prURL, nil)
	if err != nil {
		return CINone
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.http.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return CINone
	}
	defer resp.Body.Close()

	var prDetail struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prDetail); err != nil || prDetail.Head.SHA == "" {
		return CINone
	}

	// Step 2: fetch check-runs for the head SHA.
	return c.checkRunsStatus(ctx, repo, prDetail.Head.SHA)
}

// checkRunsStatus fetches check-runs for a commit SHA and returns a summary status.
func (c *Client) checkRunsStatus(ctx context.Context, repo, sha string) CIStatus {
	url := fmt.Sprintf("https://api.github.com/repos/%s/commits/%s/check-runs?per_page=100", repo, sha)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return CINone
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.http.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return CINone
	}
	defer resp.Body.Close()

	var result struct {
		TotalCount int `json:"total_count"`
		CheckRuns  []struct {
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		} `json:"check_runs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return CINone
	}
	if result.TotalCount == 0 {
		return CINone
	}

	for _, run := range result.CheckRuns {
		if run.Status == "queued" || run.Status == "in_progress" {
			return CIRunning
		}
	}
	for _, run := range result.CheckRuns {
		switch run.Conclusion {
		case "failure", "timed_out", "action_required", "cancelled":
			return CIFail
		}
	}
	return CIPass
}

// Issue is a minimal representation of a GitHub Issue.
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Repo   string `json:"repo"`
	URL    string `json:"url"`
}

// MyIssues returns open GitHub Issues assigned to the authenticated user.
func (c *Client) MyIssues(ctx context.Context) ([]Issue, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/search/issues?q=is:issue+is:open+assignee:@me+archived:false&sort=updated&per_page=30",
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: unexpected status %s", resp.Status)
	}

	var result struct {
		Items []struct {
			Number        int    `json:"number"`
			Title         string `json:"title"`
			HTMLURL       string `json:"html_url"`
			RepositoryURL string `json:"repository_url"`
			PullRequest   *struct {
				URL string `json:"url"`
			} `json:"pull_request"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("github: decoding response: %w", err)
	}

	issues := make([]Issue, 0, len(result.Items))
	for _, item := range result.Items {
		if item.PullRequest != nil {
			continue // filter out PRs
		}
		issues = append(issues, Issue{
			Number: item.Number,
			Title:  item.Title,
			Repo:   repoFromURL(item.RepositoryURL),
			URL:    item.HTMLURL,
		})
	}
	return issues, nil
}

// MergedPRs returns pull requests merged by or involving the authenticated user
// since the given time, sorted by last updated.
func (c *Client) MergedPRs(ctx context.Context, since time.Time) ([]PR, error) {
	sinceStr := since.Format("2006-01-02")
	url := "https://api.github.com/search/issues?q=is:pr+is:merged+involves:@me+archived:false+merged:>=" + sinceStr + "&sort=updated&per_page=50"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: unexpected status %s", resp.Status)
	}

	var result struct {
		Items []struct {
			Number        int    `json:"number"`
			Title         string `json:"title"`
			HTMLURL       string `json:"html_url"`
			RepositoryURL string `json:"repository_url"`
			PullRequest   *struct {
				URL string `json:"url"`
			} `json:"pull_request"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("github: decoding response: %w", err)
	}

	prs := make([]PR, 0, len(result.Items))
	for _, item := range result.Items {
		if item.PullRequest == nil {
			continue
		}
		prs = append(prs, PR{
			Number: item.Number,
			Title:  item.Title,
			Repo:   repoFromURL(item.RepositoryURL),
			URL:    item.HTMLURL,
		})
	}
	return prs, nil
}

// repoFromURL extracts "owner/repo" from a GitHub repository API URL.
// e.g. "https://api.github.com/repos/owner/repo" → "owner/repo"
func repoFromURL(apiURL string) string {
	const prefix = "https://api.github.com/repos/"
	if len(apiURL) > len(prefix) {
		return apiURL[len(prefix):]
	}
	return apiURL
}
