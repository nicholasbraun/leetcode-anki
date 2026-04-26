package tui

import (
	"strings"
	"testing"

	"leetcode-anki/internal/leetcode"
)

func TestRenderRunResult(t *testing.T) {
	tests := []struct {
		name        string
		in          *leetcode.RunResult
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "nil",
			in:          nil,
			wantContain: []string{"no result"},
		},
		{
			name: "accepted",
			in: &leetcode.RunResult{
				StatusMsg:          "Accepted",
				CorrectAnswer:      true,
				CodeAnswer:         []string{"[0,1]"},
				ExpectedCodeAnswer: []string{"[0,1]"},
			},
			wantContain: []string{"Accepted"},
			wantAbsent:  []string{"Wrong Answer"},
		},
		{
			name: "wrong answer despite status_msg=Accepted",
			in: &leetcode.RunResult{
				StatusMsg:          "Accepted",
				CorrectAnswer:      false,
				CodeAnswer:         []string{"[]"},
				ExpectedCodeAnswer: []string{"[0,1]"},
			},
			wantContain: []string{"Wrong Answer", "your answer:", "expected:"},
			wantAbsent:  []string{"Accepted"},
		},
		{
			name: "compile error",
			in: &leetcode.RunResult{
				CompileError:     "syntax error",
				FullCompileError: "line 3: missing ';'",
			},
			wantContain: []string{"Compile error", "missing ';'"},
		},
		{
			name: "runtime error",
			in: &leetcode.RunResult{
				RuntimeError:     "panic",
				FullRuntimeError: "index out of range",
				LastTestcase:     "[1,2]",
			},
			wantContain: []string{"Runtime error", "index out of range", "[1,2]"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderRunResult(tc.in)
			for _, s := range tc.wantContain {
				if !strings.Contains(got, s) {
					t.Errorf("missing %q in:\n%s", s, got)
				}
			}
			for _, s := range tc.wantAbsent {
				if strings.Contains(got, s) {
					t.Errorf("unexpected %q in:\n%s", s, got)
				}
			}
		})
	}
}

func TestRenderSubmitResult(t *testing.T) {
	tests := []struct {
		name        string
		in          *leetcode.SubmitResult
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "nil",
			in:          nil,
			wantContain: []string{"no result"},
		},
		{
			name: "accepted",
			in: &leetcode.SubmitResult{
				StatusMsg:      "Accepted",
				TotalCorrect:   58,
				TotalTestcases: 58,
				StatusRuntime:  "0 ms",
			},
			wantContain: []string{"Accepted", "58/58"},
			wantAbsent:  []string{"failing input:"},
		},
		{
			name: "wrong answer surfaces failing case",
			in: &leetcode.SubmitResult{
				StatusMsg:      "Wrong Answer",
				TotalCorrect:   12,
				TotalTestcases: 58,
				LastTestcase:   "[1,2,3]",
				ExpectedOutput: "[1,2]",
				CodeOutput:     "[1]",
			},
			wantContain: []string{"Wrong Answer", "12/58", "failing input:", "[1,2,3]", "[1,2]", "[1]"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderSubmitResult(tc.in)
			for _, s := range tc.wantContain {
				if !strings.Contains(got, s) {
					t.Errorf("missing %q in:\n%s", s, got)
				}
			}
			for _, s := range tc.wantAbsent {
				if strings.Contains(got, s) {
					t.Errorf("unexpected %q in:\n%s", s, got)
				}
			}
		})
	}
}
