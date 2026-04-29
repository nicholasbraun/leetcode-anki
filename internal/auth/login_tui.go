package auth

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// verifyFunc validates a credential pair against leetcode.com. The auth
// package can't import internal/leetcode (cycle), so the caller wires
// in a closure that constructs a leetcode.Client and calls Verify.
type verifyFunc func(context.Context, *Credentials) error

// loginState is the sub-state machine inside the login screen.
type loginState int

const (
	statePick loginState = iota
	stateBrowserExtract
	statePaste
	stateVerifying
	stateDone
)

const (
	pickBrowser = 0
	pickPaste   = 1
)

// ErrLoginCancelled is returned by RunLoginTUI when the user quits the
// login screen without completing it (ctrl+c, esc on picker). Callers
// should treat this as "user said no" rather than as an auth error.
var ErrLoginCancelled = errors.New("login cancelled")

// loginModel is the Bubble Tea model for the pre-app login screen.
type loginModel struct {
	ctx    context.Context
	finder cookieFinder
	verify verifyFunc

	state    loginState
	pickIdx  int
	inputs   [2]textinput.Model
	inputIdx int
	spin     spinner.Model

	// pendingCreds is the credential pair we're currently verifying. Set
	// when transitioning into stateVerifying so verifyCmd can pick it up,
	// and so a verify failure can re-show the picker without dropping the
	// values.
	pendingCreds *Credentials

	creds *Credentials // populated on success; main.go consumes via Result()
	err   error        // most recent error to display on the picker
	// extractInfo carries the reason browser extraction failed (or
	// found nothing) into the paste screen, so the user understands
	// why they're suddenly looking at a paste form instead of a
	// success message.
	extractInfo error

	width, height int
	quit          bool
}

// spinnerTickMsg is the spinner.TickMsg leaking into the model. Aliased
// here only so tests can short-circuit the infinite tick loop without
// importing the spinner package.
type spinnerTickMsg = spinner.TickMsg

// pasteSubmitMsg is emitted when the paste form's enter binding fires
// from the second input. Synthetic so tests can drive the form
// directly without faking key events into bubbles/textinput.
type pasteSubmitMsg struct{}

// browserResultMsg carries the outcome of LoginFromBrowser back to the
// model.
type browserResultMsg struct {
	creds *Credentials
	err   error
}

// verifyResultMsg carries the outcome of the verify callback.
type verifyResultMsg struct {
	creds *Credentials
	err   error
}

func newLoginModel(ctx context.Context, finder cookieFinder, v verifyFunc) *loginModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	sess := textinput.New()
	sess.Placeholder = "LEETCODE_SESSION value"
	sess.CharLimit = 4096
	sess.Width = 60

	csrf := textinput.New()
	csrf.Placeholder = "csrftoken value"
	csrf.CharLimit = 4096
	csrf.Width = 60

	return &loginModel{
		ctx:    ctx,
		finder: finder,
		verify: v,
		state:  statePick,
		spin:   sp,
		inputs: [2]textinput.Model{sess, csrf},
	}
}

func (m *loginModel) Init() tea.Cmd { return m.spin.Tick }

func (m *loginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.quit = true
			return m, tea.Quit
		}
		switch m.state {
		case statePick:
			return m.updatePicker(msg)
		case statePaste:
			return m.updatePaste(msg)
		}

	case browserResultMsg:
		return m.handleBrowserResult(msg)

	case verifyResultMsg:
		return m.handleVerifyResult(msg)

	case pasteSubmitMsg:
		return m.startVerify(&Credentials{
			Session: m.inputs[0].Value(),
			CSRF:    m.inputs[1].Value(),
		})

	case spinnerTickMsg:
		var c tea.Cmd
		m.spin, c = m.spin.Update(msg)
		return m, c
	}
	return m, nil
}

func (m *loginModel) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// j/k mirror Down/Up — picker has no text input so vim keys are
	// unambiguous here.
	switch msg.String() {
	case "k":
		if m.pickIdx > 0 {
			m.pickIdx--
		}
		return m, nil
	case "j":
		if m.pickIdx < 1 {
			m.pickIdx++
		}
		return m, nil
	}
	switch msg.Type {
	case tea.KeyUp:
		if m.pickIdx > 0 {
			m.pickIdx--
		}
		return m, nil
	case tea.KeyDown:
		if m.pickIdx < 1 {
			m.pickIdx++
		}
		return m, nil
	case tea.KeyEnter:
		m.err = nil
		switch m.pickIdx {
		case pickBrowser:
			m.state = stateBrowserExtract
			return m, browserExtractCmd(m.ctx, m.finder)
		case pickPaste:
			return m.enterPasteForm(), nil
		}
	}
	return m, nil
}

