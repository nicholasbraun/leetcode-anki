package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/sr"
)

// Pressing the Review key from the lists screen must set reviewMode and
// schedule a load. The cmd is async so we don't drain it; the synchronous
// state changes are what we assert here — they're what guarantees the
// breadcrumb flips before the load resolves.
func TestReviewKey_EntersReviewMode(t *testing.T) {
	fc := &fakeClient{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.listsReady = true
	m.lists = newListsList(140, 30, []leetcode.FavoriteList{{Slug: "x", Name: "X"}})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	if !m.reviewMode {
		t.Error("reviewMode should be true after pressing v")
	}
	if !m.problemsLoading {
		t.Error("problemsLoading should be true while load is in flight")
	}
	if cmd == nil {
		t.Error("expected a load cmd to be scheduled")
	}
}

// Selecting a regular Problem List with Enter must clear any prior
// reviewMode so the breadcrumb doesn't lie about which mode the screen
// is rendering.
func TestEnter_ClearsReviewMode(t *testing.T) {
	fc := &fakeClient{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.listsReady = true
	m.reviewMode = true
	m.lists = newListsList(140, 30, []leetcode.FavoriteList{{Slug: "x", Name: "X"}})

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.reviewMode {
		t.Error("reviewMode should be cleared when entering a regular list")
	}
}

// Pressing Back from the problems screen returns to lists and clears
// reviewMode so the next 'v' press starts fresh.
func TestBack_FromProblemsClearsReviewMode(t *testing.T) {
	fc := &fakeClient{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.reviewMode = true
	m.screen = screenProblems
	m.problemsReady = true
	m.problems = newProblemsList(140, 30, nil, "review", nil)

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m.reviewMode {
		t.Error("reviewMode should be cleared on back")
	}
	if m.screen != screenLists {
		t.Errorf("expected screenLists after back, got %v", m.screen)
	}
}

// Status assertion: the load cmd that loadReviewCmd returns must convert
// DueProblem entries into Question rows — that's what the existing
// problems screen expects.
func TestLoadReviewCmd_ConvertsDueProblemsToQuestions(t *testing.T) {
	fr := newFakeReviews()
	fr.dueResp = []sr.DueProblem{
		{TitleSlug: "two-sum", Title: "Two Sum", FrontendID: "1", Difficulty: "EASY", NextDue: time.Now()},
	}
	cmd := loadReviewCmd(context.Background(), fr, newFakeCache())
	msg := cmd()

	loaded, ok := msg.(problemsLoadedMsg)
	if !ok {
		t.Fatalf("expected problemsLoadedMsg, got %T", msg)
	}
	if len(loaded.questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(loaded.questions))
	}
	q := loaded.questions[0]
	if q.TitleSlug != "two-sum" || q.Title != "Two Sum" || q.QuestionFrontendID != "1" {
		t.Errorf("question shape wrong: %+v", q)
	}
	if q.Status == nil || *q.Status != "AC" {
		t.Errorf("Status should be AC for due Problems; got %v", q.Status)
	}
}

// In Review Mode, the breadcrumb must say "review mode" so the user knows
// they're not browsing a Problem List.
func TestReviewMode_BreadcrumbReflectsMode(t *testing.T) {
	fc := &fakeClient{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.reviewMode = true
	m.screen = screenProblems
	m.problemsReady = true
	m.problems = newProblemsList(140, 30, nil, "review", nil)

	view := viewProblemsView(m)
	if !strings.Contains(strings.ToLower(view), "review mode") {
		t.Errorf("breadcrumb should mention review mode; got:\n%s", view)
	}
}
