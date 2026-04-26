package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"leetcode-anki/internal/leetcode"
)

type problemItem struct {
	q leetcode.Question
}

func (p problemItem) Title() string {
	status := "·"
	if p.q.Status != nil {
		switch strings.ToUpper(*p.q.Status) {
		case "AC", "ACCEPTED":
			status = successStyle.Render("✓")
		case "TRIED", "NOT_STARTED":
			status = "○"
		}
	}
	premium := ""
	if p.q.PaidOnly {
		premium = dimStyle.Render(" 🔒")
	}
	return fmt.Sprintf("%s %s. %s%s", status, p.q.QuestionFrontendID, p.q.Title, premium)
}

func (p problemItem) Description() string {
	diff := difficultyStyle(p.q.Difficulty).Render(p.q.Difficulty)
	return fmt.Sprintf("%s  ·  %.1f%% AC  ·  %s", diff, p.q.AcRate, p.q.TitleSlug)
}

func (p problemItem) FilterValue() string {
	return p.q.QuestionFrontendID + " " + p.q.Title
}

func newProblemsList(width, height int, qs []leetcode.Question, listName string) list.Model {
	items := make([]list.Item, len(qs))
	for i, q := range qs {
		items[i] = problemItem{q: q}
	}
	d := list.NewDefaultDelegate()
	l := list.New(items, d, width, height)
	l.Title = listName
	l.SetShowStatusBar(true)
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
		m.preview.cursorMoved("", "", false)
		return nil
	}
	if !m.preview.cursorMoved(it.q.TitleSlug, it.q.Title, it.q.PaidOnly) {
		return nil
	}
	return previewTick(it.q.TitleSlug, previewDebounce)
}

func viewProblemsView(m *Model) string {
	listView := m.problems.View()
	listW, _, previewW, _ := problemsLayout(m.width, m.height)
	help := helpStyle.Render("enter open · pgup/pgdn scroll preview · / filter · esc back · q quit")

	if previewW <= 0 {
		return lipgloss.JoinVertical(lipgloss.Left, listView, help)
	}

	previewBox := lipgloss.NewStyle().
		Width(previewW).
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderForeground(lipgloss.Color("241")).
		Render(m.preview.view())

	listBox := lipgloss.NewStyle().Width(listW).Render(listView)
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, listBox, previewBox),
		help,
	)
}

// problemsLayout splits the screen between the problems list (left) and the
// description preview (right). Returns 0 widths for the preview when the
// terminal is too narrow to fit both panes legibly; the caller should then
// render list-only.
func problemsLayout(width, height int) (listW, listH, previewW, previewH int) {
	listH = height - 2
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
)
