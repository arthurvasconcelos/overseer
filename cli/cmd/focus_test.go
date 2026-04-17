package cmd

import (
	"testing"
	"time"
)

func TestFocusParseWorkDuration(t *testing.T) {
	cases := []struct {
		input   string
		wantSec int
		wantErr bool
	}{
		{"25m", 1500, false},
		{"1h", 3600, false},
		{"1h30m", 5400, false},
		{"90", 5400, false},
		{"0m", 0, true},
		{"", 0, true},
		{"abc", 0, true},
		{"-5m", 0, true},
	}

	for _, tc := range cases {
		got, err := focusParseWorkDuration(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("focusParseWorkDuration(%q): expected error, got %d", tc.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("focusParseWorkDuration(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.wantSec {
			t.Errorf("focusParseWorkDuration(%q) = %d, want %d", tc.input, got, tc.wantSec)
		}
	}
}

func TestFormatCountdown(t *testing.T) {
	cases := []struct {
		input time.Duration
		want  string
	}{
		{25 * time.Minute, "25:00"},
		{5*time.Minute + 30*time.Second, "05:30"},
		{time.Hour + 5*time.Minute + 3*time.Second, "01:05:03"},
		{time.Second, "00:01"},
		{time.Hour, "01:00:00"},
	}

	for _, tc := range cases {
		got := formatCountdown(tc.input)
		if got != tc.want {
			t.Errorf("formatCountdown(%v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
