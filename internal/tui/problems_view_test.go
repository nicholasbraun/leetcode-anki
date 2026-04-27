package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/leetcode/leetcodefake"
)

func loadFakeProblems(t *testing.T, m *Model, qs []Problem) {
	t.Helper()
	m.width, m.height = 140, 40
	m.currentList = leetcode.FavoriteList{Slug: "test", Name: "test"}
	// The returned cmd is the initial cursor-sync tea.Tick; we drive ticks
	// manually in these tests so we drop it instead of executing it.
	_, _ = m.Update(problemsLoadedMsg{problems: qs})
}

func TestProblemsScreenDebouncesRapidCursorMoves(t *testing.T) {
	fc := &leetcodefake.Fake{Details: map[string]*leetcode.ProblemDetail{
		"a": sampleDetail("a"), "b": sampleDetail("b"),
		"c": sampleDetail("c"), "d": sampleDetail("d"),
	}}
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), newFakeReviews())
	loadFakeProblems(t, m, []Problem{
		{QuestionFrontendID: "1", Title: "A", TitleSlug: "a"},
		{QuestionFrontendID: "2", Title: "B", TitleSlug: "b"},
		{QuestionFrontendID: "3", Title: "C", TitleSlug: "c"},
		{QuestionFrontendID: "4", Title: "D", TitleSlug: "d"},
	})

	// Move cursor a → b → c → d.
	for i := 0; i < 3; i++ {
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}

	// Stale ticks for slugs the cursor has passed must be discarded.
	for _, slug := range []string{"a", "b", "c"} {
		_, cmd := m.Update(previewTickMsg{slug: slug})
		if cmd != nil {
			t.Errorf("stale tick for %q returned a command (should be discarded)", slug)
		}
	}

	// The tick for the current slug fires the fetch.
	_, cmd := m.Update(previewTickMsg{slug: "d"})
	if cmd == nil {
		t.Fatal("expected current-slug tick to schedule a fetch")
	}
	if _, ok := extractMsg[previewLoadedMsg](cmd); !ok {
		t.Fatal("expected previewLoadedMsg in dispatch batch")
	}
	if len(fc.DetailCalls) != 1 || fc.DetailCalls[0] != "d" {
		t.Errorf("ProblemDetail calls = %v, want [d]", fc.DetailCalls)
	}
}

func TestProblemsScreenEnterReusesPreviewCache(t *testing.T) {
	fc := &leetcodefake.Fake{Details: map[string]*leetcode.ProblemDetail{"a": sampleDetail("a")}}
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), newFakeReviews())
	loadFakeProblems(t, m, []Problem{
		{QuestionFrontendID: "1", Title: "A", TitleSlug: "a"},
	})

	// Drive the preview pipeline to completion.
	_, cmd := m.Update(previewTickMsg{slug: "a"})
	if cmd == nil {
		t.Fatal("expected fetch to be scheduled")
	}
	loaded, ok := extractMsg[previewLoadedMsg](cmd)
	if !ok {
		t.Fatal("expected previewLoadedMsg in dispatch batch")
	}
	_, _ = m.Update(loaded)
	if len(fc.DetailCalls) == 0 || fc.DetailCalls[0] != "a" {
		t.Fatalf("expected one preview fetch for 'a', got %v", fc.DetailCalls)
	}
	fc.DetailCalls = nil

	// Enter on the same slug must not re-fetch.
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter produced no command")
	}
	if _, ok := extractMsg[problemLoadedMsg](cmd); !ok {
		t.Fatal("expected cache-served problemLoadedMsg")
	}
	if len(fc.DetailCalls) != 0 {
		t.Errorf("expected no ProblemDetail calls on cache hit, got %v", fc.DetailCalls)
	}
}

func TestRowGlyph(t *testing.T) {
	ac := "ACCEPTED"
	acShort := "AC"
	finish := "FINISH"
	tried := "TRIED"
	notStarted := "NOT_STARTED"

	cases := []struct {
		name      string
		status    *string
		solution  bool
		paid      bool
		wantGlyph string
	}{
		{"accepted", &ac, false, false, "✓"},
		{"AC short", &acShort, false, false, "✓"},
		{"FINISH variant", &finish, false, false, "✓"},
		{"accepted with solution still solved", &ac, true, false, "✓"},
		{"tried", &tried, false, false, "~"},
		{"solution only", nil, true, false, "~"},
		{"tried and solution", &tried, true, false, "~"},
		{"not_started no solution", &notStarted, false, false, "·"},
		{"nil no solution", nil, false, false, "·"},
		{"premium overrides everything", &ac, true, true, "$"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rowGlyph(tc.status, tc.solution, tc.paid)
			if !strings.Contains(got, tc.wantGlyph) {
				t.Errorf("rowGlyph()=%q, expected glyph %q", got, tc.wantGlyph)
			}
		})
	}
}

func TestSubmitAcceptedMarksProblemSolved(t *testing.T) {
	fc := &leetcodefake.Fake{}
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), newFakeReviews())
	loadFakeProblems(t, m, []Problem{
		{QuestionFrontendID: "1", Title: "A", TitleSlug: "a"},
	})
	m.currentProblem = &leetcode.ProblemDetail{TitleSlug: "a"}

	_, _ = m.Update(submitResultMsg{result: &leetcode.SubmitResult{StatusMsg: "Accepted"}})

	pi, ok := m.problems.Items()[0].(problemItem)
	if !ok {
		t.Fatal("item 0 not a problemItem")
	}
	if !isAccepted(pi.q.Status) {
		t.Errorf("list item Status not solved; got %v", pi.q.Status)
	}
	if !isAccepted(m.problem.status) {
		t.Errorf("detail-view status not solved; got %v", m.problem.status)
	}
}

