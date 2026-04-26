package leetcode

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

const submissionListQuery = `
query submissionList($offset: Int!, $limit: Int!, $lastKey: String, $questionSlug: String!, $lang: Int, $status: Int) {
  questionSubmissionList(
    offset: $offset
    limit: $limit
    lastKey: $lastKey
    questionSlug: $questionSlug
    lang: $lang
    status: $status
  ) {
    lastKey
    hasNext
    submissions {
      id
      statusDisplay
      lang
      timestamp
      notes
      flagType
    }
  }
}
`

// SubmissionList fetches a page of submissions for a Problem. Pass nextKey ""
// for the first page; iterate by passing the returned cursor until it's "".
//
// Notes are inline in the response — no separate per-submission read is
// needed. statusDisplay maps to Submission.Accepted.
func (c *Client) SubmissionList(ctx context.Context, slug, nextKey string, limit int) ([]Submission, string, error) {
	var lastKeyVar any
	if nextKey != "" {
		lastKeyVar = nextKey
	}
	vars := map[string]any{
		"questionSlug": slug,
		"offset":       0,
		"limit":        limit,
		"lastKey":      lastKeyVar,
	}
	referer := fmt.Sprintf("%s/problems/%s/submissions/", BaseURL, slug)

	data, err := c.doGraphQL(ctx, "submissionList", submissionListQuery, vars, referer)
	if err != nil {
		return nil, "", err
	}

	var wrap struct {
		QuestionSubmissionList struct {
			LastKey     *string `json:"lastKey"`
			HasNext     bool    `json:"hasNext"`
			Submissions []struct {
				ID            string `json:"id"`
				StatusDisplay string `json:"statusDisplay"`
				Lang          string `json:"lang"`
				Timestamp     string `json:"timestamp"`
				Notes         string `json:"notes"`
				FlagType      string `json:"flagType"`
			} `json:"submissions"`
		} `json:"questionSubmissionList"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, "", fmt.Errorf("decode submissionList: %w", err)
	}

	out := make([]Submission, 0, len(wrap.QuestionSubmissionList.Submissions))
	for _, w := range wrap.QuestionSubmissionList.Submissions {
		ts, _ := strconv.ParseInt(w.Timestamp, 10, 64)
		out = append(out, Submission{
			ID:         w.ID,
			OccurredAt: time.Unix(ts, 0),
			Accepted:   w.StatusDisplay == "Accepted",
			Lang:       w.Lang,
			Notes:      w.Notes,
			FlagType:   w.FlagType,
		})
	}

	var nk string
	if wrap.QuestionSubmissionList.LastKey != nil && wrap.QuestionSubmissionList.HasNext {
		nk = *wrap.QuestionSubmissionList.LastKey
	}
	return out, nk, nil
}
