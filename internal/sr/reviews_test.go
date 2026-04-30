package sr

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"leetcode-anki/internal/leetcode"
)

var errBoom = errors.New("boom")

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

// Due aggregates UserProgress + Status filtering. The test verifies that
// only AC Problems whose Status.Due(now) is true end up in the result, and
// that DueProblem carries display metadata so the TUI doesn't have to
// re-fetch.
func TestDue_FiltersToAcceptedAndDue(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := &fakeClient{
		progressResp: []leetcode.ProgressQuestion{
			// AC + first Submit was 2 days ago → 1-day interval → due.
			{TitleSlug: "two-sum", Title: "Two Sum", FrontendID: "1", Difficulty: "EASY",
				LastAccepted: true, LastSubmittedAt: at.AddDate(0, 0, -2)},
			// AC + first Submit was just now → 1-day interval → NOT due.
			{TitleSlug: "valid-anagram", Title: "Valid Anagram", FrontendID: "242", Difficulty: "EASY",
				LastAccepted: true, LastSubmittedAt: at},
			// Last result WA → never enters the SR rotation.
			{TitleSlug: "broken", Title: "Broken", FrontendID: "999", Difficulty: "HARD",
				LastAccepted: false, LastSubmittedAt: at.AddDate(0, 0, -10)},
		},
	}
	r := newTestReviews(t, fc)

	// Seed the cache so Status doesn't try to fetch SubmissionList for each.
	r.cache.Slugs["two-sum"] = slugEntry{Submissions: []cachedSubmission{
		{ID: "1", OccurredAt: at.AddDate(0, 0, -2), Accepted: true},
	}}
	r.cache.Slugs["valid-anagram"] = slugEntry{Submissions: []cachedSubmission{
		{ID: "2", OccurredAt: at, Accepted: true},
	}}

	due, err := r.Due(context.Background(), at)
	if err != nil {
		t.Fatalf("Due: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("expected 1 due Problem, got %d: %+v", len(due), due)
	}
	if due[0].TitleSlug != "two-sum" {
		t.Errorf("expected two-sum, got %q", due[0].TitleSlug)
	}
	if due[0].Title != "Two Sum" || due[0].FrontendID != "1" || due[0].Difficulty != "EASY" {
		t.Errorf("missing display metadata: %+v", due[0])
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

// On a brand-new Problem the first three ratings (Again/Hard/Good) all
// graduate at +1 day; Easy jumps ahead via the graduating bonus. So the
// rating modal shows three identical "due tomorrow" lines and a single
// longer line for Easy.
func TestPreview_FirstReview_EasyGraduatesFaster(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := &fakeClient{}
	r := newTestReviews(t, fc)

	got, err := r.Preview(context.Background(), "two-sum", at)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	tomorrow := at.AddDate(0, 0, 1)
	for i := 0; i < 3; i++ {
		if !got[i].Equal(tomorrow) {
			t.Errorf("rating %d: NextDue = %v, want %v", i+1, got[i], tomorrow)
		}
	}
	if !got[3].After(got[2]) {
		t.Errorf("Easy (%v) should graduate later than Good (%v)", got[3], got[2])
	}
}

// After several reviews, SM-2's compounding diverges the four ratings:
// Again resets, Hard ≤ Good ≤ Easy. Strict ordering keeps the test
// resilient to ease-factor tweaks.
func TestPreview_NthReview_StrictlyOrdered(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := &fakeClient{}
	r := newTestReviews(t, fc)
	r.cache.Slugs["two-sum"] = slugEntry{Submissions: []cachedSubmission{
		{ID: "1", OccurredAt: at.AddDate(0, 0, -10), Accepted: true, Notes: "[anki:3]"},
		{ID: "2", OccurredAt: at.AddDate(0, 0, -7), Accepted: true, Notes: "[anki:3]"},
		{ID: "3", OccurredAt: at.AddDate(0, 0, -1), Accepted: true, Notes: "[anki:3]"},
	}}

	got, err := r.Preview(context.Background(), "two-sum", at)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if !got[0].Before(got[1]) {
		t.Errorf("Again (%v) should be earlier than Hard (%v)", got[0], got[1])
	}
	if !got[1].Before(got[2]) {
		t.Errorf("Hard (%v) should be earlier than Good (%v)", got[1], got[2])
	}
	if !got[2].Before(got[3]) {
		t.Errorf("Good (%v) should be earlier than Easy (%v)", got[2], got[3])
	}
}

// When the underlying SubmissionList call fails (e.g. cold cache + network
// error), Preview returns the zero-array and the error so the TUI can
// degrade to "preview unavailable" rather than rendering bogus dates.
func TestPreview_StatusErrorPropagates(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := &fakeClient{listErr: errBoom}
	r := newTestReviews(t, fc)

	got, err := r.Preview(context.Background(), "two-sum", at)
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	for i, p := range got {
		if !p.IsZero() {
			t.Errorf("rating %d: expected zero time on error, got %v", i+1, p)
		}
	}
}

// sessionFixture wires a reviews against a UserProgress payload covering
// the four classes of slug Session has to handle: AC+due, AC+not-due,
// attempted-but-failed, and never-attempted (absent from UserProgress).
// Returns the prepared reviews and the timestamp every test should pass
// as `now`.
func sessionFixture(t *testing.T) (*reviews, time.Time) {
	t.Helper()
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := &fakeClient{progressResp: []leetcode.ProgressQuestion{
		{TitleSlug: "due-a", Title: "Due A", FrontendID: "1", Difficulty: "EASY", LastAccepted: true},
		{TitleSlug: "due-b", Title: "Due B", FrontendID: "2", Difficulty: "EASY", LastAccepted: true},
		{TitleSlug: "due-c", Title: "Due C", FrontendID: "3", Difficulty: "MEDIUM", LastAccepted: true},
		{TitleSlug: "fresh-ac", Title: "Fresh AC", FrontendID: "4", Difficulty: "EASY", LastAccepted: true},
		{TitleSlug: "failed", Title: "Failed", FrontendID: "5", Difficulty: "HARD", LastAccepted: false},
	}}
	r := newTestReviews(t, fc)
	// AC+due: first Submit was 2 days ago → SM-2 first interval is 1 day → due now.
	for _, slug := range []string{"due-a", "due-b", "due-c"} {
		r.cache.Slugs[slug] = slugEntry{Submissions: []cachedSubmission{
			{ID: slug, OccurredAt: at.AddDate(0, 0, -2), Accepted: true},
		}}
	}
	// AC+not-due: first Submit was just now → 1-day interval → not due.
	r.cache.Slugs["fresh-ac"] = slugEntry{Submissions: []cachedSubmission{
		{ID: "fresh-ac", OccurredAt: at, Accepted: true},
	}}
	return r, at
}

// Session must put every due item before every new item so the queue
// renders "what's overdue first, then a few new ones" — the user-facing
// promise of Review Mode.
func TestSession_DueItemsBeforeNewItems(t *testing.T) {
	r, now := sessionFixture(t)

	sess, err := r.Session(context.Background(), SessionConfig{
		Slugs: []string{"failed", "due-a", "newbie", "due-b"},
		// "newbie" isn't in UserProgress: never-attempted.
		MaxDue: 5, MaxNew: 5,
	}, now)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if len(sess.Items) != 4 {
		t.Fatalf("expected 4 items, got %d: %+v", len(sess.Items), sess.Items)
	}
	for i := 0; i < 2; i++ {
		if sess.Items[i].Kind != KindDue {
			t.Errorf("Items[%d].Kind = %v, want KindDue", i, sess.Items[i].Kind)
		}
	}
	for i := 2; i < 4; i++ {
		if sess.Items[i].Kind != KindNew {
			t.Errorf("Items[%d].Kind = %v, want KindNew", i, sess.Items[i].Kind)
		}
	}
}

// Slugs ordering is load-bearing: callers (TUI) pass slugs in display
// order and rely on Session to preserve that order within each bucket.
func TestSession_PreservesSlugsOrderWithinBuckets(t *testing.T) {
	r, now := sessionFixture(t)

	sess, err := r.Session(context.Background(), SessionConfig{
		Slugs:  []string{"due-c", "failed", "due-a", "newbie", "due-b"},
		MaxDue: 5, MaxNew: 5,
	}, now)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	want := []string{"due-c", "due-a", "due-b", "failed", "newbie"}
	got := make([]string, len(sess.Items))
	for i, it := range sess.Items {
		got[i] = it.TitleSlug
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("slug order = %v, want %v", got, want)
	}
}

// MaxDue caps how many KindDue items appear in the queue; DueTotal must
// still report the uncapped count so the TUI footer can render "2 of 5 due".
func TestSession_DueBucketCappedTotalsUncapped(t *testing.T) {
	r, now := sessionFixture(t)

	sess, err := r.Session(context.Background(), SessionConfig{
		Slugs:  []string{"due-a", "due-b", "due-c"},
		MaxDue: 2, MaxNew: 5,
	}, now)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if sess.DueCount != 2 {
		t.Errorf("DueCount = %d, want 2", sess.DueCount)
	}
	if sess.DueTotal != 3 {
		t.Errorf("DueTotal = %d, want 3 (uncapped)", sess.DueTotal)
	}
	if len(sess.Items) != 2 {
		t.Errorf("Items length = %d, want 2", len(sess.Items))
	}
}

// Symmetric to the due-cap test: MaxNew caps KindNew, NewTotal stays uncapped.
func TestSession_NewBucketCappedTotalsUncapped(t *testing.T) {
	r, now := sessionFixture(t)

	sess, err := r.Session(context.Background(), SessionConfig{
		Slugs:  []string{"failed", "newbie-1", "newbie-2", "newbie-3"},
		MaxDue: 5, MaxNew: 1,
	}, now)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if sess.NewCount != 1 {
		t.Errorf("NewCount = %d, want 1", sess.NewCount)
	}
	if sess.NewTotal != 4 {
		t.Errorf("NewTotal = %d, want 4 (uncapped)", sess.NewTotal)
	}
	if sess.Items[0].TitleSlug != "failed" {
		t.Errorf("first new = %q, want %q (preserves Slugs order)", sess.Items[0].TitleSlug, "failed")
	}
}

// MaxDue=0 is the supported way to ask for new-only — the queue must
// drop the due bucket entirely (not emit a single placeholder).
func TestSession_MaxDueZeroOmitsDueBucket(t *testing.T) {
	r, now := sessionFixture(t)

	sess, err := r.Session(context.Background(), SessionConfig{
		Slugs:  []string{"due-a", "failed"},
		MaxDue: 0, MaxNew: 5,
	}, now)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if sess.DueCount != 0 {
		t.Errorf("DueCount = %d, want 0", sess.DueCount)
	}
	if sess.DueTotal != 1 {
		t.Errorf("DueTotal = %d, want 1 (still counted)", sess.DueTotal)
	}
	if len(sess.Items) != 1 || sess.Items[0].Kind != KindNew {
		t.Errorf("Items = %+v, want one KindNew", sess.Items)
	}
}

// MaxNew=0 mirrors the old Due()-shaped behavior — only due Items returned.
func TestSession_MaxNewZeroOmitsNewBucket(t *testing.T) {
	r, now := sessionFixture(t)

	sess, err := r.Session(context.Background(), SessionConfig{
		Slugs:  []string{"due-a", "failed", "newbie"},
		MaxDue: 5, MaxNew: 0,
	}, now)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if sess.NewCount != 0 {
		t.Errorf("NewCount = %d, want 0", sess.NewCount)
	}
	if sess.NewTotal != 2 {
		t.Errorf("NewTotal = %d, want 2 (still counted)", sess.NewTotal)
	}
	if len(sess.Items) != 1 || sess.Items[0].Kind != KindDue {
		t.Errorf("Items = %+v, want one KindDue", sess.Items)
	}
}

// A slug that's never appeared in UserProgress is treated as new — it's
// the never-attempted case (no LeetCode submission history at all).
func TestSession_UnknownSlugTreatedAsNew(t *testing.T) {
	r, now := sessionFixture(t)

	sess, err := r.Session(context.Background(), SessionConfig{
		Slugs:  []string{"completely-unknown"},
		MaxDue: 5, MaxNew: 5,
	}, now)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if len(sess.Items) != 1 || sess.Items[0].Kind != KindNew {
		t.Fatalf("expected one KindNew item, got %+v", sess.Items)
	}
	if sess.Items[0].TitleSlug != "completely-unknown" {
		t.Errorf("TitleSlug = %q, want %q", sess.Items[0].TitleSlug, "completely-unknown")
	}
}

// AC+not-due Problems are review-rotation members but Status.Due == false:
// they belong in neither bucket. Otherwise Review Mode would surface
// Problems the user just successfully reviewed yesterday.
func TestSession_AcceptedButNotDueExcludedFromBothBuckets(t *testing.T) {
	r, now := sessionFixture(t)

	sess, err := r.Session(context.Background(), SessionConfig{
		Slugs:  []string{"fresh-ac"},
		MaxDue: 5, MaxNew: 5,
	}, now)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if len(sess.Items) != 0 {
		t.Errorf("expected no items, got %+v", sess.Items)
	}
	if sess.DueTotal != 0 || sess.NewTotal != 0 {
		t.Errorf("Totals = (%d due, %d new), want both 0", sess.DueTotal, sess.NewTotal)
	}
}

// Due Items must carry NextDue and Reviews so the TUI can render
// "due 3d ago" badges without re-calling Status.
func TestSession_DueItemsCarryScheduleMetadata(t *testing.T) {
	r, now := sessionFixture(t)

	sess, err := r.Session(context.Background(), SessionConfig{
		Slugs:  []string{"due-a"},
		MaxDue: 5, MaxNew: 5,
	}, now)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if len(sess.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(sess.Items))
	}
	it := sess.Items[0]
	if it.NextDue.IsZero() {
		t.Error("KindDue NextDue must be set")
	}
	if it.Reviews != 1 {
		t.Errorf("Reviews = %d, want 1", it.Reviews)
	}
	if it.Title != "Due A" || it.FrontendID != "1" || it.Difficulty != "EASY" {
		t.Errorf("missing display metadata from UserProgress: %+v", it)
	}
}

// New Items have no schedule (zero NextDue, zero Reviews). Display
// metadata still comes through when UserProgress has the slug (the
// attempted-but-failed case); never-attempted slugs come back with
// only TitleSlug populated.
func TestSession_NewItemsZeroSchedule(t *testing.T) {
	r, now := sessionFixture(t)

	sess, err := r.Session(context.Background(), SessionConfig{
		Slugs:  []string{"failed", "completely-unknown"},
		MaxDue: 5, MaxNew: 5,
	}, now)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if len(sess.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(sess.Items))
	}
	for _, it := range sess.Items {
		if !it.NextDue.IsZero() {
			t.Errorf("KindNew %q: NextDue = %v, want zero", it.TitleSlug, it.NextDue)
		}
		if it.Reviews != 0 {
			t.Errorf("KindNew %q: Reviews = %d, want 0", it.TitleSlug, it.Reviews)
		}
	}
	// "failed" is in UserProgress so its display metadata is filled.
	if sess.Items[0].Title != "Failed" || sess.Items[0].FrontendID != "5" {
		t.Errorf("attempted-but-failed should carry UserProgress metadata: %+v", sess.Items[0])
	}
	// "completely-unknown" has no UserProgress row — only TitleSlug.
	if sess.Items[1].Title != "" || sess.Items[1].FrontendID != "" {
		t.Errorf("never-attempted should have empty metadata: %+v", sess.Items[1])
	}
}
