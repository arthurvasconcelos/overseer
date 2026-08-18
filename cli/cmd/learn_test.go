package cmd

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/arthurvasconcelos/overseer/internal/learning"
	"github.com/arthurvasconcelos/overseer/internal/output"
	"github.com/spf13/cobra"
)

func TestLearnAddAndStatusJSON(t *testing.T) {
	t.Setenv("OVERSEER_BRAIN", t.TempDir())
	output.Format = "json"
	t.Cleanup(func() {
		output.Format = "text"
		learnAddOpts.source = ""
		learnAddOpts.description = ""
		learnAddOpts.questions = nil
		learnAddOpts.allowDuplicate = false
	})

	learnAddOpts.source = "test"
	learnAddOpts.description = "A CLI JSON output test."
	learnAddOpts.questions = []string{"What is being tested?"}
	addOut, err := captureStdout(func() error {
		return runLearnAdd(&cobra.Command{}, []string{"CLI JSON"})
	})
	if err != nil {
		t.Fatalf("runLearnAdd: %v", err)
	}
	var entry learning.Entry
	if err := json.Unmarshal([]byte(addOut), &entry); err != nil {
		t.Fatalf("unmarshal add output %q: %v", addOut, err)
	}
	if entry.Topic != "CLI JSON" || len(entry.Questions) != 1 {
		t.Fatalf("entry = %#v", entry)
	}

	statusOut, err := captureStdout(func() error {
		return runLearnStatus(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runLearnStatus: %v", err)
	}
	var status learning.Status
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("unmarshal status output %q: %v", statusOut, err)
	}
	if status.ActiveEntries != 1 {
		t.Fatalf("ActiveEntries = %d, want 1", status.ActiveEntries)
	}

	learnReviewOpts.entryID = entry.ID
	learnReviewOpts.rating = learning.RatingGood
	learnReviewOpts.notes = "clear"
	t.Cleanup(func() {
		learnReviewOpts.entryID = 0
		learnReviewOpts.rating = ""
		learnReviewOpts.notes = ""
	})
	if _, err := captureStdout(func() error {
		return runLearnReview(&cobra.Command{}, nil)
	}); err != nil {
		t.Fatalf("runLearnReview: %v", err)
	}

	showOut, err := captureStdout(func() error {
		return runLearnShow(&cobra.Command{}, []string{strconv.FormatInt(entry.ID, 10)})
	})
	if err != nil {
		t.Fatalf("runLearnShow: %v", err)
	}
	var detail learning.EntryDetail
	if err := json.Unmarshal([]byte(showOut), &detail); err != nil {
		t.Fatalf("unmarshal show output %q: %v", showOut, err)
	}
	if detail.Entry.ID != entry.ID {
		t.Fatalf("detail entry = %#v", detail.Entry)
	}
	if len(detail.Reviews) != 1 || detail.Reviews[0].Rating != learning.RatingGood {
		t.Fatalf("detail reviews = %#v", detail.Reviews)
	}
	if detail.ReviewSummary.TotalReviews != 1 || detail.ReviewSummary.GoodCount != 1 {
		t.Fatalf("detail review summary = %#v", detail.ReviewSummary)
	}

	archiveOut, err := captureStdout(func() error {
		return runLearnArchive(&cobra.Command{}, []string{strconv.FormatInt(entry.ID, 10)})
	})
	if err != nil {
		t.Fatalf("runLearnArchive: %v", err)
	}
	var archived learning.Entry
	if err := json.Unmarshal([]byte(archiveOut), &archived); err != nil {
		t.Fatalf("unmarshal archive output %q: %v", archiveOut, err)
	}
	if archived.ID != entry.ID || archived.Status != learning.StatusArchived {
		t.Fatalf("archived entry = %#v", archived)
	}

	statusOut, err = captureStdout(func() error {
		return runLearnStatus(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runLearnStatus after archive: %v", err)
	}
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("unmarshal status after archive output %q: %v", statusOut, err)
	}
	if status.ActiveEntries != 0 {
		t.Fatalf("ActiveEntries after archive = %d, want 0", status.ActiveEntries)
	}
}

func TestLearnSearchRequiresQuery(t *testing.T) {
	if err := learnSearchCmd.Args(&cobra.Command{}, nil); err == nil || !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Fatalf("learn search arg error = %v", err)
	}
}

func TestLearnShowRequiresEntryID(t *testing.T) {
	if err := learnShowCmd.Args(&cobra.Command{}, nil); err == nil || !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Fatalf("learn show arg error = %v", err)
	}
	if err := runLearnShow(&cobra.Command{}, []string{"nope"}); err == nil || !strings.Contains(err.Error(), "entry-id must be an integer") {
		t.Fatalf("learn show invalid id error = %v", err)
	}
}

