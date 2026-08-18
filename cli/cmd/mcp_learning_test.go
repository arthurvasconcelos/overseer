package cmd

import (
	"context"
	"encoding/json"
	"strconv"
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
	if code := mcpErrorCode(t, res); code != "invalid_input" {
		t.Fatalf("error code = %q, want invalid_input", code)
	}
}

func TestMCPLearningDraft(t *testing.T) {
	t.Setenv("OVERSEER_BRAIN", t.TempDir())
	ctx := context.Background()

	draftRes, err := mcpLearningDraft(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"topic":       " Draft topic ",
			"description": " Draft description ",
			"quiz":        []string{" question? ", ""},
			"source":      " notes ",
		}},
	})
	if err != nil {
		t.Fatalf("mcpLearningDraft: %v", err)
	}
	if draftRes.IsError {
		t.Fatalf("learning_draft error: %s", mcpText(t, draftRes))
	}
	var draft learning.Draft
	if err := json.Unmarshal([]byte(mcpText(t, draftRes)), &draft); err != nil {
		t.Fatalf("unmarshal draft: %v", err)
	}
	if draft.Topic != "Draft topic" || draft.Source != "notes" || draft.Description != "Draft description" {
		t.Fatalf("draft = %#v", draft)
	}
	if len(draft.Questions) != 1 || draft.Questions[0].Question != "question?" {
		t.Fatalf("draft questions = %#v", draft.Questions)
	}
	if draft.Duplicate || !draft.WouldCreate || draft.ExistingEntry != nil {
		t.Fatalf("draft duplicate fields = %#v", draft)
	}

	statusRes, err := mcpLearningStatus(ctx, mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("mcpLearningStatus: %v", err)
	}
	var status learning.Status
	if err := json.Unmarshal([]byte(mcpText(t, statusRes)), &status); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if status.ActiveEntries != 0 {
		t.Fatalf("ActiveEntries after draft = %d, want 0", status.ActiveEntries)
	}

	addRes, err := mcpLearningAdd(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"topic":       "Draft topic",
			"description": "Saved entry",
			"quiz":        []string{"saved?"},
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

	duplicateRes, err := mcpLearningDraft(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"topic":       "draft-topic!",
			"description": "Duplicate entry",
			"quiz":        []string{"duplicate?"},
		}},
	})
	if err != nil {
		t.Fatalf("duplicate mcpLearningDraft: %v", err)
	}
	if duplicateRes.IsError {
		t.Fatalf("duplicate learning_draft error: %s", mcpText(t, duplicateRes))
	}
	var duplicateDraft learning.Draft
	if err := json.Unmarshal([]byte(mcpText(t, duplicateRes)), &duplicateDraft); err != nil {
		t.Fatalf("unmarshal duplicate draft: %v", err)
	}
	if !duplicateDraft.Duplicate || duplicateDraft.WouldCreate || duplicateDraft.ExistingEntry == nil || duplicateDraft.ExistingEntry.ID != entry.ID {
		t.Fatalf("duplicate draft = %#v", duplicateDraft)
	}
	if duplicateDraft.DuplicateReason != "normalized_topic" || duplicateDraft.NormalizedTopic != "draft topic" {
		t.Fatalf("duplicate draft normalization = %#v", duplicateDraft)
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

	getRes, err := mcpLearningGet(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"entry_id": float64(entry.ID),
		}},
	})
	if err != nil {
		t.Fatalf("mcpLearningGet: %v", err)
	}
	if getRes.IsError {
		t.Fatalf("learning_get error: %s", mcpText(t, getRes))
	}
	var detail learning.EntryDetail
	if err := json.Unmarshal([]byte(mcpText(t, getRes)), &detail); err != nil {
		t.Fatalf("unmarshal detail: %v", err)
	}
	if detail.Entry.ID != entry.ID {
		t.Fatalf("detail entry = %#v", detail.Entry)
	}
	if len(detail.Reviews) != 1 || detail.Reviews[0].ID != review.ID {
		t.Fatalf("detail reviews = %#v", detail.Reviews)
	}
	if detail.ReviewSummary.TotalReviews != 1 || detail.ReviewSummary.EasyCount != 1 {
		t.Fatalf("detail review summary = %#v", detail.ReviewSummary)
	}

	archiveRes, err := mcpLearningArchive(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"entry_id": float64(entry.ID),
		}},
	})
	if err != nil {
		t.Fatalf("mcpLearningArchive: %v", err)
	}
	if archiveRes.IsError {
		t.Fatalf("learning_archive error: %s", mcpText(t, archiveRes))
	}
	var archived learning.Entry
	if err := json.Unmarshal([]byte(mcpText(t, archiveRes)), &archived); err != nil {
		t.Fatalf("unmarshal archived entry: %v", err)
	}
	if archived.ID != entry.ID || archived.Status != learning.StatusArchived {
		t.Fatalf("archived entry = %#v", archived)
	}

	archiveAgainRes, err := mcpLearningArchive(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"entry_id": float64(entry.ID),
		}},
	})
	if err != nil {
		t.Fatalf("second mcpLearningArchive: %v", err)
	}
	if !archiveAgainRes.IsError {
		t.Fatalf("second archive IsError = false, want true")
	}
	if code := mcpErrorCode(t, archiveAgainRes); code != "entry_not_found" {
		t.Fatalf("second archive error code = %q, want entry_not_found", code)
	}
}

