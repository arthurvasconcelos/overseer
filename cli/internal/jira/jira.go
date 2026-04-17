package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// Ping verifies credentials by calling /rest/api/3/myself.
// Returns the authenticated user's email address.
func (c *Client) Ping(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/rest/api/3/myself", nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("jira: ping: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("jira: ping: unexpected status %s", resp.Status)
	}

	var user struct {
		EmailAddress string `json:"emailAddress"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", fmt.Errorf("jira: ping: decoding response: %w", err)
	}
	return user.EmailAddress, nil
}

// RecentlyUpdated returns issues whose status transitioned to any of the given
// statuses since the provided time. Used by the standup generator.
func (c *Client) RecentlyUpdated(ctx context.Context, statuses []string, since time.Time) ([]Issue, error) {
	sinceStr := since.Format("2006/01/02 15:04")
	statusList := make([]string, len(statuses))
	for i, s := range statuses {
		statusList[i] = fmt.Sprintf("%q", s)
	}
	jql := fmt.Sprintf(
		`assignee = currentUser() AND status in (%s) AND updated >= "%s" ORDER BY updated DESC`,
		strings.Join(statusList, ", "),
		sinceStr,
	)

	body, err := json.Marshal(map[string]any{
		"jql":        jql,
		"fields":     []string{"summary", "status", "priority"},
		"maxResults": 30,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/rest/api/3/search/jql", bytes.NewReader(body))
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

// FullIssue is a detailed representation of a Jira issue.
type FullIssue struct {
	Key         string   `json:"key"`
	Summary     string   `json:"summary"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	IssueType   string   `json:"issue_type"`
	ProjectKey  string   `json:"project_key"`
	ProjectName string   `json:"project_name"`
	Assignee    string   `json:"assignee"`
	Reporter    string   `json:"reporter"`
	Description string   `json:"description"`
	Parent      string   `json:"parent,omitempty"`
	Labels      []string `json:"labels,omitempty"`
}

// Transition is a valid workflow transition for a Jira issue.
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Project is a Jira project summary.
type Project struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// IssueType is a Jira issue type.
type IssueType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

