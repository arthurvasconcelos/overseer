// Package claudeai provides a minimal Anthropic Messages API client.
package claudeai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	apiBase = "https://api.anthropic.com"
	apiVer  = "2023-06-01"
	// Model is the Claude model used for team persona consultations.
	Model = "claude-sonnet-4-6"
)

// Client is a minimal Anthropic Messages API client.
type Client struct {
	apiKey string
	http   *http.Client
}

// New returns a Client authenticated with apiKey.
func New(apiKey string) *Client {
	return &Client{apiKey: apiKey, http: http.DefaultClient}
}

type msgRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type msgResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *apiError `json:"error,omitempty"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Ask sends question to Claude with the given system prompt and returns the text
// response. Pass maxTokens=0 to use the default of 1024.
func (c *Client) Ask(ctx context.Context, systemPrompt, question string, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	body, err := json.Marshal(msgRequest{
		Model:     Model,
		MaxTokens: maxTokens,
		System:    systemPrompt,
		Messages:  []message{{Role: "user", Content: question}},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", apiVer)
	req.Header.Set("content-type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("claude api: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("claude api: reading response: %w", err)
	}

	var result msgResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("claude api: decoding response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("claude api: %s", result.Error.Message)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("claude api: HTTP %d", resp.StatusCode)
	}
	for _, block := range result.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("claude api: no text in response")
}
