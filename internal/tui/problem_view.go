package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"leetcode-anki/internal/editor"
	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/render"
)

type problemView struct {
	cache SolutionCache

	vp               viewport.Model
	rendered         string
	chosenLang       string // langSlug
	pickingLang      bool
	langCursor       int
	solutionPath     string
	// attemptPath is the path the user is editing in Review Mode. Scaffolded
	// from the language snippet on first 'e' so the canonical solutionPath
	// (which contains the answer) is never opened in $EDITOR. Reset per
	// Problem so each Review starts with a fresh attempt buffer.
	attemptPath      string
	status           *string
	hasSolution      bool
	solutionVP       viewport.Model
	solutionRendered string
}

// editPath returns the path the user is currently editing — the canonical
// Solution in Explore Mode, the scratch attempt in Review Mode. Empty when
// no scaffold has happened yet on this Problem.
func (pv *problemView) editPath(reviewMode bool) string {
	if reviewMode {
		return pv.attemptPath
	}
	return pv.solutionPath
}

func newProblemView(cache SolutionCache, width, height int) problemView {
	return problemView{
		cache:      cache,
		vp:         viewport.New(width, height),
		solutionVP: viewport.New(0, 0),
	}
}

func (pv *problemView) setProblem(p *leetcode.ProblemDetail, status *string, hasSolution bool, totalWidth, totalHeight int) error {
	pv.status = status
	pv.hasSolution = hasSolution
	pv.chosenLang = pv.pickDefaultLang(p.CodeSnippets, p.TitleSlug)
	pv.pickingLang = false
	pv.langCursor = 0
	return pv.renderForLayout(p, totalWidth, totalHeight)
}

// renderForLayout (re-)renders the description and cached-solution panes for
// the current chosenLang and overall window dimensions. Used on initial load,
// language switches, window resizes, and post-edit refreshes.
func (pv *problemView) renderForLayout(p *leetcode.ProblemDetail, totalWidth, totalHeight int) error {
	pv.solutionPath = pv.cache.ExistingPath(p.TitleSlug, pv.chosenLang)
	descW, descH, solW, solH := problemDetailLayout(totalWidth, totalHeight)

	md, err := render.HTMLToMarkdown(p.Content)
	if err != nil {
		md = p.Content
	}
	out, err := render.MarkdownToTerminal(md, descW-4)
	if err != nil {
		return err
	}
	pv.rendered = out
	pv.vp.Width = descW
	pv.vp.Height = descH
	pv.vp.SetContent(out)
	pv.vp.GotoTop()

	pv.solutionVP.Width = solW
	pv.solutionVP.Height = solH
	if solW > 0 && pv.solutionPath != "" {
		// Best-effort: render errors don't block the description pane.
		sol, _ := pv.renderCachedSolution(p.TitleSlug, pv.chosenLang, solW-4)
		pv.solutionRendered = sol
		pv.solutionVP.SetContent(sol)
		pv.solutionVP.GotoTop()
	} else {
		pv.solutionRendered = ""
		pv.solutionVP.SetContent("")
	}
	return nil
}

// renderCachedSolution loads the cached solution file for slug+langSlug
// and renders it through glamour as a fenced code block so chroma applies
// syntax highlighting. Returns "" (no error) when no file is cached.
func (pv *problemView) renderCachedSolution(slug, langSlug string, width int) (string, error) {
	path := pv.cache.ExistingPath(slug, langSlug)
	if path == "" {
		return "", nil
	}
	code, err := pv.cache.Read(path)
	if err != nil {
		return "", err
	}
	md := "```" + editor.ChromaLang(langSlug) + "\n" + code + "\n```\n"
	return render.MarkdownToTerminal(md, width)
}

// problemDetailLayout splits the detail screen between the description (left)
// and the solution / scaffold-prompt pane (right). The right pane is shown
// regardless of whether a cached solution exists — without one, it carries
// the "press l / e" prompt instead. Returns 0 widths for the right pane only
// when the terminal is too narrow to fit both legibly.
func problemDetailLayout(width, height int) (descW, descH, solW, solH int) {
	descH = height - problemChromeHeight
	if descH < 5 {
		descH = 20
	}
	solH = descH
	if width < detailMinTotalWidth {
		return width, descH, 0, 0
	}
	solW = clampInt(width*4/10, detailSolMinWidth, detailSolMaxWidth)
	descW = width - solW - detailGap
	if descW < detailDescMinWidth {
		return width, descH, 0, 0
	}
	return descW, descH, solW, solH
}

const (
	detailMinTotalWidth = 100
	detailDescMinWidth  = 40
	detailSolMinWidth   = 30
	detailSolMaxWidth   = 80
	detailGap           = 2

	// problemChromeHeight reserves lines for breadcrumb, blank, top divider,
	// bottom divider, and footer. Body fills whatever's left.
	problemChromeHeight = 5
)