// GetIssue fetches a single issue by key with full field details.
func (c *Client) GetIssue(ctx context.Context, key string) (*FullIssue, error) {
	url := fmt.Sprintf(
		"%s/rest/api/3/issue/%s?fields=summary,status,priority,issuetype,project,assignee,reporter,description,parent,labels",
		c.baseURL, key,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: get issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira: get issue: HTTP %s: %s", resp.Status, string(raw))
	}

	var raw struct {
		Key    string `json:"key"`
		Fields struct {
			Summary   string `json:"summary"`
			Status    struct{ Name string `json:"name"` } `json:"status"`
			Priority  struct{ Name string `json:"name"` } `json:"priority"`
			IssueType struct{ Name string `json:"name"` } `json:"issuetype"`
			Project   struct {
				Key  string `json:"key"`
				Name string `json:"name"`
			} `json:"project"`
			Assignee    *struct{ DisplayName string `json:"displayName"` } `json:"assignee"`
			Reporter    *struct{ DisplayName string `json:"displayName"` } `json:"reporter"`
			Description any                                                 `json:"description"`
			Parent      *struct{ Key string `json:"key"` }                  `json:"parent"`
			Labels      []string                                             `json:"labels"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("jira: get issue: decoding: %w", err)
	}

	issue := &FullIssue{
		Key:         raw.Key,
		Summary:     raw.Fields.Summary,
		Status:      raw.Fields.Status.Name,
		Priority:    raw.Fields.Priority.Name,
		IssueType:   raw.Fields.IssueType.Name,
		ProjectKey:  raw.Fields.Project.Key,
		ProjectName: raw.Fields.Project.Name,
		Description: adfToText(raw.Fields.Description),
		Labels:      raw.Fields.Labels,
	}
	if raw.Fields.Assignee != nil {
		issue.Assignee = raw.Fields.Assignee.DisplayName
	}
	if raw.Fields.Reporter != nil {
		issue.Reporter = raw.Fields.Reporter.DisplayName
	}
	if raw.Fields.Parent != nil {
		issue.Parent = raw.Fields.Parent.Key
	}
	return issue, nil
}

// GetTransitions returns available workflow transitions for a Jira issue.
func (c *Client) GetTransitions(ctx context.Context, key string) ([]Transition, error) {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.baseURL, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: transitions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira: transitions: HTTP %s", resp.Status)
	}

	var result struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"transitions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("jira: transitions: decoding: %w", err)
	}

	out := make([]Transition, len(result.Transitions))
	for i, t := range result.Transitions {
		out[i] = Transition{ID: t.ID, Name: t.Name}
	}
	return out, nil
}

// TransitionIssue applies a transition (by ID) to a Jira issue.
func (c *Client) TransitionIssue(ctx context.Context, key, transitionID string) error {
	payload, err := json.Marshal(map[string]any{
		"transition": map[string]any{"id": transitionID},
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.baseURL, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("jira: transition: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira: transition: HTTP %s: %s", resp.Status, string(raw))
	}
	return nil
}

// CreateIssue creates a new Jira issue with the given fields map and returns the new issue key.
func (c *Client) CreateIssue(ctx context.Context, fields map[string]any) (string, error) {
	payload, err := json.Marshal(map[string]any{"fields": fields})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/rest/api/3/issue", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("jira: create issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("jira: create issue: HTTP %s: %s", resp.Status, string(raw))
	}

	var result struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("jira: create issue: decoding: %w", err)
	}
	return result.Key, nil
}

// CreateIssueSimple is a convenience wrapper that builds the fields map from plain-string arguments.
func (c *Client) CreateIssueSimple(ctx context.Context, projectKey, issueTypeName, summary, priority, description string) (string, error) {
	fields := map[string]any{
		"project":   map[string]any{"key": projectKey},
		"summary":   summary,
		"issuetype": map[string]any{"name": issueTypeName},
	}
	if priority != "" {
		fields["priority"] = map[string]any{"name": priority}
	}
	if description != "" {
		fields["description"] = TextToADF(description)
	}
	return c.CreateIssue(ctx, fields)
}

// CreateSubtask creates a child/subtask issue under parentKey.
func (c *Client) CreateSubtask(ctx context.Context, projectKey, parentKey, issueTypeName, summary string) (string, error) {
	fields := map[string]any{
		"project":   map[string]any{"key": projectKey},
		"summary":   summary,
		"issuetype": map[string]any{"name": issueTypeName},
		"parent":    map[string]any{"key": parentKey},
	}
	return c.CreateIssue(ctx, fields)
}

// UpdateIssue updates fields on an existing Jira issue.
func (c *Client) UpdateIssue(ctx context.Context, key string, fields map[string]any) error {
	payload, err := json.Marshal(map[string]any{"fields": fields})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.baseURL, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("jira: update issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira: update issue: HTTP %s: %s", resp.Status, string(raw))
	}
	return nil
}

// AddWorklog logs time against a Jira issue. seconds must be > 0. comment is optional.
func (c *Client) AddWorklog(ctx context.Context, key string, seconds int, comment string) error {
	body := map[string]any{"timeSpentSeconds": seconds}
	if comment != "" {
		body["comment"] = TextToADF(comment)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/rest/api/3/issue/%s/worklog", c.baseURL, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("jira: worklog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira: worklog: HTTP %s: %s", resp.Status, string(raw))
	}
	return nil
}

// GetProjects returns all Jira projects visible to the authenticated user.
func (c *Client) GetProjects(ctx context.Context) ([]Project, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/rest/api/3/project/search?maxResults=50&orderBy=name", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: projects: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira: projects: HTTP %s", resp.Status)
	}

	var result struct {
		Values []struct {
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("jira: projects: decoding: %w", err)
	}

	projects := make([]Project, len(result.Values))
	for i, v := range result.Values {
		projects[i] = Project{Key: v.Key, Name: v.Name}
	}
	return projects, nil
}

// GetIssueTypes returns the issue types available for a given project key.
func (c *Client) GetIssueTypes(ctx context.Context, projectKey string) ([]IssueType, error) {
	url := fmt.Sprintf("%s/rest/api/3/project/%s", c.baseURL, projectKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: issue types: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira: issue types: HTTP %s", resp.Status)
	}

	var result struct {
		IssueTypes []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Subtask bool   `json:"subtask"`
		} `json:"issueTypes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("jira: issue types: decoding: %w", err)
	}

	types := make([]IssueType, len(result.IssueTypes))
	for i, t := range result.IssueTypes {
		types[i] = IssueType{ID: t.ID, Name: t.Name, Subtask: t.Subtask}
	}
	return types, nil
}

// TextToADF wraps plain text in a minimal Atlassian Document Format document.
func TextToADF(text string) map[string]any {
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{
						"type": "text",
						"text": text,
					},
				},
			},
		},
	}
}

// adfToText extracts plain text from an Atlassian Document Format (ADF) value.
func adfToText(v any) string {
	if v == nil {
		return ""
	}
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	nodeType, _ := m["type"].(string)
	switch nodeType {
	case "text":
		text, _ := m["text"].(string)
		return text
	case "hardBreak", "rule":
		return "\n"
	case "listItem":
		content, _ := m["content"].([]any)
		var sb strings.Builder
		for _, c := range content {
			sb.WriteString(adfToText(c))
		}
		return "• " + strings.TrimSpace(sb.String()) + "\n"
	default:
		content, _ := m["content"].([]any)
		var sb strings.Builder
		for _, c := range content {
			sb.WriteString(adfToText(c))
		}
		switch nodeType {
		case "paragraph", "heading", "bulletList", "orderedList", "blockquote", "codeBlock":
			sb.WriteString("\n")
		}
		return sb.String()
	}
}
