package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/sr"
	"leetcode-anki/internal/tui/modal"
)

type resultKind int

const (
	resultRun resultKind = iota
	resultSubmit
)

type resultView struct {
	kind   resultKind
	run    *leetcode.RunResult
	submit *leetcode.SubmitResult

	// bodyVP wraps only the verdict body so long Run/Submit outputs (many
	// cases, long error traces) scroll without taking the breadcrumb,
	// verdict header, dividers, or footer with them. Sized in
	// viewResultView from the screen dimensions minus chrome.
	bodyVP viewport.Model

	// exampleCount is the Problem's Example Test Case count for the Run
	// behind this view. Cases at index >= exampleCount are Custom Test
	// Cases added by the user and render with a star glyph.
	exampleCount int

	// toast is a transient confirmation message rendered above the footer
	// (e.g. "added" after promoting a failing input). Cleared on the next
	// key event so it never lingers past the action that produced it.
	toast string

	// grade is the open rating modal, nil when closed. Pointer so non-Accepted
	// results render the standard verdict view by simply leaving this nil.
	grade *gradeModalState
}

// gradeModalState backs the per-rating modal that opens after an Accepted
// Submit. cursor 0..3 maps to ratings 1..4 (Again/Hard/Good/Easy); previews
// holds the next-due timestamp the scheduler would assign for each candidate
// rating, computed once when the modal opens.
type gradeModalState struct {
	cursor   int
	previews [4]time.Time
}

func updateResultView(m *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	km, isKey := msg.(tea.KeyMsg)

	if m.result.grade != nil {
		if !isKey {
			return m, nil
		}
		switch km.String() {
		case "1", "2", "3", "4":
			return m, commitGrade(m, int(km.String()[0]-'0'))
		case "enter":
			return m, commitGrade(m, m.result.grade.cursor+1)
		case "up", "k":
			if m.result.grade.cursor > 0 {
				m.result.grade.cursor--
			}
			return m, nil
		case "down", "j":
			if m.result.grade.cursor < 3 {
				m.result.grade.cursor++
			}
			return m, nil
		case "esc":
			m.result.grade = nil
			m.screen = screenProblem
			return m, nil
		}
		// Action keys (e/r/s/n/p) are inert while the modal is open;
		// quit keys are handled by the global dispatch in app.go.
		return m, nil
	}

	if isKey {
		// A previous toast clears on the next keystroke so the
		// confirmation never lingers past the action that produced it.
		m.result.toast = ""

		if km.String() == "a" && canPromoteSubmit(m.result) {
			input := m.result.submit.LastTestcase
			if err := m.cases.Add(m.currentProblem.TitleSlug, input); err != nil {
				m.err = err
				return m, nil
			}
			m.result.toast = "added"
			return m, nil
		}

		if keyMatch(km, keys.Back) || keyMatch(km, keys.Enter) {
			m.screen = screenProblem
			return m, nil
		}
	}

	// Unhandled messages — including viewport scroll keys (↑/↓, j/k,
	// pgup/pgdn, home/end) — fall through so long bodies stay reachable.
	var cmd tea.Cmd
	m.result.bodyVP, cmd = m.result.bodyVP.Update(msg)
	return m, cmd
}

// canPromoteSubmit reports whether the current resultView is a Submit
// Verdict carrying a non-empty LastTestcase that can be added as a Custom
// Test Case. Mirrors the gate the footer hint uses so 'a' never fires
// when the affordance isn't advertised.
func canPromoteSubmit(r resultView) bool {
	return r.kind == resultSubmit && r.submit != nil && r.submit.LastTestcase != ""
}

// commitGrade records the user's rating and dispatches the next-screen
// transition: in Review Mode it loads the next due Problem from the
// current list; in Explore Mode it returns to the problems screen.
func commitGrade(m *Model, rating int) tea.Cmd {
	var slug string
	if m.currentProblem != nil && m.result.submit != nil {
		slug = m.currentProblem.TitleSlug
		_ = m.reviews.Record(m.ctx, slug, m.result.submit.SubmissionID, rating, time.Now())
	}
	m.result.grade = nil

	if m.reviewMode && slug != "" {
		return advanceToNextDue(m, slug)
	}
	m.screen = screenProblems
	return nil
}

