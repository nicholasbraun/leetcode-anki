package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/leetcode/leetcodefake"
)

// shouldConfirmQuit is the predicate gating the confirm-quit modal: a quit
// keypress only opens the modal when there is a live Review-Mode attempt
// on the problem screen. Every other situation quits immediately.
func TestShouldConfirmQuit(t *testing.T) {
	cases := []struct {
		name        string
		screen      screen
		reviewMode  bool
		attemptPath string
		want        bool
	}{
		{"problem + review + attempt → confirm", screenProblem, true, "/tmp/a.go", true},
		{"problem + review + no attempt", screenProblem, true, "", false},
		{"problem + explore + attempt", screenProblem, false, "/tmp/a.go", false},
		{"problem + explore + no attempt", screenProblem, false, "", false},
		{"lists + review + attempt", screenLists, true, "/tmp/a.go", false},
		{"problems + review + attempt", screenProblems, true, "/tmp/a.go", false},
		{"result + review + attempt", screenResult, true, "/tmp/a.go", false},
		{"lists + explore + no attempt", screenLists, false, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &Model{
				screen:     tc.screen,
				reviewMode: tc.reviewMode,
			}
			m.problem.attemptPath = tc.attemptPath
			if got := shouldConfirmQuit(m); got != tc.want {
				t.Errorf("shouldConfirmQuit = %v, want %v", got, tc.want)
			}
		})
	}
}

// keyQuit and keyCtrlC are synthetic key events for the two quit keys.
var (
	keyQuit  = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	keyCtrlC = tea.KeyMsg{Type: tea.KeyCtrlC}
	keyY     = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	keyN     = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
)

// onProblemReviewWithAttempt parks a Model on the problem screen in Review
// Mode with an attempt scaffolded — the only state where shouldConfirmQuit
// is true.
func onProblemReviewWithAttempt(t *testing.T) *Model {
	t.Helper()
	cache := newFakeCache()
	m := onProblemScreen("two-sum", cache, newFakeEditor(), &leetcodefake.Fake{})
	m.reviewMode = true
	m.problem.attemptPath = "/fake/tmp/attempt-1.golang"
	return m
}

// q on the problem screen with a live Review attempt opens the modal
// instead of quitting. No tea.Quit, no inflight cancellation.
func TestQuitDispatch_Q_WithAttempt_OpensModal(t *testing.T) {
	m := onProblemReviewWithAttempt(t)
	_, cmd := m.Update(keyQuit)
	if cmd != nil {
		t.Errorf("expected no cmd from q with live attempt, got %T", cmd)
	}
	if !m.confirmQuit {
		t.Error("expected confirmQuit modal to be open after q")
	}
}

// q without an attempt path quits immediately.
func TestQuitDispatch_Q_WithoutAttempt_Quits(t *testing.T) {
	cache := newFakeCache()
	m := onProblemScreen("two-sum", cache, newFakeEditor(), &leetcodefake.Fake{})
	m.reviewMode = true
	// attemptPath is empty
	_, cmd := m.Update(keyQuit)
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd from q without attempt")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Errorf("expected tea.Quit message, got %T(%v)", msg, msg)
	}
	if m.confirmQuit {
		t.Error("expected modal not to open when no attempt")
	}
}

// q outside of Review Mode quits immediately even with attemptPath set.
func TestQuitDispatch_Q_ExploreMode_Quits(t *testing.T) {
	cache := newFakeCache()
	m := onProblemScreen("two-sum", cache, newFakeEditor(), &leetcodefake.Fake{})
	m.reviewMode = false
	m.problem.attemptPath = "/fake/tmp/attempt-1.golang"
	_, cmd := m.Update(keyQuit)
	if cmd == nil {
		t.Fatal("expected tea.Quit from q in Explore Mode")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Errorf("expected tea.Quit, got %T(%v)", msg, msg)
	}
}

// q on screens other than problem quits immediately.
func TestQuitDispatch_Q_OnListsScreen_Quits(t *testing.T) {
	m := NewModel(context.Background(), &leetcodefake.Fake{}, newFakeCache(), newFakeEditor(), newFakeReviews())
	m.screen = screenLists
	m.listsReady = true
	_, cmd := m.Update(keyQuit)
	if cmd == nil {
		t.Fatal("expected tea.Quit from q on lists screen")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Errorf("expected tea.Quit, got %T(%v)", msg, msg)
	}
}

// ctrl+c with a live attempt opens the modal AND cancels any inflight
// request — different from q which leaves inflight alone.
func TestQuitDispatch_CtrlC_WithAttempt_CancelsAndOpensModal(t *testing.T) {
	m := onProblemReviewWithAttempt(t)
	cancelCalled := false
	m.cancelInflight = func() { cancelCalled = true }

	_, cmd := m.Update(keyCtrlC)
	if cmd != nil {
		t.Errorf("expected no cmd from ctrl+c with live attempt, got %T", cmd)
	}
	if !cancelCalled {
		t.Error("expected cancelInflight to be called by ctrl+c")
	}
	if !m.confirmQuit {
		t.Error("expected confirmQuit modal to be open after ctrl+c")
	}
}

