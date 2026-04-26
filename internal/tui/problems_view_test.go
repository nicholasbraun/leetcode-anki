package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"leetcode-anki/internal/leetcode"
)

type fakeClient struct {
	details map[string]*leetcode.ProblemDetail
	calls   []string
}

func (f *fakeClient) MyFavoriteLists(ctx context.Context) ([]leetcode.FavoriteList, error) {
	return nil, nil
}
func (f *fakeClient) FavoriteQuestionList(ctx context.Context, slug string, skip, limit int) (*leetcode.FavoriteQuestionListResult, error) {
	return nil, nil
}
func (f *fakeClient) Question(ctx context.Context, titleSlug string) (*leetcode.ProblemDetail, error) {
	f.calls = append(f.calls, titleSlug)
	if d, ok := f.details[titleSlug]; ok {
		return d, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeClient) InterpretSolution(ctx context.Context, slug, lang, qid, code, in string) (*leetcode.RunResult, error) {
	return nil, nil
}
func (f *fakeClient) Submit(ctx context.Context, slug, lang, qid, code string) (*leetcode.SubmitResult, error) {
	return nil, nil
}

func loadFakeProblems(t *testing.T, m *Model, qs []leetcode.Question) {
	t.Helper()
	m.width, m.height = 140, 40
	m.currentList = leetcode.FavoriteList{Slug: "test", Name: "test"}
	// The returned cmd is the initial cursor-sync tea.Tick; we drive ticks
	// manually in these tests so we drop it instead of executing it.
	_, _ = m.Update(problemsLoadedMsg{questions: qs})
}

func TestProblemsScreenDebouncesRapidCursorMoves(t *testing.T) {
	fc := &fakeClient{details: map[string]*leetcode.ProblemDetail{
		"a": sampleDetail("a"), "b": sampleDetail("b"),
		"c": sampleDetail("c"), "d": sampleDetail("d"),
	}}
	m := NewModel(context.Background(), fc)
	loadFakeProblems(t, m, []leetcode.Question{
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
	msg := cmd()
	if _, ok := msg.(previewLoadedMsg); !ok {
		t.Fatalf("expected previewLoadedMsg, got %T", msg)
	}
	if len(fc.calls) != 1 || fc.calls[0] != "d" {
		t.Errorf("Question calls = %v, want [d]", fc.calls)
	}
}

func TestProblemsScreenEnterReusesPreviewCache(t *testing.T) {
	fc := &fakeClient{details: map[string]*leetcode.ProblemDetail{"a": sampleDetail("a")}}
	m := NewModel(context.Background(), fc)
	loadFakeProblems(t, m, []leetcode.Question{
		{QuestionFrontendID: "1", Title: "A", TitleSlug: "a"},
	})

	// Drive the preview pipeline to completion.
	_, cmd := m.Update(previewTickMsg{slug: "a"})
	if cmd == nil {
		t.Fatal("expected fetch to be scheduled")
	}
	loaded := cmd()
	if _, _ = m.Update(loaded); fc.calls[0] != "a" {
		t.Fatalf("expected one preview fetch for 'a', got %v", fc.calls)
	}
	fc.calls = nil

	// Enter on the same slug must not re-fetch.
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter produced no command")
	}
	msg := cmd()
	if _, ok := msg.(problemLoadedMsg); !ok {
		t.Fatalf("expected cache-served problemLoadedMsg, got %T", msg)
	}
	if len(fc.calls) != 0 {
		t.Errorf("expected no Question calls on cache hit, got %v", fc.calls)
	}
}

func TestProblemItemTitleStatusGlyph(t *testing.T) {
	ac := "ACCEPTED"
	acShort := "AC"
	finish := "FINISH"
	tried := "TRIED"
	notStarted := "NOT_STARTED"

	cases := []struct {
		name       string
		status     *string
		draft      bool
		wantGlyph  string
	}{
		{"accepted", &ac, false, "✓"},
		{"AC short", &acShort, false, "✓"},
		{"FINISH variant", &finish, false, "✓"},
		{"accepted with draft still solved", &ac, true, "✓"},
		{"tried", &tried, false, "✎"},
		{"draft only", nil, true, "✎"},
		{"tried and draft", &tried, true, "✎"},
		{"not_started no draft", &notStarted, false, "·"},
		{"nil no draft", nil, false, "·"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			it := problemItem{
				q: leetcode.Question{
					QuestionFrontendID: "1",
					Title:              "Two Sum",
					Status:             tc.status,
				},
				hasLocalDraft: tc.draft,
			}
			if got := it.Title(); !strings.Contains(got, tc.wantGlyph) {
				t.Errorf("Title()=%q, expected glyph %q", got, tc.wantGlyph)
			}
		})
	}
}

func TestProblemsScreenSkipsFetchForPremium(t *testing.T) {
	fc := &fakeClient{}
	m := NewModel(context.Background(), fc)
	loadFakeProblems(t, m, []leetcode.Question{
		{QuestionFrontendID: "1", Title: "Premium", TitleSlug: "p", PaidOnly: true},
	})

	// A tick should not even be scheduled, but if one were synthesized,
	// tickFired would discard it (pending was never set).
	_, cmd := m.Update(previewTickMsg{slug: "p"})
	if cmd != nil {
		t.Errorf("expected premium tick to be discarded, got cmd")
	}
	if len(fc.calls) != 0 {
		t.Errorf("expected zero Question calls for premium, got %v", fc.calls)
	}
}
