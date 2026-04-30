package tui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"leetcode-anki/internal/sr"
)

type problemItem struct {
	q           Problem
	hasSolution bool
	// badge is the Review-Mode-only annotation rendered between title and
	// difficulty: "new" for KindNew Items, "due Xd ago" for KindDue.
	// Empty in Explore Mode and on rows that aren't part of the session.
	badge string
}

func (p problemItem) FilterValue() string {
	return p.q.QuestionFrontendID + " " + p.q.Title
}

// rowGlyph returns the leftmost status indicator for a problem row:
// $ for premium, ✓ for accepted, ~ for in-progress (tried server-side
// or a local Solution on disk), · otherwise. Every glyph is a single cell
// so the status column lines up regardless of font width quirks.
func rowGlyph(status *string, hasSolution, paidOnly bool) string {
	if paidOnly {
		return dimStyle.Render(glyphPaid)
	}
	if isAccepted(status) {
		return rowSolvedStyle.Render(glyphSolved)
	}
	if isTried(status) || hasSolution {
		return inProgressStyle.Render(glyphTried)
	}
	return dimStyle.Render("·")
}

// LeetCode's status enum has churned across endpoints — accepted problems
// have been observed as "AC", "ACCEPTED", and "FINISH". Match all three.
func isAccepted(status *string) bool {
	if status == nil {
		return false
	}
	switch strings.ToUpper(*status) {
	case "AC", "ACCEPTED", "FINISH", "SOLVED":
		return true
	}
	return false
}

func isTried(status *string) bool {
	if status == nil {
		return false
	}
	switch strings.ToUpper(*status) {
	case "TRIED", "ATTEMPTED", "NOTAC":
		return true
	}
	return false
}

// problemsDelegate renders one problem per row in the borderless minimal
// style. Width comes from the live list.Model so resizes reflow correctly.
// Selected rows show the cursor glyph and bold title; columns stay aligned
// regardless of selection state.
type problemsDelegate struct{}

func (d problemsDelegate) Height() int                             { return 1 }
func (d problemsDelegate) Spacing() int                            { return 0 }
func (d problemsDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d problemsDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(problemItem)
	if !ok {
		return
	}
	width := m.Width()
	cursor := "  "
	if index == m.Index() {
		cursor = breadcrumbActiveStyle.Render(glyphCursor) + " "
	}
	// Every status glyph is single-cell (see TestStatusGlyphsAreSingleCell),
	// so the column is a fixed 2 cells: glyph + one trailing space.
	glyph := rowGlyph(it.q.Status, it.hasSolution, it.q.PaidOnly)
	statusCell := glyph + " "

	num := fmt.Sprintf("%5s", it.q.QuestionFrontendID+".")
	diff := difficultyLabel(it.q.Difficulty)
	diffW := lipgloss.Width(diff)

	titleStr := it.q.Title
	titleStyleFn := lipgloss.NewStyle()
	if index == m.Index() {
		titleStyleFn = titleStyleFn.Bold(true)
	}

	// Available width for the title is whatever's left after the fixed
	// columns, a 2-space minimum gap, and the 1-cell right margin the
	// gap formula below reserves. Keeping these consistent ensures the
	// difficulty's right edge lands on the same column whether or not
	// the title was truncated.
	leftConsumed := 1 + lipgloss.Width(cursor) + lipgloss.Width(statusCell) + lipgloss.Width(num) + 2
	titleMax := width - leftConsumed - diffW - 3
	if titleMax < 8 {
		titleMax = 8
	}
	if lipgloss.Width(titleStr) > titleMax {
		runes := []rune(titleStr)
		if titleMax > 1 {
			titleStr = string(runes[:titleMax-1]) + "…"
		}
	}
	title := titleStyleFn.Render(titleStr)

	left := " " + cursor + statusCell + num + "  " + title

	// Right-side block is "[badge  ]diff" (trailing 2-space spacer included
	// when badge is set), so the difficulty's right edge stays anchored
	// regardless of badge width.
	right := diff
	if it.badge != "" {
		right = dimStyle.Render(it.badge) + "  " + diff
	}
	rightW := lipgloss.Width(right)
	gap := width - lipgloss.Width(left) - rightW - 1
	if gap < 2 {
		gap = 2
	}
	fmt.Fprint(w, left+strings.Repeat(" ", gap)+right)
}

// reviewFooterHint returns the verb that completes the 'v' footer entry,
// flipping with the current mode so the hint always describes the action,
// not the current state.
func reviewFooterHint(reviewMode bool) string {
	if reviewMode {
		return "explore"
	}
	return "review"
}

