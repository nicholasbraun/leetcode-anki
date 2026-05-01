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

type listItem struct {
	fav leetcode.FavoriteList
}

func (l listItem) Title() string       { return l.fav.Name }
func (l listItem) Description() string { return "" }
func (l listItem) FilterValue() string { return l.fav.Name }

// listsDelegate renders a single list row as
//   "▸  <name>            <count> problems"
// in the borderless minimal style. Width comes from the live list.Model so
// resizes reflow correctly. Selected rows show the cursor glyph and render
// the title bold; unselected rows pad the cursor slot with spaces so
// columns stay aligned regardless of selection state.
type listsDelegate struct{}

func (d listsDelegate) Height() int                             { return 1 }
func (d listsDelegate) Spacing() int                            { return 0 }
func (d listsDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d listsDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(listItem)
	if !ok {
		return
	}
	cursor := "  "
	title := it.fav.Name
	if index == m.Index() {
		cursor = breadcrumbActiveStyle.Render(glyphCursor) + " "
		title = lipgloss.NewStyle().Bold(true).Render(title)
	}
	count := dimStyle.Render(fmt.Sprintf("%d problems", it.fav.QuestionCount))
	left := " " + cursor + title
	gap := m.Width() - lipgloss.Width(left) - lipgloss.Width(count) - 2
	if gap < 2 {
		gap = 2
	}
	fmt.Fprint(w, left+strings.Repeat(" ", gap)+count)
}

func newListsList(width, height int, lists []leetcode.FavoriteList) list.Model {
	items := make([]list.Item, len(lists))
	for i, f := range lists {
		items[i] = listItem{fav: f}
	}
	l := list.New(items, listsDelegate{}, width, height)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	return l
}

func updateListsView(m *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.listsReady {
		// Lists never loaded (likely an error). Only handle the back key —
		// quit is dispatched globally in app.go.
		if km, ok := msg.(tea.KeyMsg); ok {
			if keyMatch(km, keys.Back) {
				m.err = nil
				return m, tea.Batch(m.load.Start(KindNeutral, "loading your lists"), loadListsCmd(m.ctx, m.client))
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	if km, ok := msg.(tea.KeyMsg); ok {
		// Skip global key handling while filtering — let the list handle text input.
		if !m.lists.SettingFilter() {
			switch {
			case keyMatch(km, keys.Review):
				m.reviewMode = !m.reviewMode
				return m, nil
			case keyMatch(km, keys.Enter):
				if it, ok := m.lists.SelectedItem().(listItem); ok {
					m.currentList = it.fav
					m.err = nil
					return m, tea.Batch(m.load.Start(KindNeutral, "loading problems"), loadProblemsCmd(m.ctx, m.client, m.cache, it.fav.Slug, m.reviewMode, m.reviewDue, m.reviewNew, m.userIsPremium, m.reviews))
				}
			}
		}
	}
	m.lists, cmd = m.lists.Update(msg)
	return m, cmd
}

func viewListsView(m *Model) string {
	w := m.width
	if w <= 0 {
		w = 80
	}

	var crumbs string
	if m.reviewMode {
		crumbs = breadcrumb(w, "leetcode-anki", "lists", "review mode")
	} else {
		crumbs = breadcrumb(w, "leetcode-anki", "lists")
	}
	top := divider(w, "My Lists", 0, "")
	bottom := divider(w, "", 0, "")
	foot := footer(w,
		footerItem{"j/k", "move"},
		footerItem{"enter", "open"},
		footerItem{"v", reviewFooterHint(m.reviewMode)},
		footerItem{"/", "filter"},
		footerItem{"q", "quit"},
	)

	var body string
	switch {
	case !m.listsReady:
		body = " " + helpStyle.Render("(no lists loaded — esc to retry · q to quit)")
	default:
		body = m.lists.View()
	}

	return strings.Join([]string{
		crumbs,
		"",
		top,
		"",
		body,
		"",
		bottom,
		foot,
	}, "\n")
}

// listsChromeHeight is the number of lines the lists screen reserves around
// the list view (breadcrumb, blank, top divider, blank, body, blank, bottom
// divider, footer). Used to compute the list's available height so the
// chrome doesn't push rows off-screen.
const listsChromeHeight = 7
