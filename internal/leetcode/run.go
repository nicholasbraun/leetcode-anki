package leetcode

import (
	"context"
	"encoding/json"
	"fmt"
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

// InterpretSolution invokes LeetCode's "Run code" endpoint and polls for a verdict.
//
// dataInput should be the raw test input string — typically `problem.ExampleTestcases`
// or a custom string from the user.
func (c *Client) InterpretSolution(ctx context.Context, slug, lang, questionID, code, dataInput string) (*RunResult, error) {
	url := fmt.Sprintf("%s/problems/%s/interpret_solution/", BaseURL, slug)
	referer := fmt.Sprintf("%s/problems/%s/", BaseURL, slug)

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

	var out RunResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode run result: %w", err)
	}
	return &out, nil
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
	url := fmt.Sprintf("%s/submissions/detail/%s/check/", BaseURL, id)
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
