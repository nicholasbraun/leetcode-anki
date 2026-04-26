package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"leetcode-anki/internal/editor"
	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/render"
)

type problemView struct {
	vp           viewport.Model
	rendered     string
	chosenLang   string // langSlug
	pickingLang  bool
	langCursor   int
	scaffoldPath string
	status       *string
	hasDraft     bool
}

func newProblemView(width, height int) problemView {
	vp := viewport.New(width, height)
	return problemView{vp: vp}
}

func (pv *problemView) setProblem(p *leetcode.ProblemDetail, status *string, hasDraft bool, width int) error {
	md, err := render.HTMLToMarkdown(p.Content)
	if err != nil {
		md = p.Content
	}
	out, err := render.MarkdownToTerminal(md, width-4)
	if err != nil {
		return err
	}
	pv.rendered = out
	pv.vp.SetContent(out)
	pv.vp.GotoTop()

	pv.status = status
	pv.hasDraft = hasDraft

	// Default language: first snippet that's golang/python3, else first available.
	pv.chosenLang = pickDefaultLang(p.CodeSnippets)
	pv.pickingLang = false
	pv.langCursor = 0
	pv.scaffoldPath = editor.ExistingSolutionPath(p.TitleSlug, pv.chosenLang)
	return nil
}

func snippetFor(p *leetcode.ProblemDetail, langSlug string) string {
	for _, s := range p.CodeSnippets {
		if s.LangSlug == langSlug {
			return s.Code
		}
	}
	return ""
}

func pickDefaultLang(snippets []leetcode.CodeSnippet) string {
	for _, s := range snippets {
		if s.LangSlug == "golang" {
			return s.LangSlug
		}
	}
	for _, s := range snippets {
		if s.LangSlug == "python3" {
			return s.LangSlug
		}
	}
	if len(snippets) > 0 {
		return snippets[0].LangSlug
	}
	return ""
}

func updateProblemView(m *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	pv := &m.problem

	if pv.pickingLang {
		if km, ok := msg.(tea.KeyMsg); ok {
			snippets := m.currentProblem.CodeSnippets
			switch {
			case keyMatch(km, keys.Up):
				if pv.langCursor > 0 {
					pv.langCursor--
				}
				return m, nil
			case keyMatch(km, keys.Down):
				if pv.langCursor < len(snippets)-1 {
					pv.langCursor++
				}
				return m, nil
			case keyMatch(km, keys.Enter):
				if pv.langCursor < len(snippets) {
					pv.chosenLang = snippets[pv.langCursor].LangSlug
					pv.scaffoldPath = editor.ExistingSolutionPath(m.currentProblem.TitleSlug, pv.chosenLang)
				}
				pv.pickingLang = false
				return m, nil
			case keyMatch(km, keys.Back):
				pv.pickingLang = false
				return m, nil
			}
		}
		return m, nil
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case keyMatch(km, keys.Quit):
			return m, tea.Quit
		case keyMatch(km, keys.Back):
			m.screen = screenProblems
			return m, nil
		case keyMatch(km, keys.NextLang):
			pv.pickingLang = true
			// Position cursor on currently chosen language.
			for i, s := range m.currentProblem.CodeSnippets {
				if s.LangSlug == pv.chosenLang {
					pv.langCursor = i
					break
				}
			}
			return m, nil
		case keyMatch(km, keys.Edit):
			snippet := snippetFor(m.currentProblem, pv.chosenLang)
			path, err := editor.ScaffoldFile(m.currentProblem.TitleSlug, pv.chosenLang, snippet)
			if err != nil {
				m.err = err
				return m, nil
			}
			pv.scaffoldPath = path
			return m, editor.OpenInEditor(path)
		case keyMatch(km, keys.Run):
			if pv.scaffoldPath == "" {
				m.err = fmt.Errorf("nothing to run — press 'e' to write a solution first")
				return m, nil
			}
			m.runLoading = true
			m.err = nil
			cmd, cancel := runCodeCmd(m.ctx, m.client, m.currentProblem, pv.chosenLang, pv.scaffoldPath)
			m.cancelInflight = cancel
			return m, cmd
		case keyMatch(km, keys.Submit):
			if pv.scaffoldPath == "" {
				m.err = fmt.Errorf("nothing to submit — press 'e' to write a solution first")
				return m, nil
			}
			m.submitLoading = true
			m.err = nil
			cmd, cancel := submitCodeCmd(m.ctx, m.client, m.currentProblem, pv.chosenLang, pv.scaffoldPath)
			m.cancelInflight = cancel
			return m, cmd
		case keyMatch(km, keys.NextProb):
			return m, advanceProblem(m, +1)
		case keyMatch(km, keys.PrevProb):
			return m, advanceProblem(m, -1)
		}
	}

	var cmd tea.Cmd
	pv.vp, cmd = pv.vp.Update(msg)
	return m, cmd
}

