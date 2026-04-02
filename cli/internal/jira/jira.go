package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client is a minimal Jira Cloud REST client using Basic auth.
type Client struct {
	baseURL string
	email   string
	token   string
	http    *http.Client
}

// New creates a Jira client for the given instance.
func New(baseURL, email, token string) *Client {
	return &Client{
		baseURL: baseURL,
		email:   email,
		token:   token,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Issue is a minimal representation of a Jira issue.
type Issue struct {
	Key    string
	Summary string
	Status  string
	Priority string
}

type searchResponse struct {
	Issues []struct {
		Key    string `json:"key"`
		Fields struct {
			Summary  string `json:"summary"`
			Status   struct{ Name string `json:"name"` } `json:"status"`
			Priority struct{ Name string `json:"name"` } `json:"priority"`
		} `json:"fields"`
	} `json:"issues"`
}

// MyIssues returns unresolved issues assigned to the current user.
func (c *Client) MyIssues(ctx context.Context) ([]Issue, error) {
	jql := "assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC"
	endpoint := fmt.Sprintf("%s/rest/api/3/search?jql=%s&fields=summary,status,priority&maxResults=20",
		c.baseURL, url.QueryEscape(jql))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira: unexpected status %s", resp.Status)
	}

	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("jira: decoding response: %w", err)
	}

	issues := make([]Issue, 0, len(result.Issues))
	for _, i := range result.Issues {
		issues = append(issues, Issue{
			Key:      i.Key,
			Summary:  i.Fields.Summary,
			Status:   i.Fields.Status.Name,
			Priority: i.Fields.Priority.Name,
		})
	}
	return issues, nil
}
