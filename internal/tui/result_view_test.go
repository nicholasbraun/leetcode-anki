package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"leetcode-anki/internal/leetcode"
)

// modalSetup wires a Model into "user just submitted slug a, modal is open".
// Returns the Model and the fakeReviews so tests can assert Record calls.
func modalSetup(t *testing.T) (*Model, *fakeReviews, *fakeClient) {
	t.Helper()
	tomorrow := time.Now().AddDate(0, 0, 1)
	in6 := time.Now().AddDate(0, 0, 6)
	fc := &fakeClient{details: map[string]*leetcode.ProblemDetail{
		"a": sampleDetail("a"), "b": sampleDetail("b"), "c": sampleDetail("c"),
	}}
	fr := newFakeReviews()
	fr.previewResp = [4]time.Time{tomorrow, tomorrow, in6, in6.AddDate(0, 0, 9)}
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	loadFakeProblems(t, m, []Problem{
		{QuestionFrontendID: "1", Title: "A", TitleSlug: "a"},
		{QuestionFrontendID: "2", Title: "B", TitleSlug: "b"},
		{QuestionFrontendID: "3", Title: "C", TitleSlug: "c"},
	})
	m.currentProblem = &leetcode.ProblemDetail{TitleSlug: "a"}
	_, _ = m.Update(submitResultMsg{result: &leetcode.SubmitResult{
		StatusMsg:      "Accepted",
		SubmissionID:   "S1",
		StatusRuntime:  "3 ms",
		StatusMemory:   "4.2 MB",
		TotalCorrect:   58,
		TotalTestcases: 58,
	}})
	return m, fr, fc
}

func TestRenderRunResult(t *testing.T) {
	tests := []struct {
		name        string
		in          *leetcode.RunResult
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "nil",
			in:          nil,
			wantContain: []string{"no verdict"},
		},
		{
			name: "accepted",
			in: &leetcode.RunResult{
				StatusMsg:          "Accepted",
				CorrectAnswer:      true,
				CodeAnswer:         []string{"[0,1]"},
				ExpectedCodeAnswer: []string{"[0,1]"},
			},
			wantContain: []string{"Accepted"},
			wantAbsent:  []string{"Wrong Answer"},
		},
		{
			name: "wrong answer despite status_msg=Accepted",
			in: &leetcode.RunResult{
				StatusMsg:          "Accepted",
				CorrectAnswer:      false,
				CodeAnswer:         []string{"[]"},
				ExpectedCodeAnswer: []string{"[0,1]"},
			},
			wantContain: []string{"Wrong Answer", "your output", "expected output"},
			wantAbsent:  []string{"Accepted"},
		},
		{
			name: "compile error",
			in: &leetcode.RunResult{
				CompileError:     "syntax error",
				FullCompileError: "line 3: missing ';'",
			},
			wantContain: []string{"Compile Error", "missing ';'"},
		},
		{
			name: "runtime error",
			in: &leetcode.RunResult{
				RuntimeError:     "panic",
				FullRuntimeError: "index out of range",
				LastTestcase:     "[1,2]",
			},
			wantContain: []string{"Runtime Error", "index out of range", "[1,2]"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderRunResult(tc.in)
			for _, s := range tc.wantContain {
				if !strings.Contains(got, s) {
					t.Errorf("missing %q in:\n%s", s, got)
				}
			}
			for _, s := range tc.wantAbsent {
				if strings.Contains(got, s) {
					t.Errorf("unexpected %q in:\n%s", s, got)
				}
			}
		})
	}
}

// On an Accepted Submit, the rating modal opens automatically. The eager
// Record(0) call is gone — the modal owns the Record now.
func TestAcceptedOpensRatingModal(t *testing.T) {
	m, fr, _ := modalSetup(t)

	if m.result.grade == nil {
		t.Fatal("expected modal to be open")
	}
	if m.result.grade.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (Good as default)", m.result.grade.cursor)
	}
	if len(fr.records) != 0 {
		t.Errorf("expected no Record yet (deferred to commit); got %d", len(fr.records))
	}

	view := viewResultView(m)
	for _, want := range []string{"ACCEPTED", "Again", "Hard", "Good", "Easy", "3 ms", "4.2 MB"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q\nview:\n%s", want, view)
		}
	}
}

