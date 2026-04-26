package leetcode

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"leetcode-anki/internal/auth"
)

// capturingDoer records the request body so tests can assert on the
// outgoing GraphQL variables. Unlike scriptedDoer/routedDoer, this one
// retains exactly one round-trip.
type capturingDoer struct {
	body []byte
	resp string
}

func (c *capturingDoer) Do(req *http.Request) (*http.Response, error) {
	c.body, _ = io.ReadAll(req.Body)
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(c.resp)),
		Header:     make(http.Header),
	}, nil
}

// The mutation replaces note text wholesale and must round-trip tagIds and
// flagType. Asserting the outgoing variables prevents a regression where
// future refactors silently drop one — which would clobber the user's
// manually-set flag/tags on every Review.
func TestUpdateSubmissionNote_SendsVariables(t *testing.T) {
	d := &capturingDoer{resp: `{"data":{"updateSubmissionNote":{"ok":true,"error":null}}}`}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	err := c.UpdateSubmissionNote(context.Background(), "1988694277", "TEST\n[anki:3]", []int{}, "WHITE")
	if err != nil {
		t.Fatalf("UpdateSubmissionNote: %v", err)
	}

	var sent struct {
		OperationName string         `json:"operationName"`
		Variables     map[string]any `json:"variables"`
	}
	if err := json.Unmarshal(d.body, &sent); err != nil {
		t.Fatalf("decode sent body: %v", err)
	}
	if sent.OperationName != "updateSubmissionNote" {
		t.Errorf("operationName = %q", sent.OperationName)
	}
	if sent.Variables["submissionId"] != "1988694277" {
		t.Errorf("submissionId = %v", sent.Variables["submissionId"])
	}
	if sent.Variables["note"] != "TEST\n[anki:3]" {
		t.Errorf("note = %v", sent.Variables["note"])
	}
	if sent.Variables["flagType"] != "WHITE" {
		t.Errorf("flagType = %v", sent.Variables["flagType"])
	}
	tagIds, ok := sent.Variables["tagIds"].([]any)
	if !ok {
		t.Errorf("tagIds missing or wrong type: %T %v", sent.Variables["tagIds"], sent.Variables["tagIds"])
	} else if len(tagIds) != 0 {
		t.Errorf("tagIds = %v, want empty", tagIds)
	}
}

// LeetCode signals failure via {ok:false, error:"..."} rather than a
// transport error. The wrapper must surface that as a Go error, otherwise
// callers think a write succeeded when LeetCode silently rejected it.
func TestUpdateSubmissionNote_ErrorFromServer(t *testing.T) {
	d := &capturingDoer{resp: `{"data":{"updateSubmissionNote":{"ok":false,"error":"submission not found"}}}`}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	err := c.UpdateSubmissionNote(context.Background(), "999", "x", []int{}, "WHITE")
	if err == nil {
		t.Fatal("expected error from ok=false response")
	}
	if !strings.Contains(err.Error(), "submission not found") {
		t.Errorf("error message must include server message; got %v", err)
	}
}
