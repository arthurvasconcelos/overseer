package jira

import (
	"testing"
)

func TestParseWorkDuration(t *testing.T) {
	cases := []struct {
		input   string
		wantSec int
		wantErr bool
	}{
		{"25m", 1500, false},
		{"1h", 3600, false},
		{"1h30m", 5400, false},
		{"90", 5400, false},
		{"2h15m", 8100, false},
		{"0m", 0, true},
		{"", 0, true},
		{"abc", 0, true},
		{"-10m", 0, true},
		{"0", 0, true},
	}

	for _, tc := range cases {
		got, err := ParseWorkDuration(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseWorkDuration(%q): expected error, got %d", tc.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseWorkDuration(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.wantSec {
			t.Errorf("ParseWorkDuration(%q) = %d, want %d", tc.input, got, tc.wantSec)
		}
	}
}
