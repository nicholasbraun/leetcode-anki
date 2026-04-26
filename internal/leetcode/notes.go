package leetcode

import (
	"context"
	"encoding/json"
	"fmt"
)

const updateSubmissionNoteMutation = `
mutation updateSubmissionNote($submissionId: ID!, $note: String, $tagIds: [Int], $flagType: SubmissionFlagTypeEnum) {
  updateSubmissionNote(
    submissionId: $submissionId
    note: $note
    tagIds: $tagIds
    flagType: $flagType
  ) {
    ok
    error
  }
}
`

// UpdateSubmissionNote replaces the note attached to a submission. Callers
// must round-trip tagIDs and flagType from the latest SubmissionList read
// — this mutation overwrites all four fields, so passing the wrong tagIDs
// or flagType silently clobbers the user's manual customizations.
//
// LeetCode signals failure via {ok:false, error:"..."} rather than a
// transport error; that path is converted to a Go error here.
func (c *Client) UpdateSubmissionNote(ctx context.Context, submissionID, note string, tagIDs []int, flagType string) error {
	if tagIDs == nil {
		tagIDs = []int{}
	}
	vars := map[string]any{
		"submissionId": submissionID,
		"note":         note,
		"tagIds":       tagIDs,
		"flagType":     flagType,
	}
	referer := BaseURL + "/submissions/"

	data, err := c.doGraphQL(ctx, "updateSubmissionNote", updateSubmissionNoteMutation, vars, referer)
	if err != nil {
		return err
	}

	var wrap struct {
		UpdateSubmissionNote struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		} `json:"updateSubmissionNote"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return fmt.Errorf("decode updateSubmissionNote: %w", err)
	}
	if !wrap.UpdateSubmissionNote.OK {
		return fmt.Errorf("updateSubmissionNote: %s", wrap.UpdateSubmissionNote.Error)
	}
	return nil
}