// advanceToNextDue drops the just-rated slug from the Review-Mode session
// queue, rebuilds the problems list, and returns either a loadProblemCmd
// for the next queued slug or nil with the screen set to screenProblems
// when nothing is left to review. *Total counts on the session are not
// touched — they reflect what was queued at session start, so a "1 of 3
// due" footer keeps a stable denominator as the user works through it.
func advanceToNextDue(m *Model, ratedSlug string) tea.Cmd {
	if m.session != nil {
		filtered := m.session.Items[:0]
		for _, it := range m.session.Items {
			if it.TitleSlug == ratedSlug {
				switch it.Kind {
				case sr.KindDue:
					m.session.DueCount--
				case sr.KindNew:
					m.session.NewCount--
				}
				continue
			}
			filtered = append(filtered, it)
		}
		m.session.Items = filtered
	}
	visible := visibleProblems(m.problemsAll, m.reviewMode, m.session)
	if m.problemsReady {
		lw, lh, _, _ := problemsLayout(m.width, m.height)
		m.problems = newProblemsList(lw, lh, visible, m.currentList.Name, m.solutionSlugs, sessionBadges(m.session, time.Now()))
		m.problemIndex = 0
	}
	if len(visible) == 0 {
		m.screen = screenProblems
		return nil
	}
	return tea.Batch(m.load.Start(KindNeutral, "loading problem"), loadProblemCmd(m.ctx, m.client, visible[0].TitleSlug))
}

func viewResultView(m *Model) string {
	if m.result.grade != nil {
		return gradeModalView(m)
	}

	w := m.width
	if w <= 0 {
		w = 80
	}
	h := m.height
	if h <= 0 {
		h = 24
	}

	crumbs := breadcrumb(w, "leetcode-anki", m.currentList.Name, problemTitle(m), "result")

	var header, body string
	switch m.result.kind {
	case resultRun:
		header, body = runHeaderAndBody(m.result.run, m.result.exampleCount, w)
	case resultSubmit:
		header, body = submitHeaderAndBody(m.result.submit)
	}

	top := divider(w, header, 0, "")
	bot := divider(w, "", 0, "")
	footItems := []footerItem{
		{"enter/esc", "back to problem"},
		{"q", "quit"},
	}
	if canPromoteSubmit(m.result) {
		footItems = append(footItems, footerItem{"a", "add to custom tests"})
	}
	if m.result.kind == resultRun && m.result.run != nil && hasCustomCases(m.result.run.Cases, m.result.exampleCount) {
		footItems = append(footItems, footerItem{"★", "custom"})
	}
	foot := footer(w, footItems...)

	bodyH := h - resultChromeHeight
	if m.result.toast != "" {
		bodyH--
	}
	if bodyH < 1 {
		bodyH = 1
	}
	m.result.bodyVP.Width = w
	m.result.bodyVP.Height = bodyH
	m.result.bodyVP.SetContent(body)

	parts := []string{crumbs, "", top, "", m.result.bodyVP.View(), "", bot}
	if m.result.toast != "" {
		parts = append(parts, " "+successStyle.Render(m.result.toast))
	}
	parts = append(parts, foot)
	return strings.Join(parts, "\n")
}

// resultChromeHeight reserves lines around the body viewport: breadcrumb,
// blank, top divider, blank, blank, bottom divider, footer.
const resultChromeHeight = 7

func problemTitle(m *Model) string {
	if m.currentProblem == nil {
		return ""
	}
	return m.currentProblem.Title
}

