package leetcode

import (
	"context"
	"testing"
	"time"

	"leetcode-anki/internal/auth"
)

func TestSubmissionList_DecodesCapturedResponse(t *testing.T) {
	d := &routedDoer{byOp: map[string]string{
		"submissionList": `{"data":{"questionSubmissionList":{"lastKey":null,"hasNext":false,"submissions":[
			{"id":"1988694277","status":10,"statusDisplay":"Accepted","lang":"golang","timestamp":"1777214044","notes":"TEST","flagType":"WHITE","hasNotes":null},
			{"id":"1988662844","status":10,"statusDisplay":"Accepted","lang":"golang","timestamp":"1777211238","notes":"","flagType":"WHITE","hasNotes":false},
			{"id":"1582165935","status":10,"statusDisplay":"Wrong Answer","lang":"python3","timestamp":"1742638386","notes":"","flagType":"WHITE","hasNotes":false}
		]}}}`,
	}}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	subs, nextKey, err := c.SubmissionList(context.Background(), "two-sum", "", 20)
	if err != nil {
		t.Fatalf("SubmissionList: %v", err)
	}
	if len(subs) != 3 {
		t.Fatalf("expected 3 submissions, got %d", len(subs))
	}

	first := subs[0]
	if first.ID != "1988694277" {
		t.Errorf("ID = %q, want 1988694277", first.ID)
	}
	if first.Notes != "TEST" {
		t.Errorf("Notes = %q, want TEST", first.Notes)
	}
	if !first.Accepted {
		t.Errorf("expected Accepted=true for statusDisplay=Accepted")
	}
	if first.Lang != "golang" {
		t.Errorf("Lang = %q", first.Lang)
	}
	if first.FlagType != "WHITE" {
		t.Errorf("FlagType = %q", first.FlagType)
	}
	if want := time.Unix(1777214044, 0); !first.OccurredAt.Equal(want) {
		t.Errorf("OccurredAt = %v, want %v", first.OccurredAt, want)
	}

	// statusDisplay != "Accepted" must surface as Accepted=false so the SR
	// scheduler can ignore failed submissions when computing next-due.
	if subs[2].Accepted {
		t.Errorf("Wrong Answer must yield Accepted=false")
	}

	// hasNext=false + lastKey=null means we paged everything; nextKey should
	// be the empty string so callers can use `for nextKey != ""` to drain.
	if nextKey != "" {
		t.Errorf("nextKey = %q, want empty", nextKey)
	}
}

func TestSubmissionList_PaginatesViaLastKey(t *testing.T) {
	d := &routedDoer{byOp: map[string]string{
		"submissionList": `{"data":{"questionSubmissionList":{"lastKey":"cursor-abc","hasNext":true,"submissions":[
			{"id":"1","status":10,"statusDisplay":"Accepted","lang":"golang","timestamp":"1700000000","notes":"","flagType":"WHITE"}
		]}}}`,
	}}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	_, nextKey, err := c.SubmissionList(context.Background(), "two-sum", "", 20)
	if err != nil {
		t.Fatalf("SubmissionList: %v", err)
	}
	if nextKey != "cursor-abc" {
		t.Errorf("nextKey = %q, want cursor-abc", nextKey)
	}
}
