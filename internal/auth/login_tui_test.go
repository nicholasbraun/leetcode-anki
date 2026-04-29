package auth

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// noopFinder returns the canned cookies/error every time.
type noopFinder struct {
	cookies []browserCookie
	err     error
}

func (f noopFinder) FindLeetcodeCookies(_ context.Context) ([]browserCookie, error) {
	return f.cookies, f.err
}

// trackingVerify captures invocations so tests can assert what creds got
// verified, and lets the test pre-program the result.
type trackingVerify struct {
	called []*Credentials
	err    error
}

func (v *trackingVerify) verify(_ context.Context, c *Credentials) error {
	v.called = append(v.called, c)
	return v.err
}

func freshLoginModel(t *testing.T, finder cookieFinder, verify verifyFunc) *loginModel {
	t.Helper()
	return newLoginModel(context.Background(), finder, verify)
}

// drain repeatedly feeds the cmd output back into the model so a chain
// of commands (e.g. browser extract → verify) settles. Returns the
// final model. Bubble Tea's runtime would do this normally; tests
// short-circuit.
func drain(t *testing.T, m *loginModel, cmd tea.Cmd) *loginModel {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			return m
		}
		// Spinner ticks loop forever; ignore them.
		if _, ok := msg.(spinnerTickMsg); ok {
			return m
		}
		next, c := m.Update(msg)
		nm, ok := next.(*loginModel)
		if !ok {
			t.Fatalf("Update returned non-*loginModel: %T", next)
		}
		m = nm
		cmd = c
	}
	return m
}

func TestLogin_Initial_PickerSelected(t *testing.T) {
	m := freshLoginModel(t, noopFinder{}, nil)
	if m.state != statePick {
		t.Errorf("initial state = %v, want statePick", m.state)
	}
	if m.pickIdx != 0 {
		t.Errorf("initial pickIdx = %d, want 0 (browser-extract default)", m.pickIdx)
	}
}

func TestLogin_PickerArrowKeys(t *testing.T) {
	m := freshLoginModel(t, noopFinder{}, nil)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(*loginModel)
	if m.pickIdx != 1 {
		t.Errorf("pickIdx after Down = %d, want 1", m.pickIdx)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(*loginModel)
	if m.pickIdx != 0 {
		t.Errorf("pickIdx after Up = %d, want 0", m.pickIdx)
	}
}

// j/k mirror Down/Up on the picker for vim-key users. Safe here
// because the picker has no text input — j and k can't conflict with
// typing.
func TestLogin_PickerVimKeys(t *testing.T) {
	m := freshLoginModel(t, noopFinder{}, nil)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = next.(*loginModel)
	if m.pickIdx != 1 {
		t.Errorf("pickIdx after j = %d, want 1", m.pickIdx)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = next.(*loginModel)
	if m.pickIdx != 0 {
		t.Errorf("pickIdx after k = %d, want 0", m.pickIdx)
	}
}

// User confirms "Read cookies from browser" → model goes into the
// browser-extract state and runs the finder.
func TestLogin_PickBrowser_TransitionsToExtract(t *testing.T) {
	finder := noopFinder{
		cookies: []browserCookie{
			cookie("firefox", "LEETCODE_SESSION", "sess"),
			cookie("firefox", "csrftoken", "csrf"),
		},
	}
	v := &trackingVerify{}
	m := freshLoginModel(t, finder, v.verify)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*loginModel)
	if m.state != stateBrowserExtract {
		t.Fatalf("state after enter on browser-pick = %v, want stateBrowserExtract", m.state)
	}
	m = drain(t, m, cmd)

	if m.state != stateDone {
		t.Errorf("expected stateDone after happy-path browser+verify, got %v (err=%v)", m.state, m.err)
	}
	if len(v.called) != 1 {
		t.Errorf("verify call count = %d, want 1", len(v.called))
	}
	if m.creds == nil || m.creds.Session != "sess" {
		t.Errorf("creds at done = %+v", m.creds)
	}
}

// User picks paste → model moves to paste form, browser finder is
// never called.
func TestLogin_PickPaste_TransitionsToPasteForm(t *testing.T) {
	finder := noopFinder{err: errors.New("must not be called")}
	m := freshLoginModel(t, finder, nil)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(*loginModel)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*loginModel)

	if m.state != statePaste {
		t.Errorf("state after enter on paste-pick = %v, want statePaste", m.state)
	}
}

