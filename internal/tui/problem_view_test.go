package tui

import (
	"context"
	"strings"
	"testing"

	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/leetcode/leetcodefake"
)

func TestProblemDetailLayout(t *testing.T) {
	t.Run("wide enough splits with gap", func(t *testing.T) {
		descW, _, solW, _ := problemDetailLayout(160, 40)
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
	t.Run("narrow terminal collapses to single pane", func(t *testing.T) {
		descW, _, solW, _ := problemDetailLayout(80, 40)
		if solW != 0 {
			t.Errorf("solW=%d, want 0 on narrow terminal", solW)
		}
		if descW != 80 {
			t.Errorf("descW=%d, want 80", descW)
		}
	})
}

// solutionMarker is a string seeded into the rendered solution viewport so
// the render tests can assert whether the cached Solution leaks into the
// rendered view. Picked to be unlikely to appear in chrome / footer text.
const solutionMarker = "ZZ_TWOSUM_MARKER_ZZ"

// onDetailScreenWithSolution parks a Model on the problem detail screen
// with a cached Solution already rendered into the right pane viewport.
// The solution viewport's rendered content embeds solutionMarker so tests
// can detect leakage.
func onDetailScreenWithSolution(t *testing.T, reviewMode bool) *Model {
	t.Helper()
	cache := newFakeCache()
	ed := newFakeEditor()
	m := NewModel(context.Background(), &leetcodefake.Fake{}, cache, ed, newFakeReviews())
	m.width, m.height = 140, 40
	m.reviewMode = reviewMode
	m.currentList = leetcode.FavoriteList{Slug: "x", Name: "X"}
	m.currentProblem = problemDetailFor("two-sum")
	m.problem = newProblemView(cache, 100, 30)
	m.problem.chosenLang = "golang"
	descW, descH, solW, solH := problemDetailLayout(m.width, m.height)
	m.problem.rendered = "Two Sum description body"
	m.problem.vp.Width = descW
	m.problem.vp.Height = descH
	m.problem.vp.SetContent(m.problem.rendered)
	m.problem.solutionPath = "/fake/two-sum/solution.golang"
	m.problem.solutionRendered = "func twoSum(nums []int) []int { /* " + solutionMarker + " */ }"
	m.problem.solutionVP.Width = solW
	m.problem.solutionVP.Height = solH
	m.problem.solutionVP.SetContent(m.problem.solutionRendered)
	m.screen = screenProblem
	return m
}

// In Review Mode the right pane must NOT render the cached Solution —
// the whole point of Review is recall. The placeholder appears instead.
func TestProblemView_ReviewMode_HidesSolutionPreview(t *testing.T) {
	m := onDetailScreenWithSolution(t, true)
	view := viewProblemView(m)
	if strings.Contains(view, solutionMarker) {
		t.Errorf("Review Mode leaked the cached Solution into the right pane:\n%s", view)
	}
	if !strings.Contains(strings.ToLower(view), "solution hidden in review mode") {
		t.Errorf("Review Mode placeholder missing from right pane:\n%s", view)
	}
}

// Regression for the existing behavior: Explore Mode keeps showing the
// cached Solution. Asserts the marker IS present so a future "always hide"
// regression would be caught.
func TestProblemView_ExploreMode_ShowsSolutionPreview(t *testing.T) {
	m := onDetailScreenWithSolution(t, false)
	view := viewProblemView(m)
	if !strings.Contains(view, solutionMarker) {
		t.Errorf("Explore Mode dropped the cached Solution from the right pane:\n%s", view)
	}
	if strings.Contains(strings.ToLower(view), "solution hidden in review mode") {
		t.Errorf("Explore Mode rendered the Review-Mode placeholder:\n%s", view)
	}
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
		solution  bool
		wantEmpty bool
		wantText  string
	}{
		{"accepted", &ac, false, false, "Solved"},
		{"AC short", &acShort, false, false, "Solved"},
		{"FINISH variant", &finish, false, false, "Solved"},
		{"accepted with solution still solved", &ac, true, false, "Solved"},
		{"tried", &tried, false, false, "In progress"},
		{"solution only", nil, true, false, "In progress"},
		{"tried and solution", &tried, true, false, "In progress"},
		{"not_started no solution", &notStarted, false, true, ""},
		{"nil no solution", nil, false, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := statusBadge(tc.status, tc.solution)
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