// Each numeric digit must record the matching rating and propagate the
// SubmissionID. In Review Mode with a remaining due slug, the returned
// cmd loads that slug.
func TestRatingDigitRecordsAndAdvances(t *testing.T) {
	for _, tc := range []struct {
		key    rune
		rating int
	}{{'1', 1}, {'2', 2}, {'3', 3}, {'4', 4}} {
		t.Run(string(tc.key), func(t *testing.T) {
			m, fr, _ := modalSetup(t)
			m.reviewMode = true
			m.dueSlugs = map[string]bool{"a": true, "b": true}
			m.problemsAll = []Problem{
				{QuestionFrontendID: "1", Title: "A", TitleSlug: "a"},
				{QuestionFrontendID: "2", Title: "B", TitleSlug: "b"},
			}

			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})

			if len(fr.records) != 1 {
				t.Fatalf("expected 1 Record, got %d", len(fr.records))
			}
			rec := fr.records[0]
			if rec.rating != tc.rating {
				t.Errorf("rating = %d, want %d", rec.rating, tc.rating)
			}
			if rec.slug != "a" || rec.submissionID != "S1" {
				t.Errorf("slug=%q submissionID=%q", rec.slug, rec.submissionID)
			}
			if m.result.grade != nil {
				t.Error("expected modal to be closed after rating")
			}
			if cmd == nil {
				t.Fatal("expected loadProblemCmd for next due slug")
			}
			loaded, ok := extractMsg[problemLoadedMsg](cmd)
			if !ok {
				t.Fatal("expected problemLoadedMsg in dispatch batch")
			}
			if loaded.problem == nil || loaded.problem.TitleSlug != "b" {
				t.Errorf("loaded slug = %v, want b", loaded.problem)
			}
		})
	}
}

// Enter commits whatever rating is under the cursor. Default cursor is
// Good (idx 2 → rating 3); arrow keys move it before enter.
func TestEnterCommitsCursorRating(t *testing.T) {
	m, fr, _ := modalSetup(t)
	if _, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}); len(fr.records) != 1 || fr.records[0].rating != 3 {
		t.Errorf("default-cursor enter should record rating=3, got %v", fr.records)
	}

	m, fr, _ = modalSetup(t)
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(fr.records) != 1 || fr.records[0].rating != 4 {
		t.Errorf("down+enter should record rating=4 (Easy), got %v", fr.records)
	}

	m, fr, _ = modalSetup(t)
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(fr.records) != 1 || fr.records[0].rating != 1 {
		t.Errorf("up*2+enter should record rating=1 (Again), got %v", fr.records)
	}
}

func TestArrowKeysClampCursor(t *testing.T) {
	m, _, _ := modalSetup(t)
	for i := 0; i < 10; i++ {
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.result.grade.cursor != 3 {
		t.Errorf("cursor should clamp at 3, got %d", m.result.grade.cursor)
	}
	for i := 0; i < 10; i++ {
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	}
	if m.result.grade.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", m.result.grade.cursor)
	}
}

// Esc cancels the modal without recording anything; user lands back on
// the problem screen they came from.
func TestEscCancelsWithoutRecord(t *testing.T) {
	m, fr, _ := modalSetup(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m.result.grade != nil {
		t.Error("expected modal to be closed")
	}
	if len(fr.records) != 0 {
		t.Errorf("expected no Record on esc, got %d", len(fr.records))
	}
	if m.screen != screenProblem {
		t.Errorf("screen = %d, want screenProblem", m.screen)
	}
	if cmd != nil {
		t.Errorf("expected no cmd on esc, got %v", cmd)
	}
}

// In Explore Mode the user lands on the problems screen after rating —
// no auto-advance to "next due", because Explore is a manual flow.
func TestExploreModeAdvancesToProblemsList(t *testing.T) {
	m, fr, _ := modalSetup(t)
	m.reviewMode = false

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})

	if len(fr.records) != 1 || fr.records[0].rating != 3 {
		t.Errorf("expected rating=3 record, got %v", fr.records)
	}
	if m.screen != screenProblems {
		t.Errorf("screen = %d, want screenProblems", m.screen)
	}
	if cmd != nil {
		t.Errorf("expected no cmd in Explore Mode, got %v", cmd)
	}
}

// In Review Mode with no remaining due Problems, the user lands on the
// (now-empty) problems screen rather than reloading the just-rated slug.
func TestReviewModeNoMoreDueFallsBackToList(t *testing.T) {
	m, fr, _ := modalSetup(t)
	m.reviewMode = true
	m.dueSlugs = map[string]bool{"a": true} // only the just-rated slug
	m.problemsAll = []Problem{
		{QuestionFrontendID: "1", Title: "A", TitleSlug: "a"},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})

	if len(fr.records) != 1 {
		t.Errorf("expected rating to be recorded, got %d", len(fr.records))
	}
	if m.screen != screenProblems {
		t.Errorf("screen = %d, want screenProblems", m.screen)
	}
	if cmd != nil {
		t.Errorf("expected no loadProblemCmd when no more due slugs, got %v", cmd)
	}
}