func TestMCPLearningGetNotFound(t *testing.T) {
	t.Setenv("OVERSEER_BRAIN", t.TempDir())
	res, err := mcpLearningGet(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"entry_id": 404,
		}},
	})
	if err != nil {
		t.Fatalf("mcpLearningGet: %v", err)
	}
	if !res.IsError {
		t.Fatalf("result IsError = false, want true")
	}
	if !strings.Contains(mcpText(t, res), "learning entry not found: 404") {
		t.Fatalf("error text = %q", mcpText(t, res))
	}
	if code := mcpErrorCode(t, res); code != "entry_not_found" {
		t.Fatalf("error code = %q, want entry_not_found", code)
	}
}

func TestMCPLearningResources(t *testing.T) {
	t.Setenv("OVERSEER_BRAIN", t.TempDir())
	ctx := context.Background()
	addRes, err := mcpLearningAdd(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"topic":       "MCP resources",
			"description": "Resources expose read-only context.",
			"quiz":        []string{"What should resources expose?"},
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

	statusContents, err := mcpLearningStatusResource(ctx, mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: "overseer://learning/status"},
	})
	if err != nil {
		t.Fatalf("mcpLearningStatusResource: %v", err)
	}
	var status learning.Status
	if err := json.Unmarshal([]byte(resourceText(t, statusContents)), &status); err != nil {
		t.Fatalf("unmarshal status resource: %v", err)
	}
	if status.ActiveEntries != 1 {
		t.Fatalf("status resource = %#v", status)
	}

	entryContents, err := mcpLearningEntryResource(ctx, mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: "overseer://learning/entries/" + strconv.FormatInt(entry.ID, 10)},
	})
	if err != nil {
		t.Fatalf("mcpLearningEntryResource: %v", err)
	}
	var detail learning.EntryDetail
	if err := json.Unmarshal([]byte(resourceText(t, entryContents)), &detail); err != nil {
		t.Fatalf("unmarshal entry resource: %v", err)
	}
	if detail.Entry.ID != entry.ID {
		t.Fatalf("entry resource detail = %#v", detail)
	}
}

