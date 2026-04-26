package tui

import (
	"errors"
	"strings"
	"testing"

	"leetcode-anki/internal/leetcode"
)

func newTestPreview(t *testing.T) *previewState {
	t.Helper()
	s := &previewState{}
	s.setSize(60, 20)
	return s
}

func sampleDetail(slug string) *leetcode.ProblemDetail {
	return &leetcode.ProblemDetail{
		QuestionID:         "1",
		QuestionFrontendID: "1",
		Title:              "Two Sum",
		TitleSlug:          slug,
		Difficulty:         "Easy",
		Content:            "<p>Find two numbers that sum to target.</p>",
	}
}

func TestPreviewCursorMovedTriggersFetchForUncached(t *testing.T) {
	s := newTestPreview(t)
	if !s.cursorMoved("two-sum", "Two Sum", false, 50.0) {
		t.Fatal("expected fetch to be needed")
	}
	if s.pending != "two-sum" {
		t.Errorf("pending = %q, want two-sum", s.pending)
	}
}

func TestPreviewCursorMovedSkipsFetchWhenCached(t *testing.T) {
	s := newTestPreview(t)
	s.cache = map[string]*leetcode.ProblemDetail{"two-sum": sampleDetail("two-sum")}
	if s.cursorMoved("two-sum", "Two Sum", false, 50.0) {
		t.Fatal("expected no fetch (already cached)")
	}
	if !strings.Contains(s.view(), "Two Sum") {
		t.Errorf("view missing cached content: %q", s.view())
	}
}

func TestPreviewCursorMovedShortCircuitsPremium(t *testing.T) {
	s := newTestPreview(t)
	if s.cursorMoved("locked", "Locked Problem", true, 0) {
		t.Fatal("expected no fetch for premium")
	}
	if !strings.Contains(s.view(), "premium") {
		t.Errorf("view missing premium notice: %q", s.view())
	}
}

func TestPreviewCursorMovedSameSlugNoop(t *testing.T) {
	s := newTestPreview(t)
	if !s.cursorMoved("two-sum", "Two Sum", false, 50.0) {
		t.Fatal("setup: expected first move to need a fetch")
	}
	if s.cursorMoved("two-sum", "Two Sum", false, 50.0) {
		t.Fatal("expected no fetch when slug unchanged")
	}
}

func TestPreviewTickFiredCancelledByLaterMove(t *testing.T) {
	s := newTestPreview(t)
	s.cursorMoved("a", "A", false, 0)
	s.cursorMoved("b", "B", false, 0)
	if s.tickFired("a") {
		t.Fatal("expected stale tick for 'a' to be discarded")
	}
}

func TestPreviewTickFiredMatchesPending(t *testing.T) {
	s := newTestPreview(t)
	s.cursorMoved("a", "A", false, 0)
	if !s.tickFired("a") {
		t.Fatal("expected tick for current slug to fire fetch")
	}
}

func TestPreviewFetchReturnedForCurrentSlug(t *testing.T) {
	s := newTestPreview(t)
	s.cursorMoved("two-sum", "Two Sum", false, 50.0)
	if !s.fetchReturned("two-sum", sampleDetail("two-sum"), nil) {
		t.Fatal("expected fetch result to drive view (still highlighted)")
	}
	if !strings.Contains(s.view(), "Two Sum") {
		t.Errorf("view missing rendered content: %q", s.view())
	}
}

func TestPreviewFetchReturnedStaleStillCaches(t *testing.T) {
	s := newTestPreview(t)
	s.cursorMoved("a", "A", false, 0)
	s.cursorMoved("b", "B", false, 0)
	if s.fetchReturned("a", sampleDetail("a"), nil) {
		t.Fatal("expected stale fetch to not drive view")
	}
	if s.cache["a"] == nil {
		t.Error("expected stale fetch to still populate cache")
	}
}

func TestPreviewFetchReturnedErrorShownForCurrent(t *testing.T) {
	s := newTestPreview(t)
	s.cursorMoved("two-sum", "Two Sum", false, 0)
	if !s.fetchReturned("two-sum", nil, errors.New("network down")) {
		t.Fatal("expected error to drive view")
	}
	if !strings.Contains(s.view(), "failed to load") {
		t.Errorf("view missing error notice: %q", s.view())
	}
}

func TestPreviewViewEmptyWithoutHighlight(t *testing.T) {
	s := newTestPreview(t)
	if got := s.view(); got != "" {
		t.Errorf("view = %q, want empty", got)
	}
}

func TestPreviewLoadingPlaceholder(t *testing.T) {
	s := newTestPreview(t)
	s.cursorMoved("two-sum", "Two Sum", false, 0)
	if !strings.Contains(s.view(), "loading") {
		t.Errorf("view missing loading placeholder: %q", s.view())
	}
}
