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

func TestUserProgress_DecodesCapturedResponse(t *testing.T) {
	d := &routedDoer{byOp: map[string]string{
		"userProgressQuestionList": `{"data":{"userProgressQuestionList":{"totalNum":66,"questions":[
			{"frontendId":"1","title":"Two Sum","titleSlug":"two-sum","difficulty":"EASY","lastSubmittedAt":"2026-04-26T14:34:04+00:00","numSubmitted":8,"questionStatus":"SOLVED","lastResult":"AC","topicTags":[]},
			{"frontendId":"2095","title":"Delete the Middle Node","titleSlug":"delete-the-middle-node-of-a-linked-list","difficulty":"MEDIUM","lastSubmittedAt":"2025-06-02T07:49:26+00:00","numSubmitted":13,"questionStatus":"SOLVED","lastResult":"WA","topicTags":[]}
		]}}}`,
	}}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	progress, total, err := c.UserProgress(context.Background(), 0, 50)
	if err != nil {
		t.Fatalf("UserProgress: %v", err)
	}
	if total != 66 {
		t.Errorf("total = %d, want 66", total)
	}
	if len(progress) != 2 {
		t.Fatalf("got %d, want 2", len(progress))
	}

	if progress[0].TitleSlug != "two-sum" {
		t.Errorf("TitleSlug = %q", progress[0].TitleSlug)
	}
	if progress[0].Title != "Two Sum" {
		t.Errorf("Title = %q", progress[0].Title)
	}
	if progress[0].FrontendID != "1" {
		t.Errorf("FrontendID = %q", progress[0].FrontendID)
	}
	if progress[0].Difficulty != "EASY" {
		t.Errorf("Difficulty = %q", progress[0].Difficulty)
	}
	if !progress[0].LastAccepted {
		t.Errorf("AC must yield LastAccepted=true")
	}
	if progress[0].NumSubmitted != 8 {
		t.Errorf("NumSubmitted = %d", progress[0].NumSubmitted)
	}
	wantTime, _ := time.Parse(time.RFC3339, "2026-04-26T14:34:04+00:00")
	if !progress[0].LastSubmittedAt.Equal(wantTime) {
		t.Errorf("LastSubmittedAt = %v, want %v", progress[0].LastSubmittedAt, wantTime)
	}

	// "WA" must not be treated as Accepted — SR scheduler would otherwise
	// pick a wrong-answer submission as a baseline Review.
	if progress[1].LastAccepted {
		t.Errorf("WA must yield LastAccepted=false")
	}
}

// A malformed `timestamp` field used to silently parse to 0 and surface as
// time.Unix(0, 0) — Jan 1, 1970 — which polluted the SR scheduler with
// "always overdue" entries. Skip the row instead so the caller never sees
// it.
func TestSubmissionList_SkipsRowWithBadTimestamp(t *testing.T) {
	d := &routedDoer{byOp: map[string]string{
		"submissionList": `{"data":{"questionSubmissionList":{"lastKey":null,"hasNext":false,"submissions":[
			{"id":"good","status":10,"statusDisplay":"Accepted","lang":"golang","timestamp":"1700000000","notes":"","flagType":"WHITE"},
			{"id":"bad","status":10,"statusDisplay":"Accepted","lang":"golang","timestamp":"not-a-number","notes":"","flagType":"WHITE"}
		]}}}`,
	}}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	subs, _, err := c.SubmissionList(context.Background(), "two-sum", "", 20)
	if err != nil {
		t.Fatalf("SubmissionList: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 submission (bad timestamp dropped), got %d: %+v", len(subs), subs)
	}
	if subs[0].ID != "good" {
		t.Errorf("expected the good row to survive, got %+v", subs[0])
	}
}

// Same shape as the timestamp test but for UserProgress, whose row carries
// LastSubmittedAt as RFC3339. An empty / malformed value used to parse to a
// zero time.Time, again giving the scheduler a 1-Jan-AD-1 due-date.
func TestUserProgress_SkipsRowWithBadLastSubmittedAt(t *testing.T) {
	d := &routedDoer{byOp: map[string]string{
		"userProgressQuestionList": `{"data":{"userProgressQuestionList":{"totalNum":2,"questions":[
			{"frontendId":"1","title":"Good","titleSlug":"good","difficulty":"EASY","lastSubmittedAt":"2026-04-26T14:34:04+00:00","numSubmitted":1,"questionStatus":"SOLVED","lastResult":"AC","topicTags":[]},
			{"frontendId":"2","title":"Bad","titleSlug":"bad","difficulty":"EASY","lastSubmittedAt":"","numSubmitted":1,"questionStatus":"SOLVED","lastResult":"AC","topicTags":[]}
		]}}}`,
	}}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	progress, _, err := c.UserProgress(context.Background(), 0, 50)
	if err != nil {
		t.Fatalf("UserProgress: %v", err)
	}
	if len(progress) != 1 {
		t.Fatalf("expected 1 progress row (bad lastSubmittedAt dropped), got %d: %+v", len(progress), progress)
	}
	if progress[0].TitleSlug != "good" {
		t.Errorf("expected the good row to survive, got %+v", progress[0])
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