// Rejected verdicts keep the existing flow: standard verdict screen, no
// modal, no Record call. Tests every non-Accepted path.
func TestRejectedShowsStandardResultScreen(t *testing.T) {
	cases := []struct {
		name string
		in   *leetcode.SubmitResult
	}{
		{"wrong answer", &leetcode.SubmitResult{StatusMsg: "Wrong Answer", LastTestcase: "[1,2,3]"}},
		{"compile error", &leetcode.SubmitResult{StatusMsg: "Compile Error", CompileError: "syntax"}},
		{"runtime error", &leetcode.SubmitResult{StatusMsg: "Runtime Error", RuntimeError: "panic"}},
		{"nil", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fc := &fakeClient{}
			fr := newFakeReviews()
			m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
			loadFakeProblems(t, m, []Problem{
				{QuestionFrontendID: "1", Title: "A", TitleSlug: "a"},
			})
			m.currentProblem = &leetcode.ProblemDetail{TitleSlug: "a"}
			_, _ = m.Update(submitResultMsg{result: tc.in})

			if m.result.grade != nil {
				t.Error("expected no modal on non-Accepted")
			}
			if len(fr.records) != 0 {
				t.Errorf("expected no Record, got %d", len(fr.records))
			}
			view := viewResultView(m)
			if !strings.Contains(view, "back to problem") {
				t.Errorf("expected standard footer, got:\n%s", view)
			}
		})
	}
}

// Modal mode swallows screen keys: 'q' does not quit, 'e'/'r'/'s' do
// nothing. The user must explicitly choose 1-4, enter, or esc.
func TestModalSwallowsScreenKeys(t *testing.T) {
	for _, k := range []rune{'q', 'e', 'r', 's', 'n', 'p'} {
		t.Run(string(k), func(t *testing.T) {
			m, fr, _ := modalSetup(t)
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}})
			if cmd != nil {
				t.Errorf("expected no cmd from %q while modal open, got %v", k, cmd)
			}
			if m.result.grade == nil {
				t.Errorf("expected modal still open after %q", k)
			}
			if len(fr.records) != 0 {
				t.Errorf("expected no Record from %q, got %d", k, len(fr.records))
			}
		})
	}
}

func TestRenderSubmitResult(t *testing.T) {
	tests := []struct {
		name        string
		in          *leetcode.SubmitResult
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "nil",
			in:          nil,
			wantContain: []string{"no verdict"},
		},
		{
			name: "accepted",
			in: &leetcode.SubmitResult{
				StatusMsg:      "Accepted",
				TotalCorrect:   58,
				TotalTestcases: 58,
				StatusRuntime:  "0 ms",
			},
			wantContain: []string{"Accepted", "58/58"},
			wantAbsent:  []string{"last failed input"},
		},
		{
			name: "accepted with percentiles",
			in: &leetcode.SubmitResult{
				StatusMsg:         "Accepted",
				TotalCorrect:      58,
				TotalTestcases:    58,
				StatusRuntime:     "3 ms",
				StatusMemory:      "4.2 MB",
				RuntimePercentile: 89.4,
				MemoryPercentile:  71.2,
			},
			wantContain: []string{"Accepted", "beats 89.4%", "beats 71.2%"},
		},
		{
			name: "wrong answer surfaces failing case",
			in: &leetcode.SubmitResult{
				StatusMsg:      "Wrong Answer",
				TotalCorrect:   12,
				TotalTestcases: 58,
				LastTestcase:   "[1,2,3]",
				ExpectedOutput: "[1,2]",
				CodeOutput:     "[1]",
			},
			wantContain: []string{"Wrong Answer", "12 / 58", "last failed input", "[1,2,3]", "your output", "[1]", "expected output", "[1,2]"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderSubmitResult(tc.in)
			for _, s := range tc.wantContain {
				if !strings.Contains(got, s) {
					t.Errorf("missing %q in:\n%s", s, got)
				}
			}
			for _, s := range tc.wantAbsent {
				if strings.Contains(got, s) {
					t.Errorf("unexpected %q in:\n%s", s, got)
				}
			}
		})
	}
}
