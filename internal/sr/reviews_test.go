package sr

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"leetcode-anki/internal/leetcode"
)

type fakeClient struct {
	listResp     []leetcode.Submission
	listKey      string
	listErr      error
	progressResp []leetcode.ProgressQuestion
	progressErr  error
	noteWrites   []noteWrite
	noteErr      error
}

type noteWrite struct {
	submissionID, note, flagType string
	tagIDs                       []int
}

func (f *fakeClient) SubmissionList(_ context.Context, _, _ string, _ int) ([]leetcode.Submission, string, error) {
	return f.listResp, f.listKey, f.listErr
}

func (f *fakeClient) UserProgress(_ context.Context, _, _ int) ([]leetcode.ProgressQuestion, int, error) {
	return f.progressResp, len(f.progressResp), f.progressErr
}

func (f *fakeClient) UpdateSubmissionNote(_ context.Context, submissionID, note string, tagIDs []int, flagType string) error {
	f.noteWrites = append(f.noteWrites, noteWrite{submissionID: submissionID, note: note, flagType: flagType, tagIDs: tagIDs})
	return f.noteErr
}

func newTestReviews(t *testing.T, fc *fakeClient) *reviews {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sr.json")
	c, err := loadCache(path)
	if err != nil {
		t.Fatalf("loadCache: %v", err)
	}
	return &reviews{lc: fc, sched: sm2{}, cache: c, path: path}
}

func TestStatus_NoAcceptedSubmissions_NotTracked(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := &fakeClient{listResp: []leetcode.Submission{
		{ID: "1", OccurredAt: at, Accepted: false},
		{ID: "2", OccurredAt: at.Add(time.Hour), Accepted: false},
	}}
	r := newTestReviews(t, fc)

	got, err := r.Status(context.Background(), "two-sum", at.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if got.Tracked {
		t.Errorf("Tracked must be false when only failed submissions exist")
	}
}

func TestStatus_FirstAcceptedTriggersBaseline(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := &fakeClient{listResp: []leetcode.Submission{
		// LeetCode returns descending; sort happens inside buildReviews.
		{ID: "2", OccurredAt: at.Add(time.Hour), Accepted: true},
	}}
	r := newTestReviews(t, fc)

	got, err := r.Status(context.Background(), "two-sum", at)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !got.Tracked {
		t.Fatal("expected Tracked after first AC")
	}
	want := at.Add(time.Hour).AddDate(0, 0, 1)
	if !got.NextDue.Equal(want) {
		t.Errorf("NextDue = %v, want %v", got.NextDue, want)
	}
	if got.Reviews != 1 {
		t.Errorf("Reviews = %d, want 1", got.Reviews)
	}
}

// Pre-baseline failures (Wrong Answer before the first Accepted) must not
// enter the review history. Otherwise a struggled-with Problem would enter
// SR with a long lapse chain and schedule months out from the wrong base.
func TestStatus_PreBaselineFailuresExcluded(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := &fakeClient{listResp: []leetcode.Submission{
		{ID: "1", OccurredAt: at, Accepted: false},
		{ID: "2", OccurredAt: at.Add(time.Hour), Accepted: false},
		{ID: "3", OccurredAt: at.Add(2 * time.Hour), Accepted: true},
	}}
	r := newTestReviews(t, fc)

	got, _ := r.Status(context.Background(), "two-sum", at)
	if got.Reviews != 1 {
		t.Errorf("Reviews = %d, want 1 (pre-baseline failures excluded)", got.Reviews)
	}
}

// SM-2's first two intervals are fixed (1 day, 6 days), so quality
// differences only influence intervals from the third review onward via
// the ease factor. Three reviews of "Easy" (q=5) should schedule further
// out than three "Hard" (q=3) reviews.
func TestStatus_ExplicitTagInfluencesSchedule(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	timeline := func(tag string) []leetcode.Submission {
		return []leetcode.Submission{
			{ID: "1", OccurredAt: at, Accepted: true, Notes: tag},
			{ID: "2", OccurredAt: at.AddDate(0, 0, 1), Accepted: true, Notes: tag},
			{ID: "3", OccurredAt: at.AddDate(0, 0, 7), Accepted: true, Notes: tag},
		}
	}

	easyClient := &fakeClient{listResp: timeline("[anki:4]")}
	hardClient := &fakeClient{listResp: timeline("[anki:2]")}

	easyStatus, _ := newTestReviews(t, easyClient).Status(context.Background(), "two-sum", at)
	hardStatus, _ := newTestReviews(t, hardClient).Status(context.Background(), "two-sum", at)

	if !easyStatus.NextDue.After(hardStatus.NextDue) {
		t.Errorf("Easy NextDue (%v) should be later than Hard (%v)", easyStatus.NextDue, hardStatus.NextDue)
	}
}

func TestRecord_ImplicitRatingDoesNotWriteNote(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := &fakeClient{}
	r := newTestReviews(t, fc)

	if err := r.Record(context.Background(), "two-sum", "1988694277", 0, at); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(fc.noteWrites) != 0 {
		t.Errorf("expected no UpdateSubmissionNote call for rating=0; got %d", len(fc.noteWrites))
	}
}

func TestRecord_ExplicitRatingWritesAnkiTag(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := &fakeClient{}
	r := newTestReviews(t, fc)

	if err := r.Record(context.Background(), "two-sum", "1988694277", 3, at); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(fc.noteWrites) != 1 {
		t.Fatalf("expected 1 note write, got %d", len(fc.noteWrites))
	}
	w := fc.noteWrites[0]
	if w.submissionID != "1988694277" {
		t.Errorf("submissionID = %q", w.submissionID)
	}
	if w.note != "[anki:3]" {
		t.Errorf("note = %q, want [anki:3]", w.note)
	}
	if w.flagType != "WHITE" {
		t.Errorf("flagType = %q, want WHITE", w.flagType)
	}
}

// Record must invalidate the slug's cache so the next Status refreshes
// and picks up the just-completed submission. Otherwise the badge would
// keep showing the pre-Submit state until the user manually refreshed.
func TestRecord_InvalidatesSlugCache(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := &fakeClient{}
	r := newTestReviews(t, fc)
	r.cache.Slugs["two-sum"] = slugEntry{FetchedAt: at, Submissions: []cachedSubmission{{ID: "old"}}}

	if err := r.Record(context.Background(), "two-sum", "1", 0, at); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if _, present := r.cache.Slugs["two-sum"]; present {
		t.Error("expected slug cache to be invalidated")
	}
}

func TestStatus_UsesCacheOnSecondCall(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := &fakeClient{listResp: []leetcode.Submission{
		{ID: "1", OccurredAt: at, Accepted: true},
	}}
	r := newTestReviews(t, fc)

	if _, err := r.Status(context.Background(), "two-sum", at); err != nil {
		t.Fatalf("Status: %v", err)
	}
	// Drop fake responses; if Status refetches we'll see Tracked=false.
	fc.listResp = nil
	got, err := r.Status(context.Background(), "two-sum", at)
	if err != nil {
		t.Fatalf("Status (cached): %v", err)
	}
	if !got.Tracked {
		t.Errorf("expected cached Status to remain Tracked")
	}
}

func TestDue_OnlyTrackedAndPastNextDue(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if (Status{Tracked: false}).Due(at) {
		t.Error("untracked must not be Due")
	}
	if (Status{Tracked: true, NextDue: at.Add(time.Hour)}).Due(at) {
		t.Error("not yet Due")
	}
	if !(Status{Tracked: true, NextDue: at}).Due(at.Add(time.Hour)) {
		t.Error("past NextDue should be Due")
	}
}