// runHeaderAndBody returns the colored header and the body for a Run Verdict.
// nil yields a "no verdict" header so the screen still draws.
// width sizes the per-case grid in runBody. exampleCount marks the boundary
// between Examples and Customs in the rendered case grid.
func runHeaderAndBody(r *leetcode.RunResult, exampleCount, width int) (string, string) {
	if r == nil {
		return errorStyle.Render("no verdict"), ""
	}
	switch {
	case r.CompileError != "":
		return errorStyle.Render("⚠ Compile Error"),
			errBody("", r.FullCompileError, r.CompileError)
	case r.RuntimeError != "":
		return errorStyle.Render("⚠ Runtime Error"),
			errBody(r.LastTestcase, r.FullRuntimeError, r.RuntimeError)
	case !r.CorrectAnswer:
		return errorStyle.Render("✗ Wrong Answer"), runBody(r, exampleCount, width)
	default:
		return successStyle.Render("✓ Accepted"), runBody(r, exampleCount, width)
	}
}

// submitHeaderAndBody mirrors runHeaderAndBody for Submit Verdicts.
func submitHeaderAndBody(r *leetcode.SubmitResult) (string, string) {
	if r == nil {
		return errorStyle.Render("no verdict"), ""
	}
	switch {
	case r.CompileError != "":
		return errorStyle.Render("⚠ Compile Error"),
			errBody("", r.FullCompileError, r.CompileError)
	case r.RuntimeError != "":
		return errorStyle.Render("⚠ Runtime Error"),
			errBody(r.LastTestcase, r.FullRuntimeError, r.RuntimeError)
	case r.StatusMsg != "Accepted":
		// LeetCode tags everything that isn't accepted with a status msg
		// like "Wrong Answer" / "Time Limit Exceeded" — surface that
		// exact phrase so the user knows what happened.
		return errorStyle.Render("✗ " + r.StatusMsg), submitWrongBody(r)
	default:
		return successStyle.Render("✓ Accepted"), submitAcceptedBody(r)
	}
}

// runBody renders the body for both Accepted and Wrong-Answer Run
// Verdicts: a compact summary header followed by per-case blocks
// (input, your output, expected, stdout) laid out as a grid. Compile /
// runtime branches short-circuit upstream — they never carry Cases.
func runBody(r *leetcode.RunResult, exampleCount, width int) string {
	rows := []string{}
	if n := len(r.Cases); n > 0 {
		rows = append(rows, kv("test cases", fmt.Sprintf("%d / %d passed", countPassed(r.Cases), n)))
	}
	if r.StatusRuntime != "" {
		rows = append(rows, kv("runtime", r.StatusRuntime))
	}
	if r.StatusMemory != "" {
		rows = append(rows, kv("memory", r.StatusMemory))
	}
	if r.Lang != "" {
		rows = append(rows, kv("language", r.Lang))
	}
	if grid := renderCaseGrid(r.Cases, exampleCount, width); grid != "" {
		rows = append(rows, "", grid)
	}
	return strings.Join(rows, "\n")
}

// renderCaseGrid lays per-case blocks side-by-side in 1–3 columns based
// on the available width: cols = max(1, width/38), so 38 chars is the
// minimum per column. Cases beyond the first row wrap to a new row.
//
// Cases at index >= exampleCount are Customs; they render with a star
// glyph so the user can tell their inputs from LeetCode-supplied Examples
// at a glance.
func renderCaseGrid(cases []leetcode.RunCase, exampleCount int, width int) string {
	if len(cases) == 0 {
		return ""
	}
	cols := width / 38
	if cols < 1 {
		cols = 1
	}
	colWidth := width / cols

	var rowStrs []string
	for start := 0; start < len(cases); start += cols {
		end := start + cols
		if end > len(cases) {
			end = len(cases)
		}
		blocks := make([]string, 0, end-start)
		for _, tc := range cases[start:end] {
			block := lipgloss.NewStyle().Width(colWidth).Render(runCaseBlock(tc, tc.Index >= exampleCount))
			blocks = append(blocks, block)
		}
		rowStrs = append(rowStrs, lipgloss.JoinHorizontal(lipgloss.Top, blocks...))
	}
	return strings.Join(rowStrs, "\n\n")
}

// hasCustomCases reports whether any RunCase falls past the Example
// boundary. Used to decide whether to surface the "★ custom" footer
// legend on the Run Result screen.
func hasCustomCases(cases []leetcode.RunCase, exampleCount int) bool {
	for _, c := range cases {
		if c.Index >= exampleCount {
			return true
		}
	}
	return false
}