func snippetFor(p *leetcode.ProblemDetail, langSlug string) string {
	for _, s := range p.CodeSnippets {
		if s.LangSlug == langSlug {
			return s.Code
		}
	}
	return ""
}

// pickDefaultLang chooses an initial language for a problem. A language with
// an existing cached Solution wins so the user lands back on the one they
// last worked in. Otherwise: golang → python3 → first available.
func (pv *problemView) pickDefaultLang(snippets []leetcode.CodeSnippet, slug string) string {
	if slug != "" {
		for _, s := range snippets {
			if pv.cache.ExistingPath(slug, s.LangSlug) != "" {
				return s.LangSlug
			}
		}
	}
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
					_ = pv.renderForLayout(m.currentProblem, m.width, m.height)
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
			if m.reviewMode {
				if pv.attemptPath == "" {
					path, err := m.cache.ScaffoldAttemptTmp(pv.chosenLang, snippet)
					if err != nil {
						m.err = err
						return m, nil
					}
					pv.attemptPath = path
				}
				return m, m.editor.Open(pv.attemptPath)
			}
			path, err := m.cache.Scaffold(m.currentProblem.TitleSlug, pv.chosenLang, snippet)
			if err != nil {
				m.err = err
				return m, nil
			}
			pv.solutionPath = path
			return m, m.editor.Open(path)
		case keyMatch(km, keys.Run):
			path := pv.editPath(m.reviewMode)
			if path == "" {
				m.err = fmt.Errorf("nothing to run — press 'e' to write a solution first")
				return m, nil
			}
			m.err = nil
			cmd, cancel := runCodeCmd(m.ctx, m.client, m.cache, m.currentProblem, pv.chosenLang, path)
			m.cancelInflight = cancel
			return m, tea.Batch(m.load.Start(KindRun, "running"), cmd)
		case keyMatch(km, keys.Submit):
			path := pv.editPath(m.reviewMode)
			if path == "" {
				m.err = fmt.Errorf("nothing to submit — press 'e' to write a solution first")
				return m, nil
			}
			m.err = nil
			cmd, cancel := submitCodeCmd(m.ctx, m.client, m.cache, m.currentProblem, pv.chosenLang, path)
			m.cancelInflight = cancel
			return m, tea.Batch(m.load.Start(KindSubmit, "submitting"), cmd)
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
		m.err = nil
		return tea.Batch(m.load.Start(KindNeutral, "loading problem"), loadProblemCmd(m.ctx, m.client, it.q.TitleSlug))
	}
	return nil
}

func viewProblemView(m *Model) string {
	pv := &m.problem
	if pv.pickingLang {
		return langPickerView(m)
	}

	w := m.width
	if w <= 0 {
		w = 80
	}
	descW, descH, solW, _ := problemDetailLayout(w, m.height)

	crumbs := breadcrumb(w, "leetcode-anki", m.currentList.Name, m.currentProblem.Title)
	leftLabel := fmt.Sprintf("#%s  %s   %s",
		m.currentProblem.QuestionFrontendID,
		m.currentProblem.Title,
		difficultyLabel(m.currentProblem.Difficulty),
	)
	if badge := statusBadge(pv.status, pv.hasSolution); badge != "" {
		leftLabel += "   " + badge
	}

	foot := footer(w,
		footerItem{"e", "edit"},
		footerItem{"r", "run"},
		footerItem{"s", "submit"},
		footerItem{"l", "language"},
		footerItem{"n/p", "next/prev"},
		footerItem{"esc", "back"},
		footerItem{"q", "quit"},
	)

	if solW <= 0 {
		// Single-pane fallback: too narrow to split.
		top := divider(w, leftLabel, 0, "")
		bot := divider(w, "", 0, "")
		body := pv.vp.View()
		if pv.rendered == "" {
			body = " " + loadingStyle.Render("loading…")
		}
		return strings.Join([]string{
			crumbs, "",
			top,
			body,
			bot,
			foot,
		}, "\n")
	}

	// Two-pane layout. Top divider carries left and right labels around ╮.
	rightLabel := "no solution yet"
	switch {
	case m.reviewMode:
		rightLabel = "review mode"
	case pv.solutionPath != "":
		rightLabel = filepath.Base(pv.solutionPath)
	}
	leftDiv := divider(descW, leftLabel, 0, "")
	rightDiv := divider(solW, rightLabel, 0, "")
	top := leftDiv + dividerLineStyle.Render("╮") + rightDiv

	// Description pane.
	descBody := pv.vp.View()
	if pv.rendered == "" {
		descBody = " " + loadingStyle.Render("loading…")
	}
	descBox := lipgloss.NewStyle().Width(descW).Height(descH).Render(descBody)

	// Right pane content: in Review Mode the cached Solution must not be
	// shown — Review is testing recall. Otherwise: cached solution if we
	// have one, scaffold prompt if not.
	var solBody string
	switch {
	case m.reviewMode:
		solBody = renderReviewSolutionHidden()
	case pv.solutionPath != "":
		solBody = pv.solutionVP.View()
	default:
		solBody = renderScaffoldPrompt(m.currentProblem)
	}

	// In-flight run/submit status, anchored at the bottom of the right pane.
	if status := runStatus(m); status != "" {
		solBody = strings.TrimRight(solBody, "\n") + "\n\n" + status
	}
	solBox := lipgloss.NewStyle().Width(solW).Height(descH).Padding(0, 1).Render(solBody)

	// Vertical line between the panes, matching the body height.
	vlines := make([]string, descH)
	for i := range vlines {
		vlines[i] = dividerLineStyle.Render("│")
	}
	vline := strings.Join(vlines, "\n")

	middle := lipgloss.JoinHorizontal(lipgloss.Top, descBox, vline, solBox)
	bot := divider(w, "", descW, "┴")

	return strings.Join([]string{
		crumbs, "",
		top,
		middle,
		bot,
		foot,
	}, "\n")
}

