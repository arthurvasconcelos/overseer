package output

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestPrintJSON(t *testing.T) {
	t.Run("struct", func(t *testing.T) {
		v := struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}{"alice", 30}

		out := captureStdout(func() { PrintJSON(v) })
		if !strings.Contains(out, `"name": "alice"`) {
			t.Errorf("unexpected output: %s", out)
		}
		if !strings.Contains(out, `"age": 30`) {
			t.Errorf("unexpected output: %s", out)
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		out := captureStdout(func() { PrintJSON([]any{}) })
		if strings.TrimSpace(out) != "[]" {
			t.Errorf("expected [], got: %s", out)
		}
	})

	t.Run("unmarshalable returns error", func(t *testing.T) {
		err := PrintJSON(make(chan int))
		if err == nil {
			t.Error("expected error for unmarshalable value, got nil")
		}
	})
}
