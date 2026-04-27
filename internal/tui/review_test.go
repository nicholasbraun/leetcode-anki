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

// Pressing 'v' on the lists screen toggles reviewMode without firing any
// load command — Review Mode is now a sticky session flag, not an entry
// into a synthetic "globally due" list.
func TestV_OnListsScreen_TogglesReviewMode(t *testing.T) {
	fc := &fakeClient{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.listsReady = true
	m.lists = newListsList(140, 30, []leetcode.FavoriteList{{Slug: "x", Name: "X"}})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if !m.reviewMode {
		t.Error("first 'v' should enable reviewMode")
	}
	if m.problemsLoading {
		t.Error("toggling on lists must not trigger a problems load")
	}
	if cmd != nil {
		t.Error("toggling on lists must not return a command")
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if m.reviewMode {
		t.Error("second 'v' should disable reviewMode")
	}
	if cmd != nil {
		t.Error("toggling off must not return a command either")
	}
}

// Pressing Enter on a list while reviewMode is on must keep the flag set
// so the loaded problems are filtered to due ones.
func TestEnter_InReviewMode_PreservesMode(t *testing.T) {
	fc := &fakeClient{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.listsReady = true
	m.reviewMode = true
	m.lists = newListsList(140, 30, []leetcode.FavoriteList{{Slug: "x", Name: "X"}})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.reviewMode {
		t.Error("reviewMode must persist when entering a list")
	}
	if !m.problemsLoading {
		t.Error("problemsLoading should be true while the list loads")
	}
	if cmd == nil {
		t.Error("expected loadProblemsCmd to be scheduled")
	}
}

// Pressing Back from the problems screen returns to lists with
// reviewMode unchanged — the mode is sticky across navigation.
func TestBack_FromProblems_PreservesReviewMode(t *testing.T) {
	fc := &fakeClient{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.reviewMode = true
	m.screen = screenProblems
	m.problemsReady = true
	m.problems = newProblemsList(140, 30, nil, "review", nil)

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !m.reviewMode {
		t.Error("reviewMode must persist across Back")
	}
	if m.screen != screenLists {
		t.Errorf("expected screenLists after back, got %v", m.screen)
	}
}

// Toggling 'v' on the problems screen with dueSlugs already cached
// rebuilds the visible list synchronously — no re-fetch.
func TestV_OnProblemsScreen_TogglesAndRefilters(t *testing.T) {
	ac := "AC"
	fc := &fakeClient{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.currentList = leetcode.FavoriteList{Slug: "x", Name: "X"}
	m.reviewMode = true
	m.problemsAll = []Problem{
		{TitleSlug: "a", Title: "A", QuestionFrontendID: "1", Status: &ac},
		{TitleSlug: "b", Title: "B", QuestionFrontendID: "2", Status: &ac},
		{TitleSlug: "c", Title: "C", QuestionFrontendID: "3", Status: &ac},
	}
	m.dueSlugs = map[string]bool{"a": true}
	m.problemsReady = true
	m.screen = screenProblems
	m.problems = newProblemsList(140, 30, []Problem{m.problemsAll[0]}, "X", nil)

	// Review → Explore: full list, no spinner, no re-fetch.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if m.reviewMode {
		t.Error("'v' should toggle reviewMode off")
	}
	if m.problemsLoading {
		t.Error("toggling Review→Explore must not trigger a fetch (we already have problemsAll)")
	}
	if got := len(m.problems.Items()); got != 3 {
		t.Errorf("expected 3 items after Explore toggle, got %d", got)
	}

	// Explore → Review: re-filter, still no spinner (dueSlugs cached).
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if !m.reviewMode {
		t.Error("'v' should toggle reviewMode back on")
	}
	if m.problemsLoading {
		t.Error("toggling Explore→Review with cached dueSlugs must not re-fetch")
	}
	if got := len(m.problems.Items()); got != 1 {
		t.Errorf("expected 1 item after Review toggle, got %d", got)
	}
}

// Toggling into Review Mode on the problems screen when dueSlugs has
// never been computed for this list must fire loadProblemsCmd to fetch
// it. Without this, a first-time toggle would render the full list and
// silently fail to filter.
func TestV_OnProblemsScreen_FromExplore_LoadsDueSlugs(t *testing.T) {
	ac := "AC"
	fc := &fakeClient{problems: map[string][]Problem{
		"x": {{TitleSlug: "a", Status: &ac}},
	}}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.currentList = leetcode.FavoriteList{Slug: "x", Name: "X"}
	m.reviewMode = false
	m.problemsAll = []Problem{{TitleSlug: "a", Status: &ac}}
	m.dueSlugs = nil
	m.problemsReady = true
	m.screen = screenProblems
	m.problems = newProblemsList(140, 30, m.problemsAll, "X", nil)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if !m.reviewMode {
		t.Error("'v' should switch into Review Mode")
	}
	if !m.problemsLoading {
		t.Error("first Explore→Review with no dueSlugs must trigger a load")
	}
	if cmd == nil {
		t.Error("expected loadProblemsCmd to be scheduled")
	}
}

// In Review Mode, problemsLoadedMsg with a dueSlugs set must render only
// the Problems that are present in dueSlugs.
func TestProblemsLoaded_FiltersToDueWhenReviewMode(t *testing.T) {
	ac := "AC"
	fc := &fakeClient{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.currentList = leetcode.FavoriteList{Slug: "x", Name: "X"}
	m.reviewMode = true

	_, _ = m.Update(problemsLoadedMsg{
		problems: []Problem{
			{TitleSlug: "a", Title: "A", QuestionFrontendID: "1", Status: &ac},
			{TitleSlug: "b", Title: "B", QuestionFrontendID: "2", Status: &ac},
			{TitleSlug: "c", Title: "C", QuestionFrontendID: "3", Status: &ac},
		},
		dueSlugs: map[string]bool{"a": true, "c": true},
	})

	if got := len(m.problems.Items()); got != 2 {
		t.Fatalf("expected 2 visible items in review mode, got %d", got)
	}
	got := map[string]bool{}
	for _, it := range m.problems.Items() {
		got[it.(problemItem).q.TitleSlug] = true
	}
	if !got["a"] || !got["c"] || got["b"] {
		t.Errorf("expected only {a,c} to be visible, got %v", got)
	}
	if len(m.problemsAll) != 3 {
		t.Errorf("problemsAll should retain the unfiltered slice, got %d items", len(m.problemsAll))
	}
}

// In Explore Mode, problemsLoadedMsg without dueSlugs renders the full list.
func TestProblemsLoaded_NonReview_NoFilter(t *testing.T) {
	fc := &fakeClient{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.currentList = leetcode.FavoriteList{Slug: "x", Name: "X"}

	_, _ = m.Update(problemsLoadedMsg{
		problems: []Problem{
			{TitleSlug: "a", Title: "A", QuestionFrontendID: "1"},
			{TitleSlug: "b", Title: "B", QuestionFrontendID: "2"},
		},
	})

	if got := len(m.problems.Items()); got != 2 {
		t.Errorf("expected full list in explore mode, got %d items", got)
	}
}

// loadProblemsCmd computes a dueSlugs map in the goroutine when reviewMode
// is true: only AC slugs whose Status reports Due are included. Non-AC
// slugs short-circuit without a Status call.
func TestLoadProblemsCmd_ComputesDueSlugs_WhenReviewMode(t *testing.T) {
	ac := "AC"
	tried := "TRIED"
	now := time.Now()

	fc := &fakeClient{problems: map[string][]Problem{
		"x": {
			{TitleSlug: "a", Status: &ac},    // AC + due
			{TitleSlug: "b", Status: &tried}, // not AC — skip
			{TitleSlug: "c", Status: &ac},    // AC but not due
			{TitleSlug: "d", Status: nil},    // unsolved
		},
	}}
	fr := newFakeReviews()
	fr.statusBySlug = map[string]sr.Status{
		"a": {Tracked: true, NextDue: now.Add(-time.Hour)},
		"c": {Tracked: true, NextDue: now.Add(time.Hour)},
	}

	cmd := loadProblemsCmd(context.Background(), fc, newFakeCache(), "x", true, fr)
	msg := cmd()
	loaded, ok := msg.(problemsLoadedMsg)
	if !ok {
		t.Fatalf("expected problemsLoadedMsg, got %T", msg)
	}
	if len(loaded.problems) != 4 {
		t.Errorf("expected all 4 problems to load (filter happens at render), got %d", len(loaded.problems))
	}
	if !loaded.dueSlugs["a"] {
		t.Error("a should be in dueSlugs (AC + due)")
	}
	if loaded.dueSlugs["c"] {
		t.Error("c should not be in dueSlugs (AC but not due)")
	}
	if loaded.dueSlugs["b"] || loaded.dueSlugs["d"] {
		t.Error("non-AC slugs should never be in dueSlugs")
	}
}

// loadProblemsCmd returns nil dueSlugs when reviewMode is false — no SR
// calls at all, so Explore-Mode users don't pay for the SR fan-out.
func TestLoadProblemsCmd_NoDueSlugs_WhenExploreMode(t *testing.T) {
	ac := "AC"
	fc := &fakeClient{problems: map[string][]Problem{
		"x": {{TitleSlug: "a", Status: &ac}},
	}}
	fr := newFakeReviews()

	cmd := loadProblemsCmd(context.Background(), fc, newFakeCache(), "x", false, fr)
	msg := cmd()
	loaded, ok := msg.(problemsLoadedMsg)
	if !ok {
		t.Fatalf("expected problemsLoadedMsg, got %T", msg)
	}
	if loaded.dueSlugs != nil {
		t.Errorf("dueSlugs should be nil in explore mode, got %v", loaded.dueSlugs)
	}
}

// When Review Mode is on and a list has problems but none are due, the
// problems screen must show an explicit empty-state message — a blank
// pane would leave the user wondering whether the load failed.
func TestReviewMode_EmptyState(t *testing.T) {
	ac := "AC"
	fc := &fakeClient{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.currentList = leetcode.FavoriteList{Slug: "x", Name: "X"}
	m.reviewMode = true
	m.problemsAll = []Problem{
		{TitleSlug: "a", Title: "A", QuestionFrontendID: "1", Status: &ac},
		{TitleSlug: "b", Title: "B", QuestionFrontendID: "2", Status: &ac},
	}
	m.dueSlugs = map[string]bool{} // computed but nothing due
	m.problemsReady = true
	m.screen = screenProblems
	m.problems = newProblemsList(140, 30, nil, "X", nil)

	view := viewProblemsView(m)
	if !strings.Contains(strings.ToLower(view), "nothing due") {
		t.Errorf("expected an empty-state message containing 'nothing due'; got:\n%s", view)
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
