package tui

import (
	"fmt"
	"strings"

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
	r := m.result
	var b strings.Builder
	b.WriteString(headerStyle.Render("Result") + "\n\n")

	switch r.kind {
	case resultRun:
		b.WriteString(renderRunResult(r.run))
	case resultSubmit:
		b.WriteString(renderSubmitResult(r.submit))
	}

	b.WriteString("\n" + helpStyle.Render("enter/esc back · q quit"))
	return b.String()
}

func renderRunResult(r *leetcode.RunResult) string {
	if r == nil {
		return errorStyle.Render("no result")
	}
	if r.CompileError != "" {
		return errorStyle.Render("Compile error") + "\n\n" + r.FullCompileError
	}
	if r.RuntimeError != "" {
		return errorStyle.Render("Runtime error") + "\n\n" + r.FullRuntimeError +
			"\n\nLast input:\n" + r.LastTestcase
	}
	verdict := r.StatusMsg
	style := errorStyle
	if r.CorrectAnswer {
		style = successStyle
	}
	out := style.Render(verdict)
	out += "\n" + dimStyle.Render(fmt.Sprintf("runtime: %s", r.StatusRuntime))
	if r.StatusMemory != "" {
		out += dimStyle.Render(fmt.Sprintf("  memory: %s", r.StatusMemory))
	}
	out += "\n\n" + lipgloss.NewStyle().Bold(true).Render("your answer:") + "\n" +
		strings.Join(r.CodeAnswer, "\n")
	out += "\n\n" + lipgloss.NewStyle().Bold(true).Render("expected:") + "\n" +
		strings.Join(r.ExpectedCodeAnswer, "\n")
	if r.StdOutput != "" {
		out += "\n\n" + dimStyle.Render("stdout:") + "\n" + r.StdOutput
	}
	return out
}

func renderSubmitResult(r *leetcode.SubmitResult) string {
	if r == nil {
		return errorStyle.Render("no result")
	}
	if r.CompileError != "" {
		return errorStyle.Render("Compile error") + "\n\n" + r.FullCompileError
	}
	if r.RuntimeError != "" {
		return errorStyle.Render("Runtime error") + "\n\n" + r.FullRuntimeError +
			"\n\nLast input:\n" + r.LastTestcase
	}
	verdict := r.StatusMsg
	style := errorStyle
	if verdict == "Accepted" {
		style = successStyle
	}
	out := style.Render(verdict)
	if r.TotalTestcases > 0 {
		out += "  " + dimStyle.Render(fmt.Sprintf("(%d/%d cases)", r.TotalCorrect, r.TotalTestcases))
	}
	if r.StatusRuntime != "" {
		out += "\n" + dimStyle.Render(fmt.Sprintf("runtime: %s", r.StatusRuntime))
		if r.RuntimePercentile > 0 {
			out += dimStyle.Render(fmt.Sprintf(" (beats %.1f%%)", r.RuntimePercentile))
		}
	}
	if r.StatusMemory != "" {
		out += "\n" + dimStyle.Render(fmt.Sprintf("memory:  %s", r.StatusMemory))
		if r.MemoryPercentile > 0 {
			out += dimStyle.Render(fmt.Sprintf(" (beats %.1f%%)", r.MemoryPercentile))
		}
	}
	if verdict != "Accepted" && r.LastTestcase != "" {
		out += "\n\n" + lipgloss.NewStyle().Bold(true).Render("failing input:") + "\n" + r.LastTestcase
		if r.ExpectedOutput != "" {
			out += "\n\n" + lipgloss.NewStyle().Bold(true).Render("expected:") + "\n" + r.ExpectedOutput
		}
		if r.CodeOutput != "" {
			out += "\n\n" + lipgloss.NewStyle().Bold(true).Render("got:") + "\n" + r.CodeOutput
		}
	}
	return out
}
