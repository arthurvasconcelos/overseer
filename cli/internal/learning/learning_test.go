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
	if version != 3 {
		t.Fatalf("user_version = %d, want 3", version)
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

func TestServiceUpdatePreservesScheduleAndHistory(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC)
	svc := testService(t, now)

	entry, err := svc.Add(ctx, AddInput{
		Topic:       "Conic gradient rim",
		Source:      "labs note",
		Description: "mask window is a ~68 degree arc",
		Questions:   []string{"How wide is the window?", "Why is it not a border?"},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	svc.now = func() time.Time { return now.AddDate(0, 0, 1) }
	if _, err := svc.Review(ctx, entry.ID, RatingGood, "first pass"); err != nil {
		t.Fatalf("Review: %v", err)
	}
	reviewed, err := svc.Get(ctx, entry.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	corrected := now.AddDate(0, 0, 2)
	svc.now = func() time.Time { return corrected }
	description := "mask window ramps from 180 to 320 degrees, ~138 degrees wide"
	questions := []string{"How wide is the window?"}
	updated, err := svc.Update(ctx, entry.ID, EditInput{
		Description:  &description,
		Questions:    &questions,
		RevisionNote: "68 degree arc read only the plateau",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.ID != entry.ID {
		t.Fatalf("ID = %d, want %d", updated.ID, entry.ID)
	}
	if updated.Description != description {
		t.Fatalf("Description = %q", updated.Description)
	}
	if updated.Topic != entry.Topic || updated.Source != entry.Source {
		t.Fatalf("untouched fields changed: topic %q source %q", updated.Topic, updated.Source)
	}
	if len(updated.Questions) != 1 || updated.Questions[0].Question != questions[0] || updated.Questions[0].Position != 1 {
		t.Fatalf("Questions = %#v", updated.Questions)
	}
	if !updated.NextDueAt.Equal(reviewed.NextDueAt) || updated.IntervalDays != reviewed.IntervalDays {
		t.Fatalf("schedule changed: due %s interval %d", updated.NextDueAt, updated.IntervalDays)
	}
	if !updated.CreatedAt.Equal(entry.CreatedAt) {
		t.Fatalf("CreatedAt = %s, want %s", updated.CreatedAt, entry.CreatedAt)
	}
	if updated.CorrectedAt == nil || !updated.CorrectedAt.Equal(corrected) {
		t.Fatalf("CorrectedAt = %v, want %s", updated.CorrectedAt, corrected)
	}
	if updated.RevisionNote != "68 degree arc read only the plateau" {
		t.Fatalf("RevisionNote = %q", updated.RevisionNote)
	}

	reviews, err := svc.Reviews(ctx, entry.ID)
	if err != nil {
		t.Fatalf("Reviews: %v", err)
	}
	if len(reviews) != 1 || reviews[0].Notes != "first pass" {
		t.Fatalf("review history lost: %#v", reviews)
	}
}

func TestServiceUpdateValidation(t *testing.T) {
	ctx := context.Background()
	svc := testService(t, time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC))

	entry, err := svc.Add(ctx, AddInput{Topic: "Alpha", Description: "desc", Questions: []string{"Q?"}})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	other, err := svc.Add(ctx, AddInput{Topic: "Beta", Description: "desc", Questions: []string{"Q?"}})
	if err != nil {
		t.Fatalf("Add other: %v", err)
	}

	if _, err := svc.Update(ctx, entry.ID, EditInput{RevisionNote: "note only"}); err == nil || !strings.Contains(err.Error(), "no changes provided") {
		t.Fatalf("empty update error = %v", err)
	}

	blank := ""
	if _, err := svc.Update(ctx, entry.ID, EditInput{Topic: &blank}); err == nil || !strings.Contains(err.Error(), "topic is required") {
		t.Fatalf("blank topic error = %v", err)
	}

	noQuestions := []string{"  "}
	if _, err := svc.Update(ctx, entry.ID, EditInput{Questions: &noQuestions}); err == nil || !strings.Contains(err.Error(), "at least one quiz question") {
		t.Fatalf("empty quiz error = %v", err)
	}

	clash := "beta"
	if _, err := svc.Update(ctx, entry.ID, EditInput{Topic: &clash}); !errors.Is(err, ErrDuplicateTopic) {
		t.Fatalf("duplicate topic error = %v", err)
	}
	if _, err := svc.Update(ctx, entry.ID, EditInput{Topic: &clash, AllowDuplicate: true}); err != nil {
		t.Fatalf("Update with AllowDuplicate: %v", err)
	}

	// Rewriting an entry to its own current topic must not be treated as a duplicate of itself.
	sameTopic := "Beta"
	if _, err := svc.Update(ctx, other.ID, EditInput{Topic: &sameTopic, RevisionNote: "no-op topic"}); err != nil {
		t.Fatalf("self-topic update: %v", err)
	}

	if _, err := svc.Update(ctx, 9999, EditInput{Topic: &sameTopic}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing entry error = %v", err)
	}
}

func TestMigrateFromVersion2AddsCorrectionColumns(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "learning.db")

	svc, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := svc.db.ExecContext(ctx, `ALTER TABLE learning_entries DROP COLUMN corrected_at`); err != nil {
		t.Fatalf("drop corrected_at: %v", err)
	}
	if _, err := svc.db.ExecContext(ctx, `ALTER TABLE learning_entries DROP COLUMN revision_note`); err != nil {
		t.Fatalf("drop revision_note: %v", err)
	}
	if _, err := svc.db.ExecContext(ctx, `PRAGMA user_version = 2`); err != nil {
		t.Fatalf("reset user_version: %v", err)
	}
	if err := svc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	entry, err := reopened.Add(ctx, AddInput{Topic: "Upgraded", Description: "desc", Questions: []string{"Q?"}})
	if err != nil {
		t.Fatalf("Add after upgrade: %v", err)
	}
	if entry.CorrectedAt != nil || entry.RevisionNote != "" {
		t.Fatalf("fresh entry carries correction metadata: %v %q", entry.CorrectedAt, entry.RevisionNote)
	}
}
