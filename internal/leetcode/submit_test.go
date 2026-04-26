package leetcode

import (
	"context"
	"testing"
)

// Submit must surface the submission_id returned by /submit/ so callers
// (the SR package) can attach a note to that exact submission.
func TestSubmit_PopulatesSubmissionID(t *testing.T) {
	d := &scriptedDoer{bodies: []string{
		`{"submission_id":1988694277}`,
		`{"state":"SUCCESS","status_msg":"Accepted","status_code":10,"lang":"golang","run_success":true,"total_correct":50,"total_testcases":50}`,
	}}
	c := newTestClient(d)

	res, err := c.Submit(context.Background(), "two-sum", "golang", "1", "func twoSum() {}")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if res.SubmissionID != "1988694277" {
		t.Errorf("SubmissionID = %q, want %q", res.SubmissionID, "1988694277")
	}
	if res.StatusMsg != "Accepted" {
		t.Errorf("StatusMsg = %q, want %q", res.StatusMsg, "Accepted")
	}
}
