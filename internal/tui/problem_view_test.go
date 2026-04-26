package tui

import (
	"strings"
	"testing"
)

func TestStatusBadge(t *testing.T) {
	ac := "ACCEPTED"
	tried := "TRIED"
	notStarted := "NOT_STARTED"

	cases := []struct {
		name      string
		status    *string
		draft     bool
		wantEmpty bool
		wantText  string
	}{
		{"accepted", &ac, false, false, "Solved"},
		{"tried", &tried, false, false, "In progress"},
		{"draft only", nil, true, false, "In progress"},
		{"tried and draft", &tried, true, false, "In progress"},
		{"not_started no draft", &notStarted, false, true, ""},
		{"nil no draft", nil, false, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := statusBadge(tc.status, tc.draft)
			if tc.wantEmpty {
				if got != "" {
					t.Errorf("statusBadge=%q, want empty", got)
				}
				return
			}
			if !strings.Contains(got, tc.wantText) {
				t.Errorf("statusBadge=%q, want substring %q", got, tc.wantText)
			}
		})
	}
}
