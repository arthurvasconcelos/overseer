package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PR is a minimal representation of a GitHub pull request.
type PR struct {
	Number int
	Title  string
	Repo   string // "owner/repo"
	URL    string
	Draft  bool
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
			Number          int    `json:"number"`
			Title           string `json:"title"`
			HTMLURL         string `json:"html_url"`
			RepositoryURL   string `json:"repository_url"`
			Draft           bool   `json:"draft"`
			PullRequest     *struct{} `json:"pull_request"` // present on PRs, absent on issues
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