func countPassed(cs []leetcode.RunCase) int {
	n := 0
	for _, c := range cs {
		if c.Pass {
			n++
		}
	}
	return n
}

// runCaseBlock formats one RunCase as a small labeled block. isCustom
// inserts a star glyph between the index and verdict so user-added
// Custom Test Cases stand apart from LeetCode-supplied Examples. Empty
// fields collapse — pass cases without stdout still print input /
// your output / expected, so the layout is stable across cases.
func runCaseBlock(c leetcode.RunCase, isCustom bool) string {
	header := fmt.Sprintf("  case %d", c.Index+1)
	if isCustom {
		header += " " + breadcrumbActiveStyle.Render("★")
	}
	if c.Pass {
		header += "    " + successStyle.Render("✓ pass")
	} else {
		header += "    " + errorStyle.Render("✗ fail")
	}
	rows := []string{header}
	if c.Input != "" {
		rows = append(rows, kv("input", ""), indent(c.Input, 4))
	}
	rows = append(rows, kv("your output", ""), indent(c.Output, 4))
	if c.Expected != "" {
		rows = append(rows, kv("expected", ""), indent(c.Expected, 4))
	}
	if c.Stdout != "" {
		rows = append(rows, kv("stdout", ""), indent(c.Stdout, 4))
	}
	return strings.Join(rows, "\n")
}

func submitAcceptedBody(r *leetcode.SubmitResult) string {
	rows := []string{}
	if r.StatusRuntime != "" {
		val := r.StatusRuntime
		if r.RuntimePercentile > 0 {
			val += "    " + dimStyle.Render(fmt.Sprintf("beats %.1f%%", r.RuntimePercentile))
		}
		rows = append(rows, kv("runtime", val))
	}
	if r.StatusMemory != "" {
		val := r.StatusMemory
		if r.MemoryPercentile > 0 {
			val += "    " + dimStyle.Render(fmt.Sprintf("beats %.1f%%", r.MemoryPercentile))
		}
		rows = append(rows, kv("memory", val))
	}
	if r.TotalTestcases > 0 {
		rows = append(rows, kv("test cases", fmt.Sprintf("%d/%d", r.TotalCorrect, r.TotalTestcases)))
	}
	if r.Lang != "" {
		rows = append(rows, kv("language", r.Lang))
	}
	return strings.Join(rows, "\n")
}

func submitWrongBody(r *leetcode.SubmitResult) string {
	rows := []string{}
	if r.TotalTestcases > 0 {
		rows = append(rows, kv("test cases passed", fmt.Sprintf("%d / %d", r.TotalCorrect, r.TotalTestcases)))
	}
	if r.LastTestcase != "" {
		rows = append(rows, kv("last failed input", r.LastTestcase))
	}
	if r.CodeOutput != "" {
		rows = append(rows, kv("your output", r.CodeOutput))
	}
	if r.ExpectedOutput != "" {
		rows = append(rows, kv("expected output", r.ExpectedOutput))
	}
	return strings.Join(rows, "\n")
}

// errBody renders the runtime/compile error layout: optional "last input"
// block (omit entirely when empty) followed by the full error trace.
// fullErr falls back to shortErr when the server didn't supply a full one.
func errBody(lastInput, fullErr, shortErr string) string {
	rows := []string{}
	if lastInput != "" {
		rows = append(rows, kv("last input", ""))
		rows = append(rows, indent(lastInput, 4))
		rows = append(rows, "")
	}
	rows = append(rows, kv("error", ""))
	msg := fullErr
	if msg == "" {
		msg = shortErr
	}
	rows = append(rows, indent(msg, 4))
	return strings.Join(rows, "\n")
}

// kv renders a "  key                value" row with an 18-char dim key
// column. Pass "" for value when the row is a section header followed by
// an indented block on subsequent lines.
func kv(key, val string) string {
	return "  " + dimStyle.Render(fmt.Sprintf("%-18s", key)) + val
}

