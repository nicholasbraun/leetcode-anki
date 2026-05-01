package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/leetcode/leetcodefake"
)

// keyX is the synthetic 'x' keypress that arms the remove-by-digit flow.
var keyX = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}

// digitKey synthesizes a digit keypress (e.g. '3') for driving the
// remove-case index selection.
func digitKey(d rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{d}}
}

// onRunResult parks a Model on the Run Result screen for two-sum with the
// given Cases and exampleCount, fakeCases pre-seeded with one Custom so
// removeIdx 0 maps to a real entry. Returns the model and fakeCases.
func onRunResult(t *testing.T, cases []leetcode.RunCase, exampleCount int, customs []string) (*Model, *fakeCases) {
	t.Helper()
	fc := &leetcodefake.Fake{}
	fcases := newFakeCases()
	for _, in := range customs {
		_ = fcases.Add("two-sum", in)
	}
	// Reset addCalls so the test can assert Remove without prior Add noise.
	fcases.addCalls = nil
	m := NewModel(context.Background(), fc, newFakeCache(), newFakeEditor(), fcases, newFakeReviews())
	m.width, m.height = 140, 40
	m.currentProblem = &leetcode.ProblemDetail{TitleSlug: "two-sum", Title: "Two Sum"}
	m.result = resultView{
		kind:         resultRun,
		exampleCount: exampleCount,
		run:          &leetcode.RunResult{CorrectAnswer: true, Cases: cases},
	}
	m.screen = screenResult
	return m, fcases
}

// twoExOneCustomCases returns a 2-Examples + 1-Custom case slice the
// remove tests can share.
func twoExOneCustomCases() []leetcode.RunCase {
	return []leetcode.RunCase{
		{Index: 0, Pass: true},
		{Index: 1, Pass: true},
		{Index: 2, Pass: true},
	}
}

// 'x' then '3' on a 2-Example + 1-Custom view must Remove the Custom by
// its 0-based slot in the Customs list (i.e. 0).
func TestXKey_RemovesCustomByDigit(t *testing.T) {
	m, fcases := onRunResult(t, twoExOneCustomCases(), 2, []string{"[4,5]\n9"})

	_, _ = m.Update(keyX)
	if !m.result.awaitingRemoveDigit {
		t.Fatal("expected awaitingRemoveDigit to be set after 'x'")
	}
	_, _ = m.Update(digitKey('3'))

	if m.result.awaitingRemoveDigit {
		t.Errorf("awaitingRemoveDigit not cleared after digit press")
	}
	if len(fcases.removeCalls) != 1 {
		t.Fatalf("Remove calls = %d, want 1: %v", len(fcases.removeCalls), fcases.removeCalls)
	}
	got := fcases.removeCalls[0]
	if got.Slug != "two-sum" || got.Index != 0 {
		t.Errorf("Remove called with %+v, want {two-sum, 0}", got)
	}
}

// 'x' then '1' (an Example index) must surface a "case 1 is an Example"
// toast and not call Remove.
func TestXKey_DigitOnExample_TostsAndSkipsRemove(t *testing.T) {
	m, fcases := onRunResult(t, twoExOneCustomCases(), 2, []string{"[4,5]\n9"})

	_, _ = m.Update(keyX)
	_, _ = m.Update(digitKey('1'))

	if m.result.awaitingRemoveDigit {
		t.Errorf("awaitingRemoveDigit not cleared after Example digit press")
	}
	if len(fcases.removeCalls) != 0 {
		t.Errorf("Remove called %d times on Example index; want 0: %v", len(fcases.removeCalls), fcases.removeCalls)
	}
	view := viewResultView(m)
	if !strings.Contains(view, "case 1") || !strings.Contains(view, "Example") {
		t.Errorf("expected Example warning toast in view:\n%s", view)
	}
}