// rebuildProblemsList re-derives the problems list view from problemsAll
// using the current reviewMode + session. Used when 'v' toggles Review/
// Explore on the problems screen so the user sees the new filter
// applied without losing their place.
func rebuildProblemsList(m *Model) {
	w := m.width
	if w < 20 {
		w = 80
	}
	h := m.height
	if h < 7 {
		h = 24
	}
	lw, lh, _, _ := problemsLayout(w, h)
	visible := visibleProblems(m.problemsAll, m.reviewMode, m.session)
	m.problems = newProblemsList(lw, lh, visible, m.currentList.Name, m.solutionSlugs, sessionBadges(m.session, time.Now()))
}

func newProblemsList(width, height int, qs []Problem, listName string, solutions map[string]bool, badges map[string]string) list.Model {
	items := make([]list.Item, len(qs))
	for i, q := range qs {
		items[i] = problemItem{q: q, hasSolution: solutions[q.TitleSlug], badge: badges[q.TitleSlug]}
	}
	l := list.New(items, problemsDelegate{}, width, height)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	return l
}

// sessionBadges builds the slug → badge-text map for a Review Mode
// session. Returns nil for a nil session so callers can pass the result
// straight through to newProblemsList without an empty-map allocation
// per render. now is captured by the caller and used for "Xd ago"
// formatting; using a snapshot keeps badges stable while the user works
// through a session rather than ticking second-by-second.
func sessionBadges(session *sr.Session, now time.Time) map[string]string {
	if session == nil {
		return nil
	}
	out := make(map[string]string, len(session.Items))
	for _, it := range session.Items {
		switch it.Kind {
		case sr.KindNew:
			out[it.TitleSlug] = "new"
		case sr.KindDue:
			out[it.TitleSlug] = humanizeOverdue(it.NextDue, now)
		}
	}
	return out
}

// humanizeOverdue formats how far in the past a SessionItem's NextDue
// is from now. Inverse of humanizeDue (result_view.go), with shorter
// unit suffixes that fit a per-row badge column.
func humanizeOverdue(nextDue, now time.Time) string {
	d := now.Sub(nextDue)
	switch {
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins < 1 {
			return "due now"
		}
		return fmt.Sprintf("due %dm ago", mins)
	case d < 24*time.Hour:
		return fmt.Sprintf("due %dh ago", int(d.Hours()))
	case d < 14*24*time.Hour:
		return fmt.Sprintf("due %dd ago", int(d.Hours())/24)
	case d < 60*24*time.Hour:
		return fmt.Sprintf("due %dw ago", int(d.Hours())/(24*7))
	default:
		return fmt.Sprintf("due %dmo ago", int(d.Hours())/(24*30))
	}
}

func updateProblemsView(m *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.problemsReady {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch {
			case keyMatch(km, keys.Quit):
				return m, tea.Quit
			case keyMatch(km, keys.Back):
				m.screen = screenLists
				return m, nil
			}
		}
		return m, nil
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		if !m.problems.SettingFilter() {
			switch {
			case keyMatch(km, keys.Quit):
				return m, tea.Quit
			case keyMatch(km, keys.Back):
				m.screen = screenLists
				return m, nil
			case keyMatch(km, keys.Review):
				m.reviewMode = !m.reviewMode
				// First Explore→Review for this list: the session hasn't
				// been computed yet, so re-fire the load to fan out the
				// SR call. Subsequent toggles use the cached session
				// synchronously.
				if m.reviewMode && m.session == nil {
					m.err = nil
					return m, tea.Batch(m.load.Start(KindNeutral, "loading problems"), loadProblemsCmd(m.ctx, m.client, m.cache, m.currentList.Slug, true, m.reviewDue, m.reviewNew, m.reviews))
				}
				rebuildProblemsList(m)
				return m, syncPreviewCursor(m)
			case keyMatch(km, keys.Enter):
				if it, ok := m.problems.SelectedItem().(problemItem); ok {
					if it.q.PaidOnly {
						m.err = fmt.Errorf("%s is premium-only", it.q.Title)
						return m, nil
					}
					m.problemIndex = m.problems.Index()
					m.err = nil
					if cached := m.preview.cached(it.q.TitleSlug); cached != nil {
						return m, deliverProblem(cached)
					}
					return m, tea.Batch(m.load.Start(KindNeutral, "loading problem"), loadProblemCmd(m.ctx, m.client, it.q.TitleSlug))
				}
			case keyMatch(km, keys.PreviewUp), keyMatch(km, keys.PreviewDown):
				return m, m.preview.scrollUpdate(msg)
			}
		}
	}
	var cmd tea.Cmd
	m.problems, cmd = m.problems.Update(msg)
	return m, tea.Batch(cmd, syncPreviewCursor(m))
}