// Browser extraction finds nothing → auto-switch to paste mode. The
// user obviously expected something (they picked browser-extract), so
// we shouldn't make them dismiss an error and pick again.
func TestLogin_BrowserExtract_NoCookies_FallsThroughToPaste(t *testing.T) {
	m := freshLoginModel(t, noopFinder{}, nil) // no cookies, no error → ErrNoBrowserCookies
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*loginModel)
	m = drain(t, m, cmd)

	if m.state != statePaste {
		t.Errorf("state after no-cookies extract = %v, want statePaste (auto-fallback)", m.state)
	}
	if m.extractInfo == nil {
		t.Error("expected extractInfo to be populated so paste screen can show why we fell through")
	}
}

// Browser extraction returns a non-sentinel error (cookies were found
// but no single browser had a complete LEETCODE_SESSION + csrftoken
// pair). Still fall through to paste, but carry the reason so the
// paste screen can explain what happened. The user already committed
// to "use browser cookies"; bouncing them back to the picker just to
// re-pick paste is busywork.
func TestLogin_BrowserExtract_HardError_FallsThroughToPasteWithInfo(t *testing.T) {
	finder := noopFinder{
		cookies: []browserCookie{cookie("firefox", "LEETCODE_SESSION", "sess")},
	}
	m := freshLoginModel(t, finder, nil)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*loginModel)
	m = drain(t, m, cmd)

	if m.state != statePaste {
		t.Errorf("state after hard extract error = %v, want statePaste", m.state)
	}
	if m.extractInfo == nil {
		t.Error("expected extractInfo to carry the reason browser extract failed")
	}
}

// Verify failure → back to picker with an error to show. The user
// might have stale cookies in their browser; offering them paste as a
// workaround is the right next step.
func TestLogin_VerifyFailure_ReturnsToPickerWithError(t *testing.T) {
	finder := noopFinder{
		cookies: []browserCookie{
			cookie("firefox", "LEETCODE_SESSION", "sess"),
			cookie("firefox", "csrftoken", "csrf"),
		},
	}
	v := &trackingVerify{err: errors.New("403: csrf invalid")}
	m := freshLoginModel(t, finder, v.verify)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*loginModel)
	m = drain(t, m, cmd)

	if m.state != statePick {
		t.Errorf("state after verify failure = %v, want statePick", m.state)
	}
	if m.err == nil {
		t.Error("expected err to be set after verify failure")
	}
}

// Submitting the paste form runs the same Verify path as browser
// extract, then ends.
func TestLogin_PasteSubmit_RunsVerifyAndExits(t *testing.T) {
	v := &trackingVerify{}
	m := freshLoginModel(t, noopFinder{}, v.verify)

	// Pick paste.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(*loginModel)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*loginModel)

	// Type into the inputs. Paste form has two text inputs that the
	// user fills, then submits.
	m.inputs[0].SetValue("paste-sess")
	m.inputs[1].SetValue("paste-csrf")

	// Submit by feeding the model the synthetic submit message the form
	// produces on enter from the second input.
	next, cmd := m.Update(pasteSubmitMsg{})
	m = next.(*loginModel)
	m = drain(t, m, cmd)

	if m.state != stateDone {
		t.Fatalf("state after paste submit = %v, want stateDone (err=%v)", m.state, m.err)
	}
	if len(v.called) != 1 {
		t.Fatalf("verify call count = %d, want 1", len(v.called))
	}
	if v.called[0].Session != "paste-sess" || v.called[0].CSRF != "paste-csrf" {
		t.Errorf("verify called with %+v, want paste-sess/paste-csrf", v.called[0])
	}
}

// ctrl+c at any point quits with a sentinel error — main.go uses this
// to distinguish "user gave up" from "auth genuinely failed."
func TestLogin_CtrlC_QuitsWithSentinel(t *testing.T) {
	m := freshLoginModel(t, noopFinder{}, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c produced no cmd; expected tea.Quit")
	}
	if !m.quit {
		t.Error("ctrl+c did not flag the model as quit")
	}
}
