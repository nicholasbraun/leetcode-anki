package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/leetcode/leetcodefake"
	"leetcode-anki/internal/sr"
)

// Pressing 'v' on the lists screen toggles reviewMode without firing any
// load command — Review Mode is now a sticky session flag, not an entry
// into a synthetic "globally due" list.
func TestV_OnListsScreen_TogglesReviewMode(t *testing.T) {
	fc := &leetcodefake.Fake{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.listsReady = true
	m.lists = newListsList(140, 30, []leetcode.FavoriteList{{Slug: "x", Name: "X"}})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if !m.reviewMode {
		t.Error("first 'v' should enable reviewMode")
	}
	if m.load.Active() {
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
	fc := &leetcodefake.Fake{}
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
	if !m.load.Active() || m.load.kind != KindNeutral {
		t.Errorf("loading indicator should be active while the list loads, got active=%v kind=%v", m.load.Active(), m.load.kind)
	}
	if cmd == nil {
		t.Error("expected loadProblemsCmd to be scheduled")
	}
}

// Pressing Back from the problems screen returns to lists with
// reviewMode unchanged — the mode is sticky across navigation.
func TestBack_FromProblems_PreservesReviewMode(t *testing.T) {
	fc := &leetcodefake.Fake{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.reviewMode = true
	m.screen = screenProblems
	m.problemsReady = true
	m.problems = newProblemsList(140, 30, nil, "review", nil, nil)

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !m.reviewMode {
		t.Error("reviewMode must persist across Back")
	}
	if m.screen != screenLists {
		t.Errorf("expected screenLists after back, got %v", m.screen)
	}
}

// Toggling 'v' on the problems screen with the session already cached
// rebuilds the visible list synchronously — no re-fetch.
func TestV_OnProblemsScreen_TogglesAndRefilters(t *testing.T) {
	ac := "AC"
	fc := &leetcodefake.Fake{}
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
	m.session = &sr.Session{
		Items:    []sr.SessionItem{{Kind: sr.KindDue, TitleSlug: "a"}},
		DueCount: 1, DueTotal: 1,
	}
	m.problemsReady = true
	m.screen = screenProblems
	m.problems = newProblemsList(140, 30, []Problem{m.problemsAll[0]}, "X", nil, nil)

	// Review → Explore: full list, no spinner, no re-fetch.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if m.reviewMode {
		t.Error("'v' should toggle reviewMode off")
	}
	if m.load.Active() {
		t.Error("toggling Review→Explore must not trigger a fetch (we already have problemsAll)")
	}
	if got := len(m.problems.Items()); got != 3 {
		t.Errorf("expected 3 items after Explore toggle, got %d", got)
	}

	// Explore → Review: re-filter, still no spinner (session cached).
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if !m.reviewMode {
		t.Error("'v' should toggle reviewMode back on")
	}
	if m.load.Active() {
		t.Error("toggling Explore→Review with cached session must not re-fetch")
	}
	if got := len(m.problems.Items()); got != 1 {
		t.Errorf("expected 1 item after Review toggle, got %d", got)
	}
}

// Toggling into Review Mode on the problems screen when the session has
// never been computed for this list must fire loadProblemsCmd to fetch
// it. Without this, a first-time toggle would render the full list and
// silently fail to filter.
func TestV_OnProblemsScreen_FromExplore_LoadsSession(t *testing.T) {
	ac := "AC"
	fc := &leetcodefake.Fake{Questions: map[string][]Problem{
		"x": {{TitleSlug: "a", Status: &ac}},
	}}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.currentList = leetcode.FavoriteList{Slug: "x", Name: "X"}
	m.reviewMode = false
	m.problemsAll = []Problem{{TitleSlug: "a", Status: &ac}}
	m.session = nil
	m.problemsReady = true
	m.screen = screenProblems
	m.problems = newProblemsList(140, 30, m.problemsAll, "X", nil, nil)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if !m.reviewMode {
		t.Error("'v' should switch into Review Mode")
	}
	if !m.load.Active() || m.load.kind != KindNeutral {
		t.Errorf("first Explore→Review with no session must trigger a load, got active=%v kind=%v", m.load.Active(), m.load.kind)
	}
	if cmd == nil {
		t.Error("expected loadProblemsCmd to be scheduled")
	}
}

// In Review Mode, problemsLoadedMsg with a session must render only the
// Problems referenced in session.Items, in session.Items order.
func TestProblemsLoaded_FiltersToSessionWhenReviewMode(t *testing.T) {
	ac := "AC"
	fc := &leetcodefake.Fake{}
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
		session: &sr.Session{
			Items: []sr.SessionItem{
				{Kind: sr.KindDue, TitleSlug: "c"},
				{Kind: sr.KindNew, TitleSlug: "a"},
			},
			DueCount: 1, NewCount: 1, DueTotal: 1, NewTotal: 1,
		},
	})

	if got := len(m.problems.Items()); got != 2 {
		t.Fatalf("expected 2 visible items in review mode, got %d", got)
	}
	// session order is c then a — visibleProblems must preserve it so
	// the rendered list reads "due first, then new" rather than the
	// underlying Problem List's order.
	want := []string{"c", "a"}
	for i, it := range m.problems.Items() {
		if got := it.(problemItem).q.TitleSlug; got != want[i] {
			t.Errorf("Items[%d] = %q, want %q", i, got, want[i])
		}
	}
	if len(m.problemsAll) != 3 {
		t.Errorf("problemsAll should retain the unfiltered slice, got %d items", len(m.problemsAll))
	}
}