// advanceProblem moves to the next/prev problem in the current list and loads it.
func advanceProblem(m *Model, delta int) tea.Cmd {
	items := m.problems.Items()
	if len(items) == 0 {
		return nil
	}
	next := m.problemIndex + delta
	if next < 0 || next >= len(items) {
		return nil
	}
	if it, ok := items[next].(problemItem); ok {
		if it.q.PaidOnly {
			m.err = fmt.Errorf("%s is premium-only — skipping", it.q.Title)
			return nil
		}
		m.problemIndex = next
		m.problems.Select(next)
		m.problemLoading = true
		m.err = nil
		return loadProblemCmd(m.ctx, m.client, it.q.TitleSlug)
	}
	return nil
}

func viewProblemView(m *Model) string {
	pv := &m.problem
	if pv.pickingLang {
		return langPickerView(m)
	}

	header := headerStyle.Render(fmt.Sprintf("%s. %s", m.currentProblem.QuestionFrontendID, m.currentProblem.Title))
	difficulty := lipgloss.NewStyle().Padding(0, 1).Render(
		difficultyStyle(m.currentProblem.Difficulty).Render(m.currentProblem.Difficulty),
	)
	statusRow := difficulty
	if badge := statusBadge(pv.status, pv.hasDraft); badge != "" {
		statusRow = lipgloss.JoinHorizontal(lipgloss.Top, difficulty, lipgloss.NewStyle().Padding(0, 1).Render(badge))
	}
	lang := dimStyle.Render(fmt.Sprintf("language: %s", pv.chosenLang))
	scaffold := ""
	if pv.scaffoldPath != "" {
		scaffold = dimStyle.Render(" · " + pv.scaffoldPath)
	}

	help := helpStyle.Render("e edit · l language · r run · s submit · n next · p prev · esc back · q quit")

	var body string
	if pv.rendered == "" {
		body = "(loading description...)"
	} else {
		body = pv.vp.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		statusRow,
		lang+scaffold,
		body,
		help,
	)
}

// statusBadge returns a styled "✓ Solved" / "✎ In progress" label for the
// detail screen, or "" when the problem is untouched. The same signals
// drive the lists-screen glyph (statusGlyph) so the two stay in sync.
func statusBadge(status *string, hasLocalDraft bool) string {
	if status != nil {
		switch strings.ToUpper(*status) {
		case "AC", "ACCEPTED":
			return successStyle.Render("✓ Solved")
		case "TRIED":
			return inProgressStyle.Render("✎ In progress")
		}
	}
	if hasLocalDraft {
		return inProgressStyle.Render("✎ In progress")
	}
	return ""
}

func langPickerView(m *Model) string {
	pv := &m.problem
	var b strings.Builder
	b.WriteString(headerStyle.Render("Pick a language") + "\n\n")
	for i, s := range m.currentProblem.CodeSnippets {
		cursor := "  "
		if i == pv.langCursor {
			cursor = "▶ "
		}
		line := fmt.Sprintf("%s%s (%s)", cursor, s.Lang, s.LangSlug)
		if i == pv.langCursor {
			line = lipgloss.NewStyle().Bold(true).Render(line)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + helpStyle.Render("↑/↓ select · enter confirm · esc cancel"))
	return b.String()
}

