package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/render"
)

// previewState owns the description-pane shown beside the problems list:
// a per-slug cache, the slug currently under the cursor, the slug that
// debounce is waiting on, and the viewport the rendered description scrolls
// in. The state methods are deliberately pure (no I/O) so the cursor →
// debounce → fetch → render pipeline is unit-testable without Bubble Tea.
type previewState struct {
	cache         map[string]*leetcode.ProblemDetail
	highlighted   string
	highlightTitle string
	highlightLocked bool
	pending       string
	err           error
	vp            viewport.Model
	width         int
	height        int
	ready         bool
}

// cursorMoved records the slug now under the cursor and reports whether the
// caller should schedule a debounced fetch. Returns false when nothing
// changed, when the slug is already cached, or when the problem is premium-
// locked (we can't fetch its content anyway).
func (s *previewState) cursorMoved(slug, title string, paidOnly bool) bool {
	if slug == s.highlighted {
		return false
	}
	s.highlighted = slug
	s.highlightTitle = title
	s.highlightLocked = paidOnly
	s.err = nil
	s.refreshContent()
	if slug == "" || paidOnly {
		return false
	}
	if _, ok := s.cache[slug]; ok {
		return false
	}
	s.pending = slug
	return true
}

// tickFired reports whether the slug that armed the debounce window is still
// the one under the cursor. When false, the user moved on and the fetch
// should be skipped entirely.
func (s *previewState) tickFired(slug string) bool {
	return s.pending == slug && s.highlighted == slug
}

// fetchReturned stores a successful response in the cache (regardless of
// whether the cursor moved on, so revisits are free) and reports whether the
// result is still relevant for rendering. Errors are recorded only when the
// fetch matches the current cursor — stale errors are silently dropped.
func (s *previewState) fetchReturned(slug string, detail *leetcode.ProblemDetail, err error) bool {
	if err == nil && detail != nil {
		if s.cache == nil {
			s.cache = map[string]*leetcode.ProblemDetail{}
		}
		s.cache[slug] = detail
	}
	if s.highlighted != slug {
		return false
	}
	s.err = err
	s.refreshContent()
	return true
}

// cached returns a previously fetched detail or nil. Lets the problems-screen
// enter handler skip the full-detail fetch when the preview already loaded it.
func (s *previewState) cached(slug string) *leetcode.ProblemDetail {
	return s.cache[slug]
}

// setSize updates the pane dimensions and reflows the rendered content so
// long descriptions wrap to the new width.
func (s *previewState) setSize(width, height int) {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	s.width = width
	s.height = height
	if !s.ready {
		s.vp = viewport.New(width, height)
		s.ready = true
	} else {
		s.vp.Width = width
		s.vp.Height = height
	}
	s.refreshContent()
}

// scrollUpdate forwards an input event (typically pgup/pgdn) to the preview
// viewport so the side pane scrolls without disturbing the list cursor.
func (s *previewState) scrollUpdate(msg tea.Msg) tea.Cmd {
	if !s.ready {
		return nil
	}
	var cmd tea.Cmd
	s.vp, cmd = s.vp.Update(msg)
	return cmd
}

// view returns the rendered side-pane content. Empty when no slug is under
// the cursor (e.g. an empty list) so the caller can omit the pane entirely.
func (s *previewState) view() string {
	if !s.ready || s.highlighted == "" {
		return ""
	}
	return s.vp.View()
}

// refreshContent rebuilds the viewport's content from the current state.
// Called whenever something that drives the displayed text changes:
// cursor, cache hit, error, or pane width.
func (s *previewState) refreshContent() {
	if !s.ready {
		return
	}
	s.vp.SetContent(s.contentForCurrent())
	s.vp.GotoTop()
}

func (s *previewState) contentForCurrent() string {
	if s.highlighted == "" {
		return ""
	}
	if s.highlightLocked {
		return dimStyle.Render("Premium problem (locked) — preview unavailable.")
	}
	if p := s.cache[s.highlighted]; p != nil {
		body, err := renderProblemBody(p, s.width-2)
		if err != nil {
			return errorStyle.Render("render error: " + err.Error())
		}
		return body
	}
	if s.err != nil {
		return errorStyle.Render("could not load preview: " + truncateErr(s.err.Error(), 200))
	}
	label := s.highlightTitle
	if label == "" {
		label = s.highlighted
	}
	return dimStyle.Render(fmt.Sprintf("loading description for %s…", label))
}

// renderProblemBody produces the title + difficulty + glamour-rendered
// description shown by both the full-detail screen and the preview pane.
// Kept here so both consumers stay in sync as the markdown pipeline evolves.
func renderProblemBody(p *leetcode.ProblemDetail, width int) (string, error) {
	md, err := render.HTMLToMarkdown(p.Content)
	if err != nil {
		md = p.Content
	}
	header := fmt.Sprintf("# %s. %s\n\n%s\n\n",
		p.QuestionFrontendID, p.Title,
		difficultyStyle(p.Difficulty).Render(p.Difficulty))
	return render.MarkdownToTerminal(header+md, width)
}

// previewTickMsg fires after the debounce window. The slug is the one that
// armed the window; tickFired discards the message if the cursor moved on.
type previewTickMsg struct{ slug string }

// previewLoadedMsg carries the result of a preview fetch back to the model.
type previewLoadedMsg struct {
	slug   string
	detail *leetcode.ProblemDetail
	err    error
}

// previewTick schedules a debounced fetch for the given slug.
func previewTick(slug string, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return previewTickMsg{slug: slug}
	})
}

// loadPreviewCmd fetches a problem's detail for the side pane. Uses a
// shorter timeout than user-driven loads because preview failure is silent
// (it only suppresses a side pane), so giving up faster keeps things snappy.
func loadPreviewCmd(parent context.Context, c LeetcodeClient, slug string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, previewFetchTimeout)
		defer cancel()
		p, err := c.Question(ctx, slug)
		return previewLoadedMsg{slug: slug, detail: p, err: err}
	}
}
