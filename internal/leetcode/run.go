package leetcode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// interpretResponse is the *initial* body returned by POST /interpret_solution/.
// It only contains the polling handle (interpret_id); the actual verdict
// arrives later via pollCheck and is decoded into RunResult.
type interpretResponse struct {
	InterpretID       string `json:"interpret_id"`
	InterpretExpected string `json:"interpret_expected_id"`
	TestCase          string `json:"test_case"`
}

// runResultWire is the on-the-wire shape of the check-poll body for a Run.
// RunResult exposes a higher-level []RunCase derived from the parallel
// arrays here plus the dataInput we submitted; this struct lives entirely
// inside the leetcode package as the JSON boundary.
type runResultWire struct {
	State              string   `json:"state"`
	StatusCode         int      `json:"status_code"`
	StatusMsg          string   `json:"status_msg"`
	StatusRuntime      string   `json:"status_runtime"`
	StatusMemory       string   `json:"status_memory"`
	Lang               string   `json:"lang"`
	RunSuccess         bool     `json:"run_success"`
	CorrectAnswer      bool     `json:"correct_answer"`
	CodeAnswer         []string `json:"code_answer"`
	ExpectedCodeAnswer []string `json:"expected_code_answer"`
	StdOutputList      []string `json:"std_output_list"`
	CompareResult      string   `json:"compare_result"`
	CompileError       string   `json:"compile_error"`
	FullCompileError   string   `json:"full_compile_error"`
	RuntimeError       string   `json:"runtime_error"`
	FullRuntimeError   string   `json:"full_runtime_error"`
	LastTestcase       string   `json:"last_testcase"`
}

// InterpretSolution invokes LeetCode's "Run code" endpoint and polls for a verdict.
//
// dataInput should be the raw test input string — typically `problem.ExampleTestcases`
// or a custom string from the user. metaData is the problem's MetaData JSON
// (from `ProblemDetail.MetaData`); its `params` length is used to chunk
// dataInput into per-case inputs on the returned RunResult.Cases slice.
func (c *Client) InterpretSolution(ctx context.Context, slug, lang, questionID, code, dataInput, metaData string) (*RunResult, error) {
	url := interpretURL(slug)
	referer := problemRefURL(slug)

	body := map[string]any{
		"lang":        lang,
		"question_id": questionID,
		"typed_code":  code,
		"data_input":  dataInput,
	}

	var ir interpretResponse
	if err := c.doREST(ctx, "POST", url, body, &ir, referer); err != nil {
		return nil, fmt.Errorf("interpret_solution: %w", err)
	}
	if ir.InterpretID == "" {
		return nil, fmt.Errorf("interpret_solution: empty interpret_id")
	}

	raw, err := c.pollCheck(ctx, ir.InterpretID)
	if err != nil {
		return nil, err
	}

	var w runResultWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, fmt.Errorf("decode run result: %w", err)
	}
	return &RunResult{
		State:            w.State,
		StatusCode:       w.StatusCode,
		StatusMsg:        w.StatusMsg,
		StatusRuntime:    w.StatusRuntime,
		StatusMemory:     w.StatusMemory,
		Lang:             w.Lang,
		RunSuccess:       w.RunSuccess,
		CorrectAnswer:    w.CorrectAnswer,
		Cases:            buildRunCases(w, dataInput, metaData),
		CompileError:     w.CompileError,
		FullCompileError: w.FullCompileError,
		RuntimeError:     w.RuntimeError,
		FullRuntimeError: w.FullRuntimeError,
		LastTestcase:     w.LastTestcase,
	}, nil
}