// 'x' then esc clears the flag without removing.
func TestXKey_EscClearsFlagWithoutRemove(t *testing.T) {
	m, fcases := onRunResult(t, twoExOneCustomCases(), 2, []string{"[4,5]\n9"})

	_, _ = m.Update(keyX)
	if !m.result.awaitingRemoveDigit {
		t.Fatal("expected awaitingRemoveDigit set")
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m.result.awaitingRemoveDigit {
		t.Errorf("awaitingRemoveDigit not cleared after esc")
	}
	if len(fcases.removeCalls) != 0 {
		t.Errorf("Remove should not be called on esc, got %d calls", len(fcases.removeCalls))
	}
}

// 'x' on a Run Result screen with no cases must be a no-op — the flag
// stays false and the footer hint does not change.
func TestXKey_NoCases_NoOp(t *testing.T) {
	m, _ := onRunResult(t, nil, 0, nil)

	_, _ = m.Update(keyX)

	if m.result.awaitingRemoveDigit {
		t.Errorf("awaitingRemoveDigit should remain false with no cases")
	}
	view := viewResultView(m)
	if strings.Contains(view, "remove case") {
		t.Errorf("footer should not surface remove-case hint with no cases:\n%s", view)
	}
}

// While awaitingRemoveDigit is true, the footer must display the
// '1-9 remove case · esc cancel' hint instead of the default footer items.
func TestXKey_FooterShowsRemoveHintWhileAwaiting(t *testing.T) {
	m, _ := onRunResult(t, twoExOneCustomCases(), 2, []string{"[4,5]\n9"})

	_, _ = m.Update(keyX)

	view := viewResultView(m)
	if !strings.Contains(view, "remove case") {
		t.Errorf("expected 'remove case' hint in footer while awaiting:\n%s", view)
	}
	if !strings.Contains(view, "1-9") {
		t.Errorf("expected '1-9' key hint while awaiting:\n%s", view)
	}
}

// A digit beyond the case count clears the flag and is otherwise a no-op
// (no Remove, no toast).
func TestXKey_DigitOutOfRange_NoOp(t *testing.T) {
	m, fcases := onRunResult(t, twoExOneCustomCases(), 2, []string{"[4,5]\n9"})

	_, _ = m.Update(keyX)
	_, _ = m.Update(digitKey('5'))

	if m.result.awaitingRemoveDigit {
		t.Errorf("awaitingRemoveDigit not cleared after out-of-range digit")
	}
	if len(fcases.removeCalls) != 0 {
		t.Errorf("Remove should not be called for out-of-range index, got %v", fcases.removeCalls)
	}
}

// Cases.Remove returning an error surfaces via m.err so the failure is
// visible to the user; silently swallowing storage errors is a footgun.
func TestXKey_RemoveError_SetsModelErr(t *testing.T) {
	m, fcases := onRunResult(t, twoExOneCustomCases(), 2, []string{"[4,5]\n9"})
	fcases.removeErr = errors.New("disk full")

	_, _ = m.Update(keyX)
	_, _ = m.Update(digitKey('3'))

	if m.err == nil {
		t.Fatal("m.err = nil, want remove error to surface")
	}
	if !strings.Contains(m.err.Error(), "disk full") {
		t.Errorf("m.err = %v, want it to contain 'disk full'", m.err)
	}
}

// A non-digit, non-esc key while awaiting clears the flag and routes the
// key normally — arrows must still scroll the viewport in this state.
func TestXKey_OtherKeyClearsFlagAndRoutesNormally(t *testing.T) {
	m, fcases := onRunResult(t, twoExOneCustomCases(), 2, []string{"[4,5]\n9"})

	_, _ = m.Update(keyX)
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	if m.result.awaitingRemoveDigit {
		t.Errorf("awaitingRemoveDigit should clear after non-digit/non-esc key")
	}
	if len(fcases.removeCalls) != 0 {
		t.Errorf("Remove should not be called on a non-digit key, got %v", fcases.removeCalls)
	}
}