// In Explore Mode, problemsLoadedMsg without a session renders the full list.
func TestProblemsLoaded_NonReview_NoFilter(t *testing.T) {
	fc := &leetcodefake.Fake{}
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

// loadProblemsCmd calls reviews.Session in reviewMode and forwards the
// returned queue on problemsLoadedMsg. The fake Session response stands
// in for what *sr.reviews would compute against UserProgress + cache.
func TestLoadProblemsCmd_PopulatesSession_WhenReviewMode(t *testing.T) {
	ac := "AC"
	fc := &leetcodefake.Fake{Questions: map[string][]Problem{
		"x": {
			{TitleSlug: "a", Status: &ac},
			{TitleSlug: "b", Status: &ac},
		},
	}}
	fr := newFakeReviews()
	fr.sessionResp = sr.Session{
		Items: []sr.SessionItem{
			{Kind: sr.KindDue, TitleSlug: "a"},
			{Kind: sr.KindNew, TitleSlug: "b"},
		},
		DueCount: 1, NewCount: 1, DueTotal: 1, NewTotal: 1,
	}

	cmd := loadProblemsCmd(context.Background(), fc, newFakeCache(), "x", true, 5, 5, fr)
	msg := cmd()
	loaded, ok := msg.(problemsLoadedMsg)
	if !ok {
		t.Fatalf("expected problemsLoadedMsg, got %T", msg)
	}
	if len(loaded.problems) != 2 {
		t.Errorf("expected all 2 problems to load (filter happens at render), got %d", len(loaded.problems))
	}
	if loaded.session == nil {
		t.Fatal("expected session to be populated in reviewMode")
	}
	if len(loaded.session.Items) != 2 {
		t.Errorf("expected 2 SessionItems, got %d", len(loaded.session.Items))
	}
	if loaded.session.Items[0].TitleSlug != "a" || loaded.session.Items[0].Kind != sr.KindDue {
		t.Errorf("Items[0] = %+v, want due-a", loaded.session.Items[0])
	}
}

// loadProblemsCmd leaves session nil when reviewMode is false — no SR
// call at all, so Explore-Mode users don't pay for the SR fan-out.
func TestLoadProblemsCmd_NilSession_WhenExploreMode(t *testing.T) {
	ac := "AC"
	fc := &leetcodefake.Fake{Questions: map[string][]Problem{
		"x": {{TitleSlug: "a", Status: &ac}},
	}}
	fr := newFakeReviews()

	cmd := loadProblemsCmd(context.Background(), fc, newFakeCache(), "x", false, 5, 5, fr)
	msg := cmd()
	loaded, ok := msg.(problemsLoadedMsg)
	if !ok {
		t.Fatalf("expected problemsLoadedMsg, got %T", msg)
	}
	if loaded.session != nil {
		t.Errorf("session should be nil in explore mode, got %+v", loaded.session)
	}
}

// When Review Mode is on and a list has problems but none are due, the
// problems screen must show an explicit empty-state message — a blank
// pane would leave the user wondering whether the load failed.
func TestReviewMode_EmptyState(t *testing.T) {
	ac := "AC"
	fc := &leetcodefake.Fake{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.currentList = leetcode.FavoriteList{Slug: "x", Name: "X"}
	m.reviewMode = true
	m.problemsAll = []Problem{
		{TitleSlug: "a", Title: "A", QuestionFrontendID: "1", Status: &ac},
		{TitleSlug: "b", Title: "B", QuestionFrontendID: "2", Status: &ac},
	}
	m.session = &sr.Session{} // computed but nothing due / no new
	m.problemsReady = true
	m.screen = screenProblems
	m.problems = newProblemsList(140, 30, nil, "X", nil, nil)

	view := viewProblemsView(m)
	if !strings.Contains(strings.ToLower(view), "nothing due") {
		t.Errorf("expected an empty-state message containing 'nothing due'; got:\n%s", view)
	}
}

// In Review Mode, the breadcrumb must say "review mode" so the user knows
// they're not browsing a Problem List.
func TestReviewMode_BreadcrumbReflectsMode(t *testing.T) {
	fc := &leetcodefake.Fake{}
	fr := newFakeReviews()
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fr)
	m.width, m.height = 140, 40
	m.reviewMode = true
	m.screen = screenProblems
	m.problemsReady = true
	m.problems = newProblemsList(140, 30, nil, "review", nil, nil)

	view := viewProblemsView(m)
	if !strings.Contains(strings.ToLower(view), "review mode") {
		t.Errorf("breadcrumb should mention review mode; got:\n%s", view)
	}
}