func TestSubmitWrongAnswerDoesNotMarkSolved(t *testing.T) {
	fc := &leetcodefake.Fake{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	loadFakeProblems(t, m, []Problem{
		{QuestionFrontendID: "1", Title: "A", TitleSlug: "a"},
	})
	m.currentProblem = &leetcode.ProblemDetail{TitleSlug: "a"}

	_, _ = m.Update(submitResultMsg{result: &leetcode.SubmitResult{StatusMsg: "Wrong Answer"}})

	pi, ok := m.problems.Items()[0].(problemItem)
	if !ok {
		t.Fatal("item 0 not a problemItem")
	}
	if isAccepted(pi.q.Status) {
		t.Errorf("list item incorrectly marked solved on wrong answer: %v", pi.q.Status)
	}
	// Wrong Answer must not enter SR; otherwise non-Accepted submissions
	// would seed a baseline Review and the scheduler would think a
	// failed problem is "solved enough to review".
	if len(fr.records) != 0 {
		t.Errorf("expected no SR Record on Wrong Answer; got %d", len(fr.records))
	}
}

// On Accepted, the verdict-detection site no longer eagerly Records:
// the rating modal owns the Record call so the rating reflects the
// user's actual grade, not the system's "Accepted = Good" guess.
func TestSubmitAcceptedDefersRecordToRatingModal(t *testing.T) {
	fc := &leetcodefake.Fake{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	loadFakeProblems(t, m, []Problem{
		{QuestionFrontendID: "1", Title: "A", TitleSlug: "a"},
	})
	m.currentProblem = &leetcode.ProblemDetail{TitleSlug: "a"}

	_, _ = m.Update(submitResultMsg{result: &leetcode.SubmitResult{
		StatusMsg:    "Accepted",
		SubmissionID: "1988694277",
	}})

	if len(fr.records) != 0 {
		t.Errorf("expected 0 SR Record calls (deferred to rating modal), got %d", len(fr.records))
	}
	if m.result.grade == nil {
		t.Fatal("expected rating modal to be open after Accepted submit")
	}
}

func TestProblemsScreenSkipsFetchForPremium(t *testing.T) {
	fc := &leetcodefake.Fake{}
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), newFakeReviews())
	loadFakeProblems(t, m, []Problem{
		{QuestionFrontendID: "1", Title: "Premium", TitleSlug: "p", PaidOnly: true},
	})

	// A tick should not even be scheduled, but if one were synthesized,
	// tickFired would discard it (pending was never set).
	_, cmd := m.Update(previewTickMsg{slug: "p"})
	if cmd != nil {
		t.Errorf("expected premium tick to be discarded, got cmd")
	}
	if len(fc.DetailCalls) != 0 {
		t.Errorf("expected zero ProblemDetail calls for premium, got %v", fc.DetailCalls)
	}
}

// TestProblemRowDifficultyRightAligned guards two row-render invariants:
// the difficulty word's right edge stays on the same column whether or
// not the title was truncated, and no difficulty icon (◔/◑/●) leaks back
// into the row. The truncated case used to push one cell further right
// because titleMax and the gap formula targeted different right edges.
func TestProblemRowDifficultyRightAligned(t *testing.T) {
	const width = 50
	qs := []Problem{
		{QuestionFrontendID: "1", Title: "A", TitleSlug: "a", Difficulty: "Easy"},
		{QuestionFrontendID: "424", Title: "Longest Repeating Character Replacement", TitleSlug: "lrcr", Difficulty: "Medium"},
	}
	l := newProblemsList(width, 20, qs, "test", nil)
	d := problemsDelegate{}

	var short, long strings.Builder
	d.Render(&short, l, 0, l.Items()[0])
	d.Render(&long, l, 1, l.Items()[1])

	if sw, lw := lipgloss.Width(short.String()), lipgloss.Width(long.String()); sw != lw {
		t.Errorf("row widths differ: short=%d long=%d (truncation must not shift the difficulty column)", sw, lw)
	}
	for _, glyph := range []string{"◔", "◑", "●"} {
		if strings.Contains(short.String(), glyph) || strings.Contains(long.String(), glyph) {
			t.Errorf("row contains stripped difficulty glyph %q", glyph)
		}
	}
}

// TestRowGlyphRendersSingleCell guards the column-alignment invariant:
// problemsDelegate.Render assumes every styled status glyph is exactly
// one cell wide. Banner styles like successStyle add horizontal padding
// that silently inflates the cell — this test catches that regression.
func TestRowGlyphRendersSingleCell(t *testing.T) {
	ac := "ACCEPTED"
	tried := "TRIED"
	cases := []struct {
		name     string
		status   *string
		solution bool
		paid     bool
	}{
		{"solved", &ac, false, false},
		{"tried", &tried, false, false},
		{"solution only", nil, true, false},
		{"paid", nil, false, true},
		{"default", nil, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rowGlyph(tc.status, tc.solution, tc.paid)
			if w := lipgloss.Width(got); w != 1 {
				t.Errorf("rowGlyph width=%d, want 1; raw=%q", w, got)
			}
		})
	}
}
