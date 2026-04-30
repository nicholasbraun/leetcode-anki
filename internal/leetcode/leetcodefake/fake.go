// Package leetcodefake provides a method-level fake of *leetcode.Client for
// tests. Seed the public fields with canned data; install hooks to override
// any method's default behavior (e.g. block on context for cancel-flow
// tests, return canned errors for failure paths).
package leetcodefake

import (
	"context"
	"errors"

	"leetcode-anki/internal/leetcode"
)

// Fake implements every public method on *leetcode.Client. When a hook is
// non-nil, it shadows the default behavior; otherwise the method consults
// the seeded fields.
type Fake struct {
	Lists         []leetcode.FavoriteList
	Questions     map[string][]leetcode.Question     // by list slug
	Details       map[string]*leetcode.ProblemDetail // by problem title slug
	Submissions   map[string][]leetcode.Submission   // by problem title slug
	Progress      []leetcode.ProgressQuestion
	ProgressTotal int
	// UserStatus is what Verify returns by default. The zero value is
	// {IsSignedIn: false} which Verify reports as a signed-out error —
	// tests that exercise the success path should seed at least
	// {IsSignedIn: true}.
	UserStatus leetcode.UserStatus

	RunResult    *leetcode.RunResult
	SubmitResult *leetcode.SubmitResult
	NoteErr      error

	ListsHook          func(ctx context.Context) ([]leetcode.FavoriteList, error)
	QuestionsHook      func(ctx context.Context, slug string, skip, limit int) (*leetcode.FavoriteQuestionListResult, error)
	DetailHook         func(ctx context.Context, slug string) (*leetcode.ProblemDetail, error)
	SubmissionListHook func(ctx context.Context, slug, nextKey string, limit int) ([]leetcode.Submission, string, error)
	ProgressHook       func(ctx context.Context, skip, limit int) ([]leetcode.ProgressQuestion, int, error)
	RunHook            func(ctx context.Context, slug, lang, qid, code, in, meta string) (*leetcode.RunResult, error)
	SubmitHook         func(ctx context.Context, slug, lang, qid, code string) (*leetcode.SubmitResult, error)
	UpdateNoteHook     func(ctx context.Context, submissionID, note string, tagIDs []int, flagType string) error
	VerifyHook         func(ctx context.Context) (leetcode.UserStatus, error)

	// DetailCalls records the slugs ProblemDetail was invoked with, in
	// order. Tests inspect and reset it directly (matching the prior
	// fakeClient.calls API). Not concurrency-safe; tests that fan out
	// ProblemDetail across goroutines must drain before asserting.
	DetailCalls []string
}

func (f *Fake) MyFavoriteLists(ctx context.Context) ([]leetcode.FavoriteList, error) {
	if f.ListsHook != nil {
		return f.ListsHook(ctx)
	}
	return f.Lists, nil
}

func (f *Fake) FavoriteQuestionList(ctx context.Context, slug string, skip, limit int) (*leetcode.FavoriteQuestionListResult, error) {
	if f.QuestionsHook != nil {
		return f.QuestionsHook(ctx, slug, skip, limit)
	}
	qs := f.Questions[slug]
	return &leetcode.FavoriteQuestionListResult{
		Questions:   qs,
		TotalLength: len(qs),
		HasMore:     false,
	}, nil
}

func (f *Fake) ProblemDetail(ctx context.Context, titleSlug string) (*leetcode.ProblemDetail, error) {
	f.DetailCalls = append(f.DetailCalls, titleSlug)
	if f.DetailHook != nil {
		return f.DetailHook(ctx, titleSlug)
	}
	if d, ok := f.Details[titleSlug]; ok {
		return d, nil
	}
	return nil, errors.New("not found")
}

func (f *Fake) SubmissionList(ctx context.Context, slug, nextKey string, limit int) ([]leetcode.Submission, string, error) {
	if f.SubmissionListHook != nil {
		return f.SubmissionListHook(ctx, slug, nextKey, limit)
	}
	return f.Submissions[slug], "", nil
}

func (f *Fake) UserProgress(ctx context.Context, skip, limit int) ([]leetcode.ProgressQuestion, int, error) {
	if f.ProgressHook != nil {
		return f.ProgressHook(ctx, skip, limit)
	}
	total := f.ProgressTotal
	if total == 0 {
		total = len(f.Progress)
	}
	return f.Progress, total, nil
}

func (f *Fake) InterpretSolution(ctx context.Context, slug, lang, qid, code, in, meta string) (*leetcode.RunResult, error) {
	if f.RunHook != nil {
		return f.RunHook(ctx, slug, lang, qid, code, in, meta)
	}
	return f.RunResult, nil
}

func (f *Fake) Submit(ctx context.Context, slug, lang, qid, code string) (*leetcode.SubmitResult, error) {
	if f.SubmitHook != nil {
		return f.SubmitHook(ctx, slug, lang, qid, code)
	}
	return f.SubmitResult, nil
}

func (f *Fake) UpdateSubmissionNote(ctx context.Context, submissionID, note string, tagIDs []int, flagType string) error {
	if f.UpdateNoteHook != nil {
		return f.UpdateNoteHook(ctx, submissionID, note, tagIDs, flagType)
	}
	return f.NoteErr
}

func (f *Fake) Verify(ctx context.Context) (leetcode.UserStatus, error) {
	if f.VerifyHook != nil {
		return f.VerifyHook(ctx)
	}
	if !f.UserStatus.IsSignedIn {
		return leetcode.UserStatus{}, errors.New("fake verify: not signed in")
	}
	return f.UserStatus, nil
}