func TestLearnArchiveRequiresEntryID(t *testing.T) {
	if err := learnArchiveCmd.Args(&cobra.Command{}, nil); err == nil || !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Fatalf("learn archive arg error = %v", err)
	}
	if err := runLearnArchive(&cobra.Command{}, []string{"nope"}); err == nil || !strings.Contains(err.Error(), "entry-id must be an integer") {
		t.Fatalf("learn archive invalid id error = %v", err)
	}
}

func captureStdout(fn func() error) (string, error) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	err = fn()
	closeErr := w.Close()
	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, r)
	_ = r.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if copyErr != nil {
		return "", copyErr
	}
	return buf.String(), nil
}

func TestLearnEditJSON(t *testing.T) {
	brain := t.TempDir()
	t.Setenv("OVERSEER_BRAIN", brain)
	output.Format = "json"
	t.Cleanup(func() {
		output.Format = "text"
		learnAddOpts.description = ""
		learnAddOpts.questions = nil
	})

	learnAddOpts.description = "mask window is a ~68 degree arc"
	learnAddOpts.questions = []string{"How wide is the window?", "Why is it not a border?"}
	out, err := captureStdout(func() error {
		return runLearnAdd(&cobra.Command{}, []string{"Conic gradient rim"})
	})
	if err != nil {
		t.Fatalf("learn add: %v", err)
	}
	var added learning.Entry
	if err := json.Unmarshal([]byte(out), &added); err != nil {
		t.Fatalf("unmarshal add: %v", err)
	}

	cmd := newLearnEditTestCmd()
	mustSetFlag(t, cmd, "description", "window ramps from 180 to 320 degrees")
	mustSetFlag(t, cmd, "quiz", "How wide is the window?")
	mustSetFlag(t, cmd, "note", "the 68 degree arc read only the plateau")
	out, err = captureStdout(func() error {
		return runLearnEdit(cmd, []string{strconv.FormatInt(added.ID, 10)})
	})
	if err != nil {
		t.Fatalf("learn edit: %v", err)
	}
	var edited learning.Entry
	if err := json.Unmarshal([]byte(out), &edited); err != nil {
		t.Fatalf("unmarshal edit: %v", err)
	}

	if edited.ID != added.ID {
		t.Fatalf("ID = %d, want %d", edited.ID, added.ID)
	}
	if edited.Topic != added.Topic {
		t.Fatalf("Topic = %q, want unchanged %q", edited.Topic, added.Topic)
	}
	if edited.Description != "window ramps from 180 to 320 degrees" {
		t.Fatalf("Description = %q", edited.Description)
	}
	if len(edited.Questions) != 1 {
		t.Fatalf("Questions = %#v", edited.Questions)
	}
	if edited.RevisionNote != "the 68 degree arc read only the plateau" {
		t.Fatalf("RevisionNote = %q", edited.RevisionNote)
	}
	if edited.CorrectedAt == nil {
		t.Fatalf("CorrectedAt not recorded")
	}
	if !edited.NextDueAt.Equal(added.NextDueAt) {
		t.Fatalf("NextDueAt = %s, want unchanged %s", edited.NextDueAt, added.NextDueAt)
	}
}

