package learning

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMigrateIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "learning.db")
	svc, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := svc.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	var version int
	if err := svc.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("PRAGMA user_version: %v", err)
	}
	if version != 2 {
		t.Fatalf("user_version = %d, want 2", version)
	}
	if err := svc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestServiceAddSearchDueStatusReview(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	svc := testService(t, now)

	entry, err := svc.Add(ctx, AddInput{
		Topic:       "SQLite indexes",
		Source:      "db notes",
		Description: "Indexes speed lookups by trading write cost.",
		Questions:   []string{"What do indexes speed up?"},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if entry.ID == 0 {
		t.Fatalf("entry ID was not set")
	}
	if got, want := entry.NextDueAt, now.AddDate(0, 0, 1); !got.Equal(want) {
		t.Fatalf("NextDueAt = %s, want %s", got, want)
	}

	if _, err := svc.Add(ctx, AddInput{
		Topic:       "SQLite indexes",
		Description: "duplicate",
		Questions:   []string{"duplicate?"},
	}); !errors.Is(err, ErrDuplicateTopic) {
		t.Fatalf("duplicate Add error = %v, want ErrDuplicateTopic", err)
	}

	results, err := svc.Search(ctx, "write cost")
	if err != nil {
		t.Fatalf("Search description: %v", err)
	}
	if len(results) != 1 || results[0].ID != entry.ID {
		t.Fatalf("Search description results = %#v", results)
	}
	results, err = svc.Search(ctx, "speed up")
	if err != nil {
		t.Fatalf("Search question: %v", err)
	}
	if len(results) != 1 || results[0].Questions[0].Question != "What do indexes speed up?" {
		t.Fatalf("Search question results = %#v", results)
	}

	due, err := svc.Due(ctx)
	if err != nil {
		t.Fatalf("Due before date: %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("Due before date len = %d, want 0", len(due))
	}

	svc.now = func() time.Time { return now.AddDate(0, 0, 2) }
	due, err = svc.Due(ctx)
	if err != nil {
		t.Fatalf("Due after date: %v", err)
	}
	if len(due) != 1 || due[0].ID != entry.ID {
		t.Fatalf("Due after date = %#v", due)
	}

	review, err := svc.Review(ctx, entry.ID, RatingGood, "remembered")
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if review.IntervalDays != 2 {
		t.Fatalf("review interval = %d, want 2", review.IntervalDays)
	}
	if !review.NextDueAt.Equal(now.AddDate(0, 0, 4)) {
		t.Fatalf("review next due = %s, want %s", review.NextDueAt, now.AddDate(0, 0, 4))
	}

	detail, err := svc.Detail(ctx, entry.ID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if detail.Entry.ID != entry.ID {
		t.Fatalf("Detail entry = %#v", detail.Entry)
	}
	if len(detail.Reviews) != 1 || detail.Reviews[0].ID != review.ID || detail.Reviews[0].Entry != nil {
		t.Fatalf("Detail reviews = %#v", detail.Reviews)
	}

	status, err := svc.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.ActiveEntries != 1 || status.DueCount != 0 || status.RecentReviewCount != 1 {
		t.Fatalf("Status = %#v", status)
	}
	if len(status.UpcomingReviews) != 1 || status.UpcomingReviews[0].ID != entry.ID {
		t.Fatalf("UpcomingReviews = %#v", status.UpcomingReviews)
	}

	if _, err := svc.Add(ctx, AddInput{
		Topic:          "SQLite indexes",
		Description:    "duplicate allowed",
		Questions:      []string{"allowed?"},
		AllowDuplicate: true,
	}); err != nil {
		t.Fatalf("Add allow duplicate: %v", err)
	}
}

func TestNextIntervalDays(t *testing.T) {
	tests := []struct {
		current int
		rating  string
		want    int
	}{
		{10, RatingMissed, 1},
		{1, RatingHard, 2},
		{5, RatingHard, 6},
		{3, RatingGood, 6},
		{3, RatingEasy, 10},
		{0, RatingGood, 2},
	}
	for _, tt := range tests {
		if got := NextIntervalDays(tt.current, tt.rating); got != tt.want {
			t.Fatalf("NextIntervalDays(%d, %q) = %d, want %d", tt.current, tt.rating, got, tt.want)
		}
	}
}

func TestServiceArchive(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	svc := testService(t, now)

	entry, err := svc.Add(ctx, AddInput{
		Topic:       "archive me",
		Description: "entry to archive",
		Questions:   []string{"archive?"},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	svc.now = func() time.Time { return now.Add(time.Hour) }
	archived, err := svc.Archive(ctx, entry.ID)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if archived.Status != StatusArchived {
		t.Fatalf("archived status = %q, want %q", archived.Status, StatusArchived)
	}
	if !archived.UpdatedAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("archived updated at = %s, want %s", archived.UpdatedAt, now.Add(time.Hour))
	}

	status, err := svc.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.ActiveEntries != 0 {
		t.Fatalf("ActiveEntries = %d, want 0", status.ActiveEntries)
	}

	if _, err := svc.Archive(ctx, entry.ID); err == nil || !strings.Contains(err.Error(), "active learning entry not found") {
		t.Fatalf("second Archive error = %v", err)
	}
}

func TestServiceDraft(t *testing.T) {
	ctx := context.Background()
	svc := testService(t, time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC))

	draft, err := svc.Draft(ctx, AddInput{
		Topic:       "  Draft topic  ",
		Source:      " notes ",
		Description: "  draft description  ",
		Questions:   []string{" first? ", "", "second?"},
	})
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	if draft.Topic != "Draft topic" || draft.Source != "notes" || draft.Description != "draft description" {
		t.Fatalf("draft = %#v", draft)
	}
	if len(draft.Questions) != 2 || draft.Questions[0].Position != 1 || draft.Questions[0].Question != "first?" {
		t.Fatalf("draft questions = %#v", draft.Questions)
	}
	if draft.Duplicate || !draft.WouldCreate || draft.ExistingEntry != nil {
		t.Fatalf("draft duplicate fields = %#v", draft)
	}

	status, err := svc.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.ActiveEntries != 0 {
		t.Fatalf("ActiveEntries after draft = %d, want 0", status.ActiveEntries)
	}

	entry, err := svc.Add(ctx, AddInput{
		Topic:       "Draft topic",
		Description: "saved",
		Questions:   []string{"saved?"},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	duplicateDraft, err := svc.Draft(ctx, AddInput{
		Topic:       "Draft topic",
		Description: "duplicate",
		Questions:   []string{"duplicate?"},
	})
	if err != nil {
		t.Fatalf("duplicate Draft: %v", err)
	}
	if !duplicateDraft.Duplicate || duplicateDraft.WouldCreate || duplicateDraft.ExistingEntry == nil || duplicateDraft.ExistingEntry.ID != entry.ID {
		t.Fatalf("duplicate draft = %#v", duplicateDraft)
	}
	if duplicateDraft.DuplicateReason != "normalized_topic" || duplicateDraft.NormalizedTopic != "draft topic" {
		t.Fatalf("duplicate draft normalization = %#v", duplicateDraft)
	}
}

func TestServiceNormalizedDuplicateDetection(t *testing.T) {
	ctx := context.Background()
	svc := testService(t, time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC))

	entry, err := svc.Add(ctx, AddInput{
		Topic:       "SQLite indexes",
		Source:      "DB notes",
		Description: "Indexes speed reads.",
		Questions:   []string{"What do indexes speed up?"},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if _, err := svc.Add(ctx, AddInput{
		Topic:       " sqlite   indexes! ",
		Description: "duplicate by normalized topic",
		Questions:   []string{"duplicate?"},
	}); !errors.Is(err, ErrDuplicateTopic) {
		t.Fatalf("normalized duplicate Add error = %v, want ErrDuplicateTopic", err)
	}

	duplicateDraft, err := svc.Draft(ctx, AddInput{
		Topic:       "SQLITE, indexes",
		Source:      "db notes",
		Description: "duplicate draft",
		Questions:   []string{"duplicate?"},
	})
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	if !duplicateDraft.Duplicate || duplicateDraft.DuplicateReason != "normalized_topic_and_source" || duplicateDraft.ExistingEntry == nil || duplicateDraft.ExistingEntry.ID != entry.ID {
		t.Fatalf("duplicate draft = %#v", duplicateDraft)
	}

	if _, err := svc.Archive(ctx, entry.ID); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if _, err := svc.Add(ctx, AddInput{
		Topic:       "sqlite indexes",
		Description: "allowed after archive",
		Questions:   []string{"allowed?"},
	}); err != nil {
		t.Fatalf("Add after archive: %v", err)
	}
}

func TestServiceReviewSummary(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	svc := testService(t, now)

	entry, err := svc.Add(ctx, AddInput{
		Topic:       "leech candidate",
		Description: "missed repeatedly",
		Questions:   []string{"question?"},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	ratings := []string{RatingMissed, RatingHard, RatingMissed, RatingMissed}
	for i, rating := range ratings {
		reviewTime := now.AddDate(0, 0, i+1)
		svc.now = func() time.Time { return reviewTime }
		if _, err := svc.Review(ctx, entry.ID, rating, ""); err != nil {
			t.Fatalf("Review %d: %v", i, err)
		}
	}

	detail, err := svc.Detail(ctx, entry.ID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	summary := detail.ReviewSummary
	if summary.TotalReviews != 4 || summary.MissedCount != 3 || summary.HardCount != 1 || summary.GoodCount != 0 || summary.EasyCount != 0 {
		t.Fatalf("summary counts = %#v", summary)
	}
	if summary.ConsecutiveMissed != 2 || summary.ConsecutiveStruggled != 4 {
		t.Fatalf("summary streaks = %#v", summary)
	}
	if summary.LastReviewedAt == nil || !summary.LastReviewedAt.Equal(now.AddDate(0, 0, 4)) {
		t.Fatalf("last reviewed at = %#v, want %s", summary.LastReviewedAt, now.AddDate(0, 0, 4))
	}
	if !summary.LeechCandidate {
		t.Fatalf("LeechCandidate = false, want true")
	}
}

func TestSingleConnectionInMemoryDB(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	db.SetMaxOpenConns(1)
	svc := NewForDB(db, func() time.Time {
		return time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	})
	t.Cleanup(func() { _ = svc.Close() })
	if err := svc.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	entry, err := svc.Add(ctx, AddInput{
		Topic:       "single connection",
		Description: "scan entries without a second connection",
		Questions:   []string{"question?"},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(entry.Questions) != 1 {
		t.Fatalf("Questions = %#v", entry.Questions)
	}
}

func testService(t *testing.T, now time.Time) *Service {
	t.Helper()
	svc, err := Open(context.Background(), filepath.Join(t.TempDir(), "learning.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	svc.now = func() time.Time { return now }
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}
