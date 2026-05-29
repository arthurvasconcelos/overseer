package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
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
}

func TestLearnSearchRequiresQuery(t *testing.T) {
	if err := learnSearchCmd.Args(&cobra.Command{}, nil); err == nil || !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Fatalf("learn search arg error = %v", err)
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
