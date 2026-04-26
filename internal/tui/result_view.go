package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

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
}

func updateResultView(m *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
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

func viewResultView(m *Model) string {
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

// runHeaderAndBody returns the colored header and the body for a run result.
// nil yields a "no result" header so the screen still draws.
func runHeaderAndBody(r *leetcode.RunResult) (string, string) {
	if r == nil {
		return errorStyle.Render("no result"), ""
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

// submitHeaderAndBody mirrors runHeaderAndBody for submit verdicts.
func submitHeaderAndBody(r *leetcode.SubmitResult) (string, string) {
	if r == nil {
		return errorStyle.Render("no result"), ""
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