func (m *loginModel) enterPasteForm() *loginModel {
	m.state = statePaste
	m.inputIdx = 0
	m.inputs[0].Focus()
	m.inputs[1].Blur()
	return m
}

func (m *loginModel) updatePaste(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.state = statePick
		m.extractInfo = nil
		return m, nil
	case tea.KeyTab, tea.KeyDown:
		m.toggleInputFocus()
		return m, nil
	case tea.KeyShiftTab, tea.KeyUp:
		m.toggleInputFocus()
		return m, nil
	case tea.KeyEnter:
		// Submit only when the user is on the second input AND both
		// fields are non-empty. Enter on the first input advances to
		// the second instead — saves them a tab keystroke.
		if m.inputIdx == 0 {
			m.toggleInputFocus()
			return m, nil
		}
		if m.inputs[0].Value() == "" || m.inputs[1].Value() == "" {
			return m, nil
		}
		return m, func() tea.Msg { return pasteSubmitMsg{} }
	}
	var cmd tea.Cmd
	m.inputs[m.inputIdx], cmd = m.inputs[m.inputIdx].Update(msg)
	return m, cmd
}

func (m *loginModel) toggleInputFocus() {
	m.inputs[m.inputIdx].Blur()
	m.inputIdx = 1 - m.inputIdx
	m.inputs[m.inputIdx].Focus()
}

func (m *loginModel) handleBrowserResult(msg browserResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		// Any browser-extract failure — no cookies, incomplete pair,
		// keychain decryption error, anything — falls through to paste
		// with the reason carried over. The user already committed to
		// "use browser cookies" by picking that option; the most
		// efficient next step is to put them in front of the paste
		// form, not to bounce them back to the picker.
		m.extractInfo = msg.err
		return m.enterPasteForm(), nil
	}
	return m.startVerify(msg.creds)
}

func (m *loginModel) startVerify(c *Credentials) (tea.Model, tea.Cmd) {
	m.pendingCreds = c
	m.state = stateVerifying
	return m, verifyCmd(m.ctx, m.verify, c)
}

func (m *loginModel) handleVerifyResult(msg verifyResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.state = statePick
		m.err = fmt.Errorf("verify: %w", msg.err)
		return m, nil
	}
	m.creds = msg.creds
	m.state = stateDone
	return m, tea.Quit
}

func browserExtractCmd(ctx context.Context, finder cookieFinder) tea.Cmd {
	return func() tea.Msg {
		c, err := LoginFromBrowser(ctx, finder)
		return browserResultMsg{creds: c, err: err}
	}
}

func verifyCmd(ctx context.Context, v verifyFunc, c *Credentials) tea.Cmd {
	return func() tea.Msg {
		if v == nil {
			return verifyResultMsg{creds: c}
		}
		err := v(ctx, c)
		return verifyResultMsg{creds: c, err: err}
	}
}

// --- view ---

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	descStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	infoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
)

func (m *loginModel) View() string {
	switch m.state {
	case statePick:
		return m.viewPick()
	case stateBrowserExtract:
		return m.spin.View() + " Reading cookies from your local browsers…"
	case statePaste:
		return m.viewPaste()
	case stateVerifying:
		return m.spin.View() + " Verifying with leetcode.com…"
	case stateDone:
		return ""
	}
	return ""
}

