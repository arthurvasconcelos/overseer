package google

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		input time.Duration
		want  string
	}{
		{90 * time.Minute, "1h30m"},
		{45 * time.Minute, "45m"},
		{2 * time.Hour, "2h0m"},
		{time.Hour + time.Minute, "1h1m"},
		{29 * time.Second, "0m"},
		{30 * time.Second, "1m"},
		{0, "0m"},
	}

	for _, tc := range cases {
		got := formatDuration(tc.input)
		if got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
