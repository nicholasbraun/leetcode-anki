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

// keyA is the synthetic 'a' keypress that promotes a failing input.
var keyA = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}

// onSubmitResult parks a Model on the Submit Result screen for two-sum
// with the given SubmitResult. fakeCases is wired in so we can assert
// Add behavior directly.
func onSubmitResult(t *testing.T, sr *leetcode.SubmitResult) (*Model, *fakeCases) {
	t.Helper()
	cache := newFakeCache()
	fc := &leetcodefake.Fake{}
	fcases := newFakeCases()
	m := NewModel(context.Background(), fc, cache, newFakeEditor(), fcases, newFakeReviews())
	m.width, m.height = 140, 40
	m.currentProblem = problemDetailFor("two-sum")
	m.result = resultView{kind: resultSubmit, submit: sr}
	m.screen = screenResult
	return m, fcases
}

// Wrong Answer with a non-empty LastTestcase + 'a' must call Cases.Add
// once with the slug and the failing input — the whole point of the flow.
func TestAKey_WrongAnswer_PromotesLastTestcase(t *testing.T) {
	m, fcases := onSubmitResult(t, &leetcode.SubmitResult{
		StatusMsg:    "Wrong Answer",
		LastTestcase: "[3,2,4]\n6",
	})

	_, _ = m.Update(keyA)

	if len(fcases.addCalls) != 1 {
		t.Fatalf("Add calls = %d, want 1", len(fcases.addCalls))
	}
	got := fcases.addCalls[0]
	if got.Slug != "two-sum" || got.Input != "[3,2,4]\n6" {
		t.Errorf("Add called with %+v, want {two-sum, [3,2,4]\\n6}", got)
	}
}

// Footer hint must appear only when there's something to promote (a
// non-Accepted Submit with a non-empty LastTestcase).
func TestAKey_FooterHintOnlyWhenPromotable(t *testing.T) {
	tests := []struct {
		name        string
		in          *leetcode.SubmitResult
		wantPresent bool
	}{
		{"wrong answer with input", &leetcode.SubmitResult{StatusMsg: "Wrong Answer", LastTestcase: "[1,2]"}, true},
		{"runtime error with input", &leetcode.SubmitResult{StatusMsg: "Runtime Error", RuntimeError: "panic", LastTestcase: "[1,2]"}, true},
		{"accepted (no input)", &leetcode.SubmitResult{StatusMsg: "Accepted"}, false},
		{"compile error with empty input", &leetcode.SubmitResult{StatusMsg: "Compile Error", CompileError: "syntax", LastTestcase: ""}, false},
		{"wrong answer empty input", &leetcode.SubmitResult{StatusMsg: "Wrong Answer", LastTestcase: ""}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := onSubmitResult(t, tc.in)
			view := viewResultView(m)
			has := strings.Contains(view, "add to custom tests")
			if has != tc.wantPresent {
				t.Errorf("hint present=%v, want %v\nview:\n%s", has, tc.wantPresent, view)
			}
		})
	}
}

// 'a' on an Accepted Submit is inert. The rating modal already handles
// the Accepted flow; promoting a non-existent failing input must not
// happen here either.
func TestAKey_AcceptedSubmit_Inert(t *testing.T) {
	m, fcases := onSubmitResult(t, &leetcode.SubmitResult{StatusMsg: "Accepted"})
	// The Accepted path opens the grade modal in production, but in this
	// fixture we don't trigger it — we want to assert the gate, not the
	// modal interaction.

	_, _ = m.Update(keyA)

	if len(fcases.addCalls) != 0 {
		t.Errorf("Add called %d times on Accepted; want 0: %v", len(fcases.addCalls), fcases.addCalls)
	}
}

// Compile Error with no LastTestcase must be inert too — there's nothing
// to promote, and saving "" as a custom case would be garbage.
func TestAKey_CompileErrorNoInput_Inert(t *testing.T) {
	m, fcases := onSubmitResult(t, &leetcode.SubmitResult{
		StatusMsg:    "Compile Error",
		CompileError: "syntax",
	})

	_, _ = m.Update(keyA)

	if len(fcases.addCalls) != 0 {
		t.Errorf("Add called %d times on empty LastTestcase; want 0", len(fcases.addCalls))
	}
}

// A Cases.Add error must surface to the user via m.err so the failure
// is visible — silently swallowing means the user thinks the case was
// saved when it wasn't.
func TestAKey_AddError_SetsModelErr(t *testing.T) {
	m, fcases := onSubmitResult(t, &leetcode.SubmitResult{
		StatusMsg:    "Wrong Answer",
		LastTestcase: "[3,2,4]\n6",
	})
	fcases.addErr = errors.New("disk full")

	_, _ = m.Update(keyA)

	if m.err == nil {
		t.Fatal("m.err = nil, want add error to surface")
	}
	if !strings.Contains(m.err.Error(), "disk full") {
		t.Errorf("m.err = %v, want it to contain 'disk full'", m.err)
	}
}

// Successful Add renders a transient "added" toast near the footer so
// the user sees confirmation. The next keypress clears it.
func TestAKey_SuccessTogglesToast(t *testing.T) {
	m, _ := onSubmitResult(t, &leetcode.SubmitResult{
		StatusMsg:    "Wrong Answer",
		LastTestcase: "[3,2,4]\n6",
	})

	_, _ = m.Update(keyA)

	if !strings.Contains(viewResultView(m), "added") {
		t.Errorf("expected 'added' toast in view:\n%s", viewResultView(m))
	}
}
