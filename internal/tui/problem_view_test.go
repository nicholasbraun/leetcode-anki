package tui

import (
	"strings"
	"testing"
)

func TestProblemDetailLayout(t *testing.T) {
	t.Run("no solution gives full width to description", func(t *testing.T) {
		descW, _, solW, _ := problemDetailLayout(160, 40, false)
		if solW != 0 {
			t.Errorf("solW=%d, want 0", solW)
		}
		if descW != 160 {
			t.Errorf("descW=%d, want 160", descW)
		}
	})
	t.Run("with solution and wide enough, panes split with gap", func(t *testing.T) {
		descW, _, solW, _ := problemDetailLayout(160, 40, true)
		if solW == 0 {
			t.Fatal("expected non-zero solution pane on wide terminal")
		}
		if descW+detailGap+solW != 160 {
			t.Errorf("widths don't account for total: descW=%d gap=%d solW=%d sum=%d", descW, detailGap, solW, descW+detailGap+solW)
		}
		if solW < detailSolMinWidth || solW > detailSolMaxWidth {
			t.Errorf("solW=%d outside [%d,%d]", solW, detailSolMinWidth, detailSolMaxWidth)
		}
	})
	t.Run("narrow terminal drops solution pane", func(t *testing.T) {
		descW, _, solW, _ := problemDetailLayout(80, 40, true)
		if solW != 0 {
			t.Errorf("solW=%d, want 0 on narrow terminal", solW)
		}
		if descW != 80 {
			t.Errorf("descW=%d, want 80", descW)
		}
	})
}

func TestStatusBadge(t *testing.T) {
	ac := "ACCEPTED"
	acShort := "AC"
	finish := "FINISH"
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
		{"AC short", &acShort, false, false, "Solved"},
		{"FINISH variant", &finish, false, false, "Solved"},
		{"accepted with draft still solved", &ac, true, false, "Solved"},
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