// syncPreviewCursor diffs the list's currently highlighted slug against the
// preview's last-recorded one and arms a debounce tick when they differ.
// Diffing post-update covers every cursor change — arrow keys, page nav,
// type-ahead jumps, mouse wheel — without intercepting individual key events.
func syncPreviewCursor(m *Model) tea.Cmd {
	if !m.problemsReady {
		return nil
	}
	it, ok := m.problems.SelectedItem().(problemItem)
	if !ok {
		m.preview.cursorMoved("", "", false, 0)
		return nil
	}
	if !m.preview.cursorMoved(it.q.TitleSlug, it.q.Title, it.q.PaidOnly, it.q.AcRate) {
		return nil
	}
	return previewTick(it.q.TitleSlug, previewDebounce)
}

func viewProblemsView(m *Model) string {
	w := m.width
	if w <= 0 {
		w = 80
	}
	listW, listH, previewW, _ := problemsLayout(w, m.height)

	var crumbs string
	var label string
	count := len(m.problems.Items())
	if m.reviewMode {
		crumbs = breadcrumb(w, "leetcode-anki", "lists", m.currentList.Name, "review mode")
		label = fmt.Sprintf("Due for review  (%d)", count)
	} else {
		crumbs = breadcrumb(w, "leetcode-anki", "lists", m.currentList.Name)
		label = fmt.Sprintf("Problems  (%d)", count)
	}
	foot := footer(w,
		footerItem{"j/k", "move"},
		footerItem{"enter", "open"},
		footerItem{"v", reviewFooterHint(m.reviewMode)},
		footerItem{"/", "filter"},
		footerItem{"pgup/pgdn", "scroll"},
		footerItem{"esc", "back"},
		footerItem{"q", "quit"},
	)

	// Empty-state in Review Mode: a non-empty list with zero due Problems
	// is meaningfully different from a list that failed to load — render
	// a hint so the user knows to toggle back to Explore Mode.
	listBody := m.problems.View()
	if m.reviewMode && count == 0 && len(m.problemsAll) > 0 {
		listBody = helpStyle.Render("Nothing due in this list — press 'v' to switch to Explore Mode")
	}

	if previewW <= 0 {
		top := divider(w, label, 0, "")
		bot := divider(w, "", 0, "")
		return strings.Join([]string{
			crumbs,
			"",
			top,
			listBody,
			bot,
			foot,
		}, "\n")
	}

	top := divider(w, label, listW, "╮")
	bot := divider(w, "", listW, "┴")

	listBox := lipgloss.NewStyle().Width(listW).Render(listBody)
	vlines := make([]string, listH)
	for i := range vlines {
		vlines[i] = dividerLineStyle.Render("│")
	}
	vline := strings.Join(vlines, "\n")
	previewContent := m.preview.view()
	if m.previewLoad.Active() {
		previewContent = m.previewLoad.Inline()
	}
	previewBox := lipgloss.NewStyle().
		Width(previewW-1).
		Padding(0, 1).
		Render(previewContent)

	middle := lipgloss.JoinHorizontal(lipgloss.Top, listBox, vline, previewBox)

	return strings.Join([]string{
		crumbs,
		"",
		top,
		middle,
		bot,
		foot,
	}, "\n")
}

// problemsLayout splits the screen between the problems list (left) and the
// description preview (right). Returns 0 widths for the preview when the
// terminal is too narrow to fit both panes legibly; the caller should then
// render list-only.
func problemsLayout(width, height int) (listW, listH, previewW, previewH int) {
	listH = height - problemsChromeHeight
	if listH < 5 {
		listH = 20
	}
	previewH = listH
	if width < previewMinTotalWidth {
		return width, listH, 0, 0
	}
	listW = clampInt(width*4/10, previewListMinWidth, previewListMaxWidth)
	previewW = width - listW - previewGap
	if previewW < previewMinPaneWidth {
		return width, listH, 0, 0
	}
	return listW, listH, previewW, previewH
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

const (
	previewMinTotalWidth = 100
	previewListMinWidth  = 30
	previewListMaxWidth  = 60
	previewMinPaneWidth  = 30
	previewGap           = 2

	// problemsChromeHeight is the lines reserved for breadcrumb, blank,
	// top divider, bottom divider, and footer. The list/preview body fills
	// whatever's left.
	problemsChromeHeight = 5
)