// indent prefixes every line of s with n spaces.
func indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = pad + ln
	}
	return strings.Join(lines, "\n")
}

// renderRunResult composes the header + body for tests; the screen view
// drives the same building blocks through divider chrome. Width 80 is a
// conservative default that keeps cases in a single column so existing
// expectations about content order are preserved. exampleCount=0 here
// because the legacy fixtures predate Custom Test Cases.
func renderRunResult(r *leetcode.RunResult) string {
	header, body := runHeaderAndBody(r, 0, 80)
	if body == "" {
		return header
	}
	return header + "\n\n" + body
}

func renderSubmitResult(r *leetcode.SubmitResult) string {
	header, body := submitHeaderAndBody(r)
	if body == "" {
		return header
	}
	return header + "\n\n" + body
}

// gradeModalView renders the post-Accepted rating prompt as a centered,
// bordered panel. Mirrors the lang-picker compositor in problem_view.go:
// the underlying result screen is replaced rather than dimmed behind.
func gradeModalView(m *Model) string {
	w := m.width
	if w <= 0 {
		w = 80
	}
	h := m.height
	if h <= 0 {
		h = 24
	}

	g := m.result.grade
	now := time.Now()

	options := []struct {
		digit, label string
		style        lipgloss.Style
	}{
		{"1", "Again", hardStyle},
		{"2", "Hard", mediumStyle},
		{"3", "Good", successStyle},
		{"4", "Easy", easyStyle},
	}

	rows := []string{
		successStyle.Render("ACCEPTED"),
	}
	if stats := submitStatsLine(m.result.submit); stats != "" {
		rows = append(rows, dimStyle.Render(stats))
	}
	rows = append(rows, "", dimStyle.Render("how confidently did you solve it?"), "")
	for i, o := range options {
		cursor := "  "
		if i == g.cursor {
			cursor = breadcrumbActiveStyle.Render("▸ ")
		}
		due := dimStyle.Render(humanizeDue(g.previews[i], now))
		rows = append(rows, cursor+footerKeyStyle.Render(o.digit)+"  "+o.style.Render(o.label)+"   "+due)
	}

	help := footer(w,
		footerItem{"1-4", "rate"},
		footerItem{"↑/↓ enter", "pick (default Good)"},
		footerItem{"esc", "cancel"},
	)
	return modal.Render(modal.Options{
		Body:   strings.Join(rows, "\n"),
		Width:  w,
		Height: h,
		PadV:   1,
		PadH:   3,
		Footer: help,
	})
}

// submitStatsLine returns the compact runtime/memory/beats line shown
// inside the rating modal. Returns "" when the submit result carries
// no measurable stats (e.g. server didn't populate them).
func submitStatsLine(s *leetcode.SubmitResult) string {
	if s == nil {
		return ""
	}
	parts := []string{}
	if s.StatusRuntime != "" {
		seg := s.StatusRuntime
		if s.RuntimePercentile > 0 {
			seg += fmt.Sprintf(" (beats %.0f%%)", s.RuntimePercentile)
		}
		parts = append(parts, seg)
	}
	if s.StatusMemory != "" {
		seg := s.StatusMemory
		if s.MemoryPercentile > 0 {
			seg += fmt.Sprintf(" (beats %.0f%%)", s.MemoryPercentile)
		}
		parts = append(parts, seg)
	}
	return strings.Join(parts, " · ")
}

// humanizeDue formats a next-due timestamp as a short relative phrase.
// Zero timestamps render as "—" so a missing preview doesn't crash or
// surface a 1970 epoch date in the modal.
func humanizeDue(due, now time.Time) string {
	if due.IsZero() {
		return "—"
	}
	days := int(due.Sub(now).Hours()/24 + 0.5)
	switch {
	case days <= 0:
		return "due today"
	case days == 1:
		return "due tomorrow"
	case days < 14:
		return fmt.Sprintf("due in %d days", days)
	case days < 60:
		return fmt.Sprintf("due in %d weeks", (days+3)/7)
	default:
		return fmt.Sprintf("due in %d months", (days+15)/30)
	}
}