func (m *loginModel) viewPick() string {
	var b []byte
	b = append(b, titleStyle.Render("LeetCode authentication")...)
	b = append(b, "\n\n"...)
	b = append(b, "Choose how to log in:\n\n"...)

	options := []struct {
		label string
		desc  string
	}{
		{
			"Read cookies from your browser",
			"Reads LEETCODE_SESSION + csrftoken from your local browser cookie\nstore (Firefox, Chrome, Safari, Edge, Brave). No browser is launched.\n\nOn macOS, Chrome may prompt for your login password to unlock the\nsystem keychain (which holds the cookie encryption key). On Linux,\nlibsecret may prompt similarly. Firefox and Safari work without prompts.",
		},
		{
			"Paste cookies manually",
			"Open https://leetcode.com in a logged-in browser, then in devtools:\nApplication / Storage → Cookies → https://leetcode.com.\nCopy the LEETCODE_SESSION and csrftoken values. You'll paste them\non the next screen.",
		},
	}
	for i, o := range options {
		marker := "  "
		label := o.label
		if i == m.pickIdx {
			marker = "▸ "
			label = selectedStyle.Render(o.label)
		}
		b = append(b, marker...)
		b = append(b, label...)
		b = append(b, '\n')
		b = append(b, descStyle.Render(indent(o.desc, "    "))...)
		b = append(b, "\n\n"...)
	}
	if m.err != nil {
		b = append(b, errStyle.Render("error: "+m.err.Error())...)
		b = append(b, "\n\n"...)
	}
	b = append(b, hintStyle.Render("↑/↓ or j/k select • enter confirm • ctrl+c quit")...)
	return string(b)
}

func (m *loginModel) viewPaste() string {
	var b []byte
	b = append(b, titleStyle.Render("Paste your LeetCode cookies")...)
	b = append(b, "\n\n"...)

	if m.extractInfo != nil {
		b = append(b, infoStyle.Render("Browser extraction couldn't find usable cookies:")...)
		b = append(b, '\n')
		b = append(b, infoStyle.Render("  "+m.extractInfo.Error())...)
		b = append(b, '\n')
		if path := LoginDebugLogPath(); path != "" {
			b = append(b, hintStyle.Render("  see "+path+" for the cookie stores kooky enumerated")...)
			b = append(b, '\n')
		}
		b = append(b, '\n')
	}

	b = append(b, descStyle.Render("Open https://leetcode.com in a logged-in browser. In devtools:")...)
	b = append(b, '\n')
	b = append(b, descStyle.Render("Application / Storage → Cookies → https://leetcode.com — copy the values.")...)
	b = append(b, "\n\n"...)

	labels := [...]string{"LEETCODE_SESSION:", "csrftoken:       "}
	for i := range m.inputs {
		b = append(b, labels[i]...)
		b = append(b, ' ')
		b = append(b, m.inputs[i].View()...)
		b = append(b, '\n')
	}
	b = append(b, '\n')
	if m.err != nil {
		b = append(b, errStyle.Render("error: "+m.err.Error())...)
		b = append(b, "\n\n"...)
	}
	b = append(b, hintStyle.Render("tab move • enter submit (on second field) • esc back • ctrl+c quit")...)
	return string(b)
}

func indent(s, prefix string) string {
	out := []byte(prefix)
	for i := 0; i < len(s); i++ {
		out = append(out, s[i])
		if s[i] == '\n' {
			out = append(out, prefix...)
		}
	}
	return string(out)
}

// RunLoginTUI runs the interactive login screen and returns the
// verified credentials. The caller persists them. The verify callback
// validates a credential pair against leetcode.com — passed in as a
// function to avoid an import cycle (auth → leetcode → auth).
//
// Returns ErrLoginCancelled if the user ctrl+c's out without completing
// login.
func RunLoginTUI(ctx context.Context, v verifyFunc) (*Credentials, error) {
	return runLoginTUI(ctx, KookyFinder{DebugLog: openLoginDebugLog()}, v)
}

func runLoginTUI(ctx context.Context, finder cookieFinder, v verifyFunc) (*Credentials, error) {
	m := newLoginModel(ctx, finder, v)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm, ok := final.(*loginModel)
	if !ok {
		return nil, fmt.Errorf("login tui: unexpected final model type %T", final)
	}
	if fm.creds != nil {
		return fm.creds, nil
	}
	if fm.err != nil {
		return nil, fm.err
	}
	return nil, ErrLoginCancelled
}

// openLoginDebugLog returns a writer for the truncate-on-each-run
// diagnostic log, or nil if the file can't be opened (read-only cache
// dir, etc.). Login still works without it; only diagnostics are lost.
func openLoginDebugLog() io.Writer {
	w, _ := openTruncatedFile("login-debug.log")
	return w
}
