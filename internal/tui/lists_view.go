package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"leetcode-anki/internal/leetcode"
)

type listItem struct {
	fav leetcode.FavoriteList
}

func (l listItem) Title() string       { return l.fav.Name }
func (l listItem) Description() string {
	return fmt.Sprintf("%d problems  ·  %s", l.fav.QuestionCount, l.fav.Slug)
}
func (l listItem) FilterValue() string { return l.fav.Name }

func newListsList(width, height int, lists []leetcode.FavoriteList) list.Model {
	items := make([]list.Item, len(lists))
	for i, f := range lists {
		items[i] = listItem{fav: f}
	}
	d := list.NewDefaultDelegate()
	l := list.New(items, d, width, height)
	l.Title = "Your LeetCode lists"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	return l
}

func updateListsView(m *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.listsReady {
		// Lists never loaded (likely an error). Only handle quit/back keys.
		if km, ok := msg.(tea.KeyMsg); ok {
			switch {
			case keyMatch(km, keys.Quit):
				return m, tea.Quit
			case keyMatch(km, keys.Back):
				m.err = nil
				m.listsLoading = true
				return m, loadListsCmd(m.ctx, m.client)
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	if km, ok := msg.(tea.KeyMsg); ok {
		// Skip global key handling while filtering — let the list handle text input.
		if !m.lists.SettingFilter() {
			switch {
			case keyMatch(km, keys.Quit):
				return m, tea.Quit
			case keyMatch(km, keys.Enter):
				if it, ok := m.lists.SelectedItem().(listItem); ok {
					m.currentList = it.fav
					m.problemsLoading = true
					m.err = nil
					return m, loadProblemsCmd(m.ctx, m.client, it.fav.Slug)
				}
			}
		}
	}
	m.lists, cmd = m.lists.Update(msg)
	return m, cmd
}

func viewListsView(m *Model) string {
	if !m.listsReady {
		return helpStyle.Render("(no lists loaded — esc to retry · q to quit)")
	}
	return m.lists.View()
}
