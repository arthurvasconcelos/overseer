package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/arthurvasconcelos/overseer/internal/learning"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCPLearningAddValidation(t *testing.T) {
	t.Setenv("OVERSEER_BRAIN", t.TempDir())
	res, err := mcpLearningAdd(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"description": "missing topic",
			"quiz":        []string{"Question?"},
		}},
	})
	if err != nil {
		t.Fatalf("mcpLearningAdd: %v", err)
	}
	if !res.IsError {
		t.Fatalf("result IsError = false, want true")
	}
	if !strings.Contains(mcpText(t, res), "topic is required") {
		t.Fatalf("error text = %q", mcpText(t, res))
	}
}

func TestMCPLearningAddAndReview(t *testing.T) {
	t.Setenv("OVERSEER_BRAIN", t.TempDir())
	ctx := context.Background()
	addRes, err := mcpLearningAdd(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"topic":       "Go database/sql",
			"description": "Use database/sql with drivers.",
			"quiz":        []string{"What package provides the generic DB API?"},
			"source":      "go docs",
		}},
	})
	if err != nil {
		t.Fatalf("mcpLearningAdd: %v", err)
	}
	if addRes.IsError {
		t.Fatalf("learning_add error: %s", mcpText(t, addRes))
	}
	var entry learning.Entry
	if err := json.Unmarshal([]byte(mcpText(t, addRes)), &entry); err != nil {
		t.Fatalf("unmarshal entry: %v", err)
	}
	if entry.ID == 0 || entry.Topic != "Go database/sql" {
		t.Fatalf("entry = %#v", entry)
	}

	reviewRes, err := mcpLearningReview(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"entry_id": float64(entry.ID),
			"rating":   learning.RatingEasy,
			"notes":    "solid",
		}},
	})
	if err != nil {
		t.Fatalf("mcpLearningReview: %v", err)
	}
	if reviewRes.IsError {
		t.Fatalf("learning_review error: %s", mcpText(t, reviewRes))
	}
	var review learning.Review
	if err := json.Unmarshal([]byte(mcpText(t, reviewRes)), &review); err != nil {
		t.Fatalf("unmarshal review: %v", err)
	}
	if review.EntryID != entry.ID || review.IntervalDays != 4 {
		t.Fatalf("review = %#v", review)
	}
}

func mcpText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatalf("empty MCP content")
	}
	text, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T, want mcp.TextContent", res.Content[0])
	}
	return text.Text
}