func TestLearnEditClearsSourceAndRejectsBadID(t *testing.T) {
	brain := t.TempDir()
	t.Setenv("OVERSEER_BRAIN", brain)
	output.Format = "json"
	t.Cleanup(func() {
		output.Format = "text"
		learnAddOpts.source = ""
		learnAddOpts.description = ""
		learnAddOpts.questions = nil
	})

	learnAddOpts.source = "https://example.test/wrong"
	learnAddOpts.description = "desc"
	learnAddOpts.questions = []string{"Q?"}
	if _, err := captureStdout(func() error {
		return runLearnAdd(&cobra.Command{}, []string{"Alpha"})
	}); err != nil {
		t.Fatalf("learn add: %v", err)
	}

	cmd := newLearnEditTestCmd()
	mustSetFlag(t, cmd, "source", "")
	out, err := captureStdout(func() error {
		return runLearnEdit(cmd, []string{"1"})
	})
	if err != nil {
		t.Fatalf("learn edit: %v", err)
	}
	var edited learning.Entry
	if err := json.Unmarshal([]byte(out), &edited); err != nil {
		t.Fatalf("unmarshal edit: %v", err)
	}
	if edited.Source != "" {
		t.Fatalf("Source = %q, want cleared", edited.Source)
	}
	if edited.Description != "desc" {
		t.Fatalf("Description = %q, want unchanged", edited.Description)
	}

	if err := runLearnEdit(newLearnEditTestCmd(), []string{"nope"}); err == nil || !strings.Contains(err.Error(), "entry-id must be an integer") {
		t.Fatalf("invalid id error = %v", err)
	}

	cmd = newLearnEditTestCmd()
	mustSetFlag(t, cmd, "description", "orphan")
	if err := runLearnEdit(cmd, []string{"4242"}); err == nil || !strings.Contains(err.Error(), "learning entry not found: 4242") {
		t.Fatalf("missing entry error = %v", err)
	}
}

func TestLearnReviewAllJSON(t *testing.T) {
	brain := t.TempDir()
	t.Setenv("OVERSEER_BRAIN", brain)
	output.Format = "json"
	t.Cleanup(func() {
		output.Format = "text"
		learnAddOpts.description = ""
		learnAddOpts.questions = nil
		learnReviewOpts.all = false
		learnReviewOpts.rating = ""
		learnReviewOpts.notes = ""
	})

	learnAddOpts.description = "desc"
	learnAddOpts.questions = []string{"Q?"}
	for _, topic := range []string{"Alpha", "Beta", "Gamma"} {
		if _, err := captureStdout(func() error {
			return runLearnAdd(&cobra.Command{}, []string{topic})
		}); err != nil {
			t.Fatalf("learn add %s: %v", topic, err)
		}
	}
	forceLearningDue(t, brain)

	learnReviewOpts.all = true
	learnReviewOpts.rating = learning.RatingGood
	learnReviewOpts.notes = "batch pass"
	out, err := captureStdout(func() error {
		return runLearnReview(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("learn review --all: %v", err)
	}
	var reviews []learning.Review
	if err := json.Unmarshal([]byte(out), &reviews); err != nil {
		t.Fatalf("unmarshal reviews: %v", err)
	}
	if len(reviews) != 3 {
		t.Fatalf("reviewed %d entries, want 3", len(reviews))
	}
	seen := map[int64]bool{}
	for _, r := range reviews {
		if r.Rating != learning.RatingGood || r.Notes != "batch pass" {
			t.Fatalf("review %#v", r)
		}
		seen[r.EntryID] = true
	}
	if len(seen) != 3 {
		t.Fatalf("distinct entries reviewed = %d, want 3", len(seen))
	}

	out, err = captureStdout(func() error {
		return runLearnReview(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("second learn review --all: %v", err)
	}
	var empty []learning.Review
	if err := json.Unmarshal([]byte(out), &empty); err != nil {
		t.Fatalf("unmarshal empty reviews: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected nothing due, got %d", len(empty))
	}
}

func newLearnEditTestCmd() *cobra.Command {
	learnEditOpts.topic = ""
	learnEditOpts.source = ""
	learnEditOpts.description = ""
	learnEditOpts.questions = nil
	learnEditOpts.note = ""
	learnEditOpts.allowDuplicate = false

	cmd := &cobra.Command{}
	cmd.Flags().StringVar(&learnEditOpts.topic, "topic", "", "")
	cmd.Flags().StringVar(&learnEditOpts.source, "source", "", "")
	cmd.Flags().StringVar(&learnEditOpts.description, "description", "", "")
	cmd.Flags().StringArrayVar(&learnEditOpts.questions, "quiz", nil, "")
	cmd.Flags().StringVar(&learnEditOpts.note, "note", "", "")
	cmd.Flags().BoolVar(&learnEditOpts.allowDuplicate, "allow-duplicate", false, "")
	return cmd
}

func mustSetFlag(t *testing.T, cmd *cobra.Command, name, value string) {
	t.Helper()
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatalf("set --%s: %v", name, err)
	}
}

func forceLearningDue(t *testing.T, brain string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(brain, "overseer", "learning.db"))
	if err != nil {
		t.Fatalf("open learning db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE learning_entries SET next_due_at = '2020-01-01T00:00:00Z'`); err != nil {
		t.Fatalf("force due: %v", err)
	}
}
