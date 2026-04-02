package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	endpoint := fmt.Sprintf("%s/rest/api/3/search/jql", c.baseURL)

	body, err := json.Marshal(map[string]any{
		"jql":        "assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC",
		"fields":     []string{"summary", "status", "priority"},
		"maxResults": 20,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Content-Type", "application/json")
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