func TestMCPLearningReviewSessionPrompt(t *testing.T) {
	res, err := mcpLearningReviewSessionPrompt(context.Background(), mcp.GetPromptRequest{})
	if err != nil {
		t.Fatalf("mcpLearningReviewSessionPrompt: %v", err)
	}
	if len(res.Messages) != 2 {
		t.Fatalf("prompt messages len = %d, want 2", len(res.Messages))
	}
	link, ok := res.Messages[1].Content.(mcp.ResourceLink)
	if !ok {
		t.Fatalf("prompt content = %T, want mcp.ResourceLink", res.Messages[1].Content)
	}
	if link.URI != "overseer://learning/due" {
		t.Fatalf("resource link URI = %q", link.URI)
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

func resourceText(t *testing.T, contents []mcp.ResourceContents) string {
	t.Helper()
	if len(contents) == 0 {
		t.Fatalf("empty MCP resource contents")
	}
	text, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("resource content type = %T, want mcp.TextResourceContents", contents[0])
	}
	if text.MIMEType != "application/json" {
		t.Fatalf("resource MIME type = %q, want application/json", text.MIMEType)
	}
	return text.Text
}

func mcpErrorCode(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(mcpText(t, res)), &payload); err != nil {
		t.Fatalf("unmarshal MCP error payload: %v", err)
	}
	return payload.Code
}

func TestMCPLearningEdit(t *testing.T) {
	t.Setenv("OVERSEER_BRAIN", t.TempDir())
	ctx := context.Background()

	addRes, err := mcpLearningAdd(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"topic":       "Conic gradient rim",
			"description": "mask window is a ~68 degree arc",
			"quiz":        []string{"How wide is the window?", "Why is it not a border?"},
			"source":      "labs note",
		}},
	})
	if err != nil {
		t.Fatalf("mcpLearningAdd: %v", err)
	}
	var added learning.Entry
	if err := json.Unmarshal([]byte(mcpText(t, addRes)), &added); err != nil {
		t.Fatalf("unmarshal added entry: %v", err)
	}

	editRes, err := mcpLearningEdit(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"entry_id":      float64(added.ID),
			"description":   "window ramps from 180 to 320 degrees, ~138 degrees wide",
			"quiz":          []string{"How wide is the window?"},
			"revision_note": "the 68 degree arc read only the plateau",
		}},
	})
	if err != nil {
		t.Fatalf("mcpLearningEdit: %v", err)
	}
	if editRes.IsError {
		t.Fatalf("edit returned error: %s", mcpText(t, editRes))
	}
	var edited learning.Entry
	if err := json.Unmarshal([]byte(mcpText(t, editRes)), &edited); err != nil {
		t.Fatalf("unmarshal edited entry: %v", err)
	}
	if edited.ID != added.ID {
		t.Fatalf("ID = %d, want %d", edited.ID, added.ID)
	}
	if edited.Topic != added.Topic || edited.Source != added.Source {
		t.Fatalf("unpassed fields changed: topic %q source %q", edited.Topic, edited.Source)
	}
	if !strings.Contains(edited.Description, "138 degrees wide") {
		t.Fatalf("Description = %q", edited.Description)
	}
	if len(edited.Questions) != 1 {
		t.Fatalf("Questions = %#v", edited.Questions)
	}
	if edited.RevisionNote != "the 68 degree arc read only the plateau" || edited.CorrectedAt == nil {
		t.Fatalf("correction metadata = %q %v", edited.RevisionNote, edited.CorrectedAt)
	}
	if !edited.NextDueAt.Equal(added.NextDueAt) {
		t.Fatalf("NextDueAt = %s, want unchanged %s", edited.NextDueAt, added.NextDueAt)
	}
}

func TestMCPLearningEditErrors(t *testing.T) {
	t.Setenv("OVERSEER_BRAIN", t.TempDir())
	ctx := context.Background()

	cases := []struct {
		name string
		args map[string]any
		code string
	}{
		{"missing entry_id", map[string]any{"description": "x"}, "invalid_input"},
		{"no fields", map[string]any{"entry_id": float64(1), "revision_note": "why"}, "invalid_input"},
		{"empty quiz", map[string]any{"entry_id": float64(1), "quiz": []string{"  "}}, "invalid_input"},
		{"unknown entry", map[string]any{"entry_id": float64(4242), "description": "x"}, "entry_not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := mcpLearningEdit(ctx, mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: tc.args}})
			if err != nil {
				t.Fatalf("mcpLearningEdit: %v", err)
			}
			if !res.IsError {
				t.Fatalf("IsError = false, want true")
			}
			if code := mcpErrorCode(t, res); code != tc.code {
				t.Fatalf("error code = %q, want %q", code, tc.code)
			}
		})
	}
}