// ctrl+c without a live attempt preserves today's behavior: cancel + quit.
func TestQuitDispatch_CtrlC_WithoutAttempt_CancelsAndQuits(t *testing.T) {
	cache := newFakeCache()
	m := onProblemScreen("two-sum", cache, newFakeEditor(), &leetcodefake.Fake{})
	cancelCalled := false
	m.cancelInflight = func() { cancelCalled = true }

	_, cmd := m.Update(keyCtrlC)
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd from ctrl+c without attempt")
	}
	if !cancelCalled {
		t.Error("expected cancelInflight to be called")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Errorf("expected tea.Quit, got %T(%v)", msg, msg)
	}
	if m.confirmQuit {
		t.Error("expected modal not to open without attempt")
	}
}

// y inside the modal confirms quit and returns tea.Quit.
func TestModal_Y_ConfirmsQuit(t *testing.T) {
	m := onProblemReviewWithAttempt(t)
	m.confirmQuit = true

	_, cmd := m.Update(keyY)
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd from y in modal")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Errorf("expected tea.Quit, got %T(%v)", msg, msg)
	}
}

// esc inside the modal dismisses without quitting.
func TestModal_Esc_Dismisses(t *testing.T) {
	m := onProblemReviewWithAttempt(t)
	m.confirmQuit = true

	_, cmd := m.Update(keyEsc)
	if cmd != nil {
		t.Errorf("expected no cmd from esc in modal, got %T", cmd)
	}
	if m.confirmQuit {
		t.Error("expected modal to be dismissed after esc")
	}
}

// n inside the modal dismisses (mirror of esc).
func TestModal_N_Dismisses(t *testing.T) {
	m := onProblemReviewWithAttempt(t)
	m.confirmQuit = true

	_, cmd := m.Update(keyN)
	if cmd != nil {
		t.Errorf("expected no cmd from n in modal, got %T", cmd)
	}
	if m.confirmQuit {
		t.Error("expected modal to be dismissed after n")
	}
}

// Any other key while the modal is open is a noop. The modal stays up,
// no command is scheduled, no inflight cancellation, no second-q quit.
func TestModal_OtherKeys_AreNoops(t *testing.T) {
	keys := []tea.KeyMsg{
		keyQuit,                                                // second q
		keyCtrlC,                                               // second ctrl+c
		{Type: tea.KeyEnter},                                   // enter
		{Type: tea.KeyUp},                                      // arrow
		{Type: tea.KeyRunes, Runes: []rune{'r'}},               // run
		{Type: tea.KeyRunes, Runes: []rune{'s'}},               // submit
		{Type: tea.KeyRunes, Runes: []rune{'e'}},               // edit
		{Type: tea.KeyRunes, Runes: []rune{'x'}},               // arbitrary
	}
	for _, k := range keys {
		t.Run(k.String(), func(t *testing.T) {
			m := onProblemReviewWithAttempt(t)
			m.confirmQuit = true
			cancelCalled := false
			m.cancelInflight = func() { cancelCalled = true }

			_, cmd := m.Update(k)
			if cmd != nil {
				t.Errorf("expected no cmd from %q in modal, got %T", k.String(), cmd)
			}
			if !m.confirmQuit {
				t.Errorf("expected modal to remain open after %q", k.String())
			}
			if cancelCalled {
				t.Errorf("expected no inflight cancellation from %q", k.String())
			}
		})
	}
}

// Non-key messages while the modal is open propagate to the underlying
// screen update so e.g. submitResultMsg can transition screenProblem →
// screenResult while the modal stays up. Dismissing then lands the user
// on the new screen.
func TestModal_NonKeyMessage_PropagatesToScreen(t *testing.T) {
	m := onProblemReviewWithAttempt(t)
	m.confirmQuit = true

	_, _ = m.Update(submitResultMsg{result: &leetcode.SubmitResult{StatusMsg: "Wrong Answer"}})

	if !m.confirmQuit {
		t.Error("expected modal to remain open while non-key msg processed")
	}
	if m.screen != screenResult {
		t.Errorf("expected screen to transition to screenResult under modal, got %v", m.screen)
	}
}

// View() must replace the underlying screen with the centered modal when
// the modal is open. The substring is tied to the prompt text so the
// assertion isn't brittle against lipgloss styling.
func TestView_ModalOpen_RendersConfirmPrompt(t *testing.T) {
	m := onProblemReviewWithAttempt(t)
	m.width, m.height = 100, 30
	m.confirmQuit = true

	view := m.View()
	if !strings.Contains(view, "really quit?") {
		t.Errorf("expected View to render confirm prompt; got:\n%s", view)
	}
	if !strings.Contains(view, "y") || !strings.Contains(view, "esc") {
		t.Errorf("expected modal footer to mention y / esc; got:\n%s", view)
	}
}
