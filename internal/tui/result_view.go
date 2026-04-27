package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"leetcode-anki/internal/leetcode"
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
		// Any other key (q, ctrl+c handled upstream, e, r, s, n, p) is
		// inert while the modal is open, mirroring the lang-picker pattern.
		return m, nil
	}

	if isKey {
		switch {
		case keyMatch(km, keys.Quit):
			return m, tea.Quit
		case keyMatch(km, keys.Back), keyMatch(km, keys.Enter):
			m.screen = screenProblem
			return m, nil
		}
	}
	return m, nil
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

// advanceToNextDue drops the just-rated slug from the Review-Mode due set,
// rebuilds the problems list, and returns either a loadProblemCmd for the
// next due slug or nil with the screen set to screenProblems when nothing
// is left to review.
func advanceToNextDue(m *Model, ratedSlug string) tea.Cmd {
	if m.dueSlugs != nil {
		delete(m.dueSlugs, ratedSlug)
	}
	visible := visibleProblems(m.problemsAll, m.reviewMode, m.dueSlugs)
	if m.problemsReady {
		lw, lh, _, _ := problemsLayout(m.width, m.height)
		m.problems = newProblemsList(lw, lh, visible, m.currentList.Name, m.solutionSlugs)
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

	crumbs := breadcrumb(w, "leetcode-anki", m.currentList.Name, problemTitle(m), "result")

	var header, body string
	switch m.result.kind {
	case resultRun:
		header, body = runHeaderAndBody(m.result.run)
	case resultSubmit:
		header, body = submitHeaderAndBody(m.result.submit)
	}

	top := divider(w, header, 0, "")
	bot := divider(w, "", 0, "")
	foot := footer(w,
		footerItem{"enter/esc", "back to problem"},
		footerItem{"q", "quit"},
	)

	return strings.Join([]string{
		crumbs, "",
		top,
		"",
		body,
		"",
		bot,
		foot,
	}, "\n")
}

func problemTitle(m *Model) string {
	if m.currentProblem == nil {
		return ""
	}
	return m.currentProblem.Title
}

// runHeaderAndBody returns the colored header and the body for a Run Verdict.
// nil yields a "no verdict" header so the screen still draws.
func runHeaderAndBody(r *leetcode.RunResult) (string, string) {
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
		return errorStyle.Render("✗ Wrong Answer"), runWrongBody(r)
	default:
		return successStyle.Render("✓ Accepted"), runAcceptedBody(r)
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

func runAcceptedBody(r *leetcode.RunResult) string {
	rows := []string{}
	if r.StatusRuntime != "" {
		rows = append(rows, kv("runtime", r.StatusRuntime))
	}
	if r.StatusMemory != "" {
		rows = append(rows, kv("memory", r.StatusMemory))
	}
	if n := len(r.CodeAnswer); n > 0 {
		rows = append(rows, kv("test cases", fmt.Sprintf("%d ran", n)))
	}
	if r.Lang != "" {
		rows = append(rows, kv("language", r.Lang))
	}
	if r.StdOutput != "" {
		rows = append(rows, "", kv("stdout", ""), indent(r.StdOutput, 4))
	}
	return strings.Join(rows, "\n")
}

func runWrongBody(r *leetcode.RunResult) string {
	rows := []string{}
	if n := len(r.CodeAnswer); n > 0 {
		rows = append(rows, kv("test cases ran", fmt.Sprintf("%d", n)))
	}
	rows = append(rows, "")
	rows = append(rows, kv("your output", ""))
	rows = append(rows, indent(strings.Join(r.CodeAnswer, "\n"), 4))
	rows = append(rows, "")
	rows = append(rows, kv("expected output", ""))
	rows = append(rows, indent(strings.Join(r.ExpectedCodeAnswer, "\n"), 4))
	if r.StdOutput != "" {
		rows = append(rows, "", kv("stdout", ""), indent(r.StdOutput, 4))
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
// drives the same building blocks through divider chrome.
func renderRunResult(r *leetcode.RunResult) string {
	header, body := runHeaderAndBody(r)
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

	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7DD3FC")).
		Padding(1, 3).
		Render(strings.Join(rows, "\n"))

	help := footer(w,
		footerItem{"1-4", "rate"},
		footerItem{"↑/↓ enter", "pick (default Good)"},
		footerItem{"esc", "cancel"},
	)
	placed := lipgloss.Place(w, h-1, lipgloss.Center, lipgloss.Center, modal)
	return placed + "\n" + help
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