// runStatus returns the inline run/submit indicator for in-flight requests
// on the problem screen. Renders nothing when idle or when the active load
// is a full-screen-takeover (KindNeutral) — those don't reach this code
// path because the takeover replaces the whole screen.
func runStatus(m *Model) string {
	if m.load.kind == KindRun || m.load.kind == KindSubmit {
		return m.load.Inline()
	}
	return ""
}

// renderReviewSolutionHidden is the right-pane body in Review Mode. The
// cached Solution must not be visible — Review is testing recall — so the
// pane carries a placeholder plus the same edit/run/submit hints that
// apply to the (hidden) attempt buffer.
func renderReviewSolutionHidden() string {
	return strings.Join([]string{
		"",
		dimStyle.Render("solution hidden in review mode"),
		"",
		dimStyle.Render("press  ") + footerKeyStyle.Render("e") + dimStyle.Render("  to open a fresh attempt"),
		dimStyle.Render("press  ") + footerKeyStyle.Render("r") + dimStyle.Render("  to run"),
		dimStyle.Render("press  ") + footerKeyStyle.Render("s") + dimStyle.Render("  to submit"),
	}, "\n")
}

// renderScaffoldPrompt is the right-pane body for problems that don't have
// a cached solution yet. Lists the available language templates so the user
// can hit `l` to pick one and `e` to scaffold + edit.
func renderScaffoldPrompt(p *leetcode.ProblemDetail) string {
	rows := []string{
		"",
		dimStyle.Render("press  ") + footerKeyStyle.Render("l") + dimStyle.Render("  to pick a language"),
		dimStyle.Render("press  ") + footerKeyStyle.Render("e") + dimStyle.Render("  to scaffold + edit"),
		"",
		dimStyle.Render("available templates"),
	}
	const maxRows = 6
	for i, s := range p.CodeSnippets {
		if i >= maxRows {
			extra := len(p.CodeSnippets) - maxRows
			rows = append(rows, dimStyle.Render(fmt.Sprintf("  ⋮  +%d more", extra)))
			break
		}
		rows = append(rows, dimStyle.Render("  • "+s.LangSlug))
	}
	return strings.Join(rows, "\n")
}

// statusBadge returns a styled "✓ Solved" / "✎ In progress" label, or "" when
// the problem is untouched. Drives both the lists-screen glyph and the
// problem-screen header so the two stay in sync.
func statusBadge(status *string, hasSolution bool) string {
	if isAccepted(status) {
		return successStyle.Render("✓ Solved")
	}
	if isTried(status) || hasSolution {
		return inProgressStyle.Render("✎ In progress")
	}
	return ""
}

func langPickerView(m *Model) string {
	pv := &m.problem
	w := m.width
	if w <= 0 {
		w = 80
	}
	h := m.height
	if h <= 0 {
		h = 24
	}

	var rows []string
	rows = append(rows, dimStyle.Render("pick a language"))
	rows = append(rows, "")
	for i, s := range m.currentProblem.CodeSnippets {
		line := s.LangSlug
		if i == pv.langCursor {
			line = breadcrumbActiveStyle.Render("▸ ") + lipgloss.NewStyle().Bold(true).Render(s.LangSlug)
		} else {
			line = "  " + line
		}
		rows = append(rows, line)
	}
	body := strings.Join(rows, "\n")
	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7DD3FC")).
		Padding(0, 2).
		Render(body)

	help := footer(w,
		footerItem{"↑/↓", "move"},
		footerItem{"enter", "select"},
		footerItem{"esc", "cancel"},
	)
	placed := lipgloss.Place(w, h-1, lipgloss.Center, lipgloss.Center, modal)
	return placed + "\n" + help
}
