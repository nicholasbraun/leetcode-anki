package leetcode

import (
	"context"
	"encoding/json"
	"fmt"
)

type submitResponse struct {
	SubmissionID json.Number `json:"submission_id"`
}

// Submit posts the solution to LeetCode and polls for the final verdict.
func (c *Client) Submit(ctx context.Context, slug, lang, questionID, code string) (*SubmitResult, error) {
	url := fmt.Sprintf("%s/problems/%s/submit/", BaseURL, slug)
	referer := fmt.Sprintf("%s/problems/%s/", BaseURL, slug)

	body := map[string]any{
		"lang":        lang,
		"question_id": questionID,
		"typed_code":  code,
	}

	var sr submitResponse
	if err := c.doREST(ctx, "POST", url, body, &sr, referer); err != nil {
		return nil, fmt.Errorf("submit: %w", err)
	}
	if sr.SubmissionID.String() == "" {
		return nil, fmt.Errorf("submit: empty submission_id")
	}

	raw, err := c.pollCheck(ctx, sr.SubmissionID.String())
	if err != nil {
		return nil, err
	}

	var out SubmitResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode submit result: %w", err)
	}
	return &out, nil
}
