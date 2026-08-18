package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
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
