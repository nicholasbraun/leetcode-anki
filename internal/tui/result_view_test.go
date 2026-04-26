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
			wantContain: []string{"Wrong Answer", "your output", "expected output"},
			wantAbsent:  []string{"Accepted"},
		},
		{
			name: "compile error",
			in: &leetcode.RunResult{
				CompileError:     "syntax error",
				FullCompileError: "line 3: missing ';'",
			},
			wantContain: []string{"Compile Error", "missing ';'"},
		},
		{
			name: "runtime error",
			in: &leetcode.RunResult{
				RuntimeError:     "panic",
				FullRuntimeError: "index out of range",
				LastTestcase:     "[1,2]",
			},
			wantContain: []string{"Runtime Error", "index out of range", "[1,2]"},
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
			wantAbsent:  []string{"last failed input"},
		},
		{
			name: "accepted with percentiles",
			in: &leetcode.SubmitResult{
				StatusMsg:         "Accepted",
				TotalCorrect:      58,
				TotalTestcases:    58,
				StatusRuntime:     "3 ms",
				StatusMemory:      "4.2 MB",
				RuntimePercentile: 89.4,
				MemoryPercentile:  71.2,
			},
			wantContain: []string{"Accepted", "beats 89.4%", "beats 71.2%"},
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
			wantContain: []string{"Wrong Answer", "12 / 58", "last failed input", "[1,2,3]", "your output", "[1]", "expected output", "[1,2]"},
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
