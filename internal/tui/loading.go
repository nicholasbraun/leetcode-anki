package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Kind selects the visual treatment for a loading indicator. The neutral
// kind is used for cold-load full-screen takeovers; run/submit get accent
// colors and an elapsed-time clock so a slow LeetCode judge stays
// visually distinct from a hung request.
type Kind int

const (
	KindNeutral Kind = iota
	KindRun
	KindSubmit
)

// Indicator is an animated loading widget shared across every async site
// in the TUI. Idle indicators render to the empty string, so callers can
// embed Inline() / View() unconditionally.
type Indicator struct {
	sp        spinner.Model
	label     string
	kind      Kind
	startedAt time.Time
	now       func() time.Time
}

func NewIndicator() Indicator {
	return Indicator{
		sp:  spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		now: time.Now,
	}
}

// Start activates the indicator with a label and visual kind, returning the
// initial tick cmd that drives the animation. Calling Start while already
// active relabels in place and returns nil — the existing tick chain
// keeps the spinner ticking.
func (i *Indicator) Start(kind Kind, label string) tea.Cmd {
	if i.Active() {
		i.label = label
		i.kind = kind
		i.sp.Style = styleFor(kind)
		return nil
	}
	i.label = label
	i.kind = kind
	i.startedAt = i.now()
	i.sp.Style = styleFor(kind)
	return i.sp.Tick
}

func (i *Indicator) Stop() {
	i.label = ""
	i.startedAt = time.Time{}
}

// Update routes spinner.TickMsg back into this indicator's spinner. Returns
// (false, nil) for messages that aren't ours, so the caller can let other
// indicators or handlers see them. A stale tick that arrives after Stop()
// is consumed without re-arming, terminating the tick loop.
func (i *Indicator) Update(msg tea.Msg) (bool, tea.Cmd) {
	tick, ok := msg.(spinner.TickMsg)
	if !ok {
		return false, nil
	}
	if tick.ID != i.sp.ID() {
		return false, nil
	}
	if !i.Active() {
		return true, nil
	}
	var cmd tea.Cmd
	i.sp, cmd = i.sp.Update(tick)
	return true, cmd
}

func (i Indicator) Active() bool { return i.label != "" }

// Elapsed reports how long the current activation has been running. Zero
// when idle.
func (i Indicator) Elapsed() time.Duration {
	if !i.Active() {
		return 0
	}
	return i.now().Sub(i.startedAt)
}

// View renders a padded full-screen loading message. Empty when idle.
func (i Indicator) View() string {
	if !i.Active() {
		return ""
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(i.Inline())
}

// Inline renders the indicator on a single line for embedding next to
// other UI. Run/submit append an elapsed clock; neutral loads omit it
// because the wait isn't actionable.
func (i Indicator) Inline() string {
	if !i.Active() {
		return ""
	}
	style := styleFor(i.kind)
	body := i.sp.View() + style.Render(i.label)
	if i.kind == KindRun || i.kind == KindSubmit {
		body += "  " + style.Render(formatElapsed(i.Elapsed()))
	}
	return body
}

func styleFor(kind Kind) lipgloss.Style {
	switch kind {
	case KindRun:
		return loadingStyleRun
	case KindSubmit:
		return loadingStyleSubmit
	default:
		return loadingStyle
	}
}

func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	secs := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", secs/60, secs%60)
}