// buildRunCases materializes RunResult.Cases from the parallel arrays the
// check endpoint returns plus the raw dataInput the caller submitted.
//
// The number of cases is len(wire.CodeAnswer); shorter parallel slices are
// padded with empty strings. paramCount comes from metaData (`params`
// length); when MetaData is missing or malformed the input split degrades
// gracefully (even-chunk by case count, then per-line) rather than panicking.
// Pass reads from compare_result when present and falls back to per-case
// equality otherwise.
func buildRunCases(w runResultWire, dataInput, metaData string) []RunCase {
	n := len(w.CodeAnswer)
	if n == 0 {
		return nil
	}

	inputs := splitDataInput(dataInput, metaData, n)
	cases := make([]RunCase, n)
	for i := 0; i < n; i++ {
		c := RunCase{
			Index:  i,
			Output: w.CodeAnswer[i],
		}
		if i < len(w.ExpectedCodeAnswer) {
			c.Expected = w.ExpectedCodeAnswer[i]
		}
		if i < len(w.StdOutputList) {
			c.Stdout = w.StdOutputList[i]
		}
		if i < len(inputs) {
			c.Input = inputs[i]
		}
		c.Pass = compareResultBit(w.CompareResult, i, c.Output, c.Expected)
		cases[i] = c
	}
	return cases
}

// splitDataInput slices dataInput into n per-case input strings using
// metaData's params count. Falls back to even chunks then per-line input
// when MetaData is unparseable or doesn't divide cleanly.
func splitDataInput(dataInput, metaData string, n int) []string {
	if n <= 0 || dataInput == "" {
		return nil
	}
	lines := strings.Split(dataInput, "\n")

	paramCount := metaParamCount(metaData)
	if paramCount > 0 && len(lines) == paramCount*n {
		return chunkLines(lines, paramCount)
	}

	if len(lines)%n == 0 {
		return chunkLines(lines, len(lines)/n)
	}

	out := make([]string, n)
	for i := 0; i < n; i++ {
		if i < len(lines) {
			out[i] = lines[i]
		}
	}
	return out
}

func chunkLines(lines []string, perCase int) []string {
	if perCase <= 0 {
		return nil
	}
	out := make([]string, 0, len(lines)/perCase)
	for i := 0; i+perCase <= len(lines); i += perCase {
		out = append(out, strings.Join(lines[i:i+perCase], "\n"))
	}
	return out
}

// metaParamCount returns len(params) parsed from a problem's MetaData JSON,
// or 0 when metaData is empty or unparseable.
func metaParamCount(metaData string) int {
	if metaData == "" {
		return 0
	}
	var m struct {
		Params []json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal([]byte(metaData), &m); err != nil {
		return 0
	}
	return len(m.Params)
}

// compareResultBit reads the i-th '0'/'1' bit out of LeetCode's
// compare_result string. When the string is too short or absent it falls
// back to direct output==expected comparison so missing wire data never
// silently flips a case to "pass".
func compareResultBit(compare string, i int, output, expected string) bool {
	if i < len(compare) {
		return compare[i] == '1'
	}
	return output == expected
}

// pollCheck polls /submissions/detail/{id}/check/ until the response reports
// a non-pending state, then returns the raw JSON for the caller to decode
// into the verdict shape it wants (RunResult vs SubmitResult).
//
// LeetCode publishes "PENDING" and "STARTED" while the judge is still running.
// Anything else — including unexpected values — is treated as terminal: the
// API has produced its final word and there's no upside to looping further.
// A hard iteration cap prevents an infinite loop if the API ever returns an
// empty body (state=="").
func (c *Client) pollCheck(ctx context.Context, id string) (json.RawMessage, error) {
	url := submissionCheckURL(id)
	referer := BaseURL + "/"

	delay := 700 * time.Millisecond
	const maxDelay = 2 * time.Second
	const maxPolls = 240 // ~8 minutes at 2s cap, well past LeetCode's worst case

	for i := 0; i < maxPolls; i++ {
		var raw json.RawMessage
		if err := c.doREST(ctx, "GET", url, nil, &raw, referer); err != nil {
			return nil, fmt.Errorf("poll check: %w", err)
		}

		var probe struct {
			State string `json:"state"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			return nil, fmt.Errorf("poll decode: %w", err)
		}

		if probe.State != "" && probe.State != "PENDING" && probe.State != "STARTED" {
			return raw, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}

		delay = delay * 5 / 4
		if delay > maxDelay {
			delay = maxDelay
		}
	}
	return nil, fmt.Errorf("poll check: gave up after %d iterations without a terminal state", maxPolls)
}
