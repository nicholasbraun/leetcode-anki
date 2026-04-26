package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"leetcode-anki/internal/leetcode"
)

type problemItem struct {
	q             leetcode.Question
	hasLocalDraft bool
}

func (p problemItem) FilterValue() string {
	return p.q.QuestionFrontendID + " " + p.q.Title
}

// rowGlyph returns the leftmost status indicator for a problem row:
// $ for premium, ✓ for accepted, ~ for in-progress (tried server-side
// or a local draft on disk), · otherwise. Every glyph is a single cell
// so the status column lines up regardless of font width quirks.
func rowGlyph(status *string, hasLocalDraft, paidOnly bool) string {
	if paidOnly {
		return dimStyle.Render(glyphPaid)
	}
	if isAccepted(status) {
		return rowSolvedStyle.Render(glyphSolved)
	}
	if isTried(status) || hasLocalDraft {
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
	glyph := rowGlyph(it.q.Status, it.hasLocalDraft, it.q.PaidOnly)
	statusCell := glyph + " "

	num := fmt.Sprintf("%5s", it.q.QuestionFrontendID+".")
	diff := difficultyGlyph(it.q.Difficulty) + "  " + difficultyLabel(it.q.Difficulty)
	diffW := lipgloss.Width(diff)

	titleStr := it.q.Title
	titleStyleFn := lipgloss.NewStyle()
	if index == m.Index() {
		titleStyleFn = titleStyleFn.Bold(true)
	}

	// Available width for the title is whatever's left after the fixed
	// columns and a 2-space minimum gap before the difficulty.
	leftConsumed := 1 + lipgloss.Width(cursor) + lipgloss.Width(statusCell) + lipgloss.Width(num) + 2
	titleMax := width - leftConsumed - diffW - 2
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
	gap := width - lipgloss.Width(left) - diffW - 1
	if gap < 2 {
		gap = 2
	}
	fmt.Fprint(w, left+strings.Repeat(" ", gap)+diff)
}

func newProblemsList(width, height int, qs []leetcode.Question, listName string, drafts map[string]bool) list.Model {
	items := make([]list.Item, len(qs))
	for i, q := range qs {
		items[i] = problemItem{q: q, hasLocalDraft: drafts[q.TitleSlug]}
	}
	l := list.New(items, problemsDelegate{}, width, height)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	return l
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
					m.problemLoading = true
					return m, loadProblemCmd(m.ctx, m.client, it.q.TitleSlug)
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

	crumbs := breadcrumb(w, "leetcode-anki", "lists", m.currentList.Name)
	count := len(m.problems.Items())
	label := fmt.Sprintf("Problems  (%d)", count)
	foot := footer(w,
		footerItem{"j/k", "move"},
		footerItem{"enter", "open"},
		footerItem{"/", "filter"},
		footerItem{"pgup/pgdn", "scroll"},
		footerItem{"esc", "back"},
		footerItem{"q", "quit"},
	)

	if previewW <= 0 {
		top := divider(w, label, 0, "")
		bot := divider(w, "", 0, "")
		return strings.Join([]string{
			crumbs,
			"",
			top,
			m.problems.View(),
			bot,
			foot,
		}, "\n")
	}

	top := divider(w, label, listW, "╮")
	bot := divider(w, "", listW, "┴")

	listBox := lipgloss.NewStyle().Width(listW).Render(m.problems.View())
	vlines := make([]string, listH)
	for i := range vlines {
		vlines[i] = dividerLineStyle.Render("│")
	}
	vline := strings.Join(vlines, "\n")
	previewBox := lipgloss.NewStyle().
		Width(previewW - 1).
		Padding(0, 1).
		Render(m.preview.view())

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
