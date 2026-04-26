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

const userProgressQuestionListQuery = `
query userProgressQuestionList($filters: UserProgressQuestionListInput) {
  userProgressQuestionList(filters: $filters) {
    totalNum
    questions {
      frontendId
      title
      titleSlug
      difficulty
      lastSubmittedAt
      numSubmitted
      questionStatus
      lastResult
    }
  }
}
`

// UserProgress returns a page of every Problem the user has submitted to —
// the global candidate set for Review-Mode entry. totalNum lets callers page
// to completion; iterate by skip += len(returned) until skip >= totalNum.
//
// LastAccepted (lastResult == "AC") is the gate for inclusion in the SR
// rotation: only Problems with at least one Accepted submission count.
func (c *Client) UserProgress(ctx context.Context, skip, limit int) ([]ProgressQuestion, int, error) {
	vars := map[string]any{
		"filters": map[string]any{"skip": skip, "limit": limit},
	}
	referer := BaseURL + "/progress/"

	data, err := c.doGraphQL(ctx, "userProgressQuestionList", userProgressQuestionListQuery, vars, referer)
	if err != nil {
		return nil, 0, err
	}

	var wrap struct {
		UserProgressQuestionList struct {
			TotalNum  int `json:"totalNum"`
			Questions []struct {
				TitleSlug       string `json:"titleSlug"`
				Title           string `json:"title"`
				FrontendID      string `json:"frontendId"`
				Difficulty      string `json:"difficulty"`
				LastSubmittedAt string `json:"lastSubmittedAt"`
				NumSubmitted    int    `json:"numSubmitted"`
				LastResult      string `json:"lastResult"`
			} `json:"questions"`
		} `json:"userProgressQuestionList"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, 0, fmt.Errorf("decode userProgressQuestionList: %w", err)
	}

	out := make([]ProgressQuestion, 0, len(wrap.UserProgressQuestionList.Questions))
	for _, w := range wrap.UserProgressQuestionList.Questions {
		t, _ := time.Parse(time.RFC3339, w.LastSubmittedAt)
		out = append(out, ProgressQuestion{
			TitleSlug:       w.TitleSlug,
			Title:           w.Title,
			FrontendID:      w.FrontendID,
			Difficulty:      w.Difficulty,
			LastSubmittedAt: t,
			NumSubmitted:    w.NumSubmitted,
			LastAccepted:    w.LastResult == "AC",
		})
	}
	return out, wrap.UserProgressQuestionList.TotalNum, nil
}
