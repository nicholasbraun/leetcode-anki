package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

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
	var cmd tea.Cmd
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
					m.problemLoading = true
					m.err = nil
					return m, loadProblemCmd(m.ctx, m.client, it.q.TitleSlug)
				}
			}
		}
	}
	m.problems, cmd = m.problems.Update(msg)
	return m, cmd
}

func viewProblemsView(m *Model) string {
	return m.problems.View()
}
