// Package contracttest defines a single suite of shape-invariant
// assertions over the *leetcode.Client surface. The same suite runs
// against an in-memory fake (every fast `go test ./...`) and against
// the real LeetCode API behind the `integration` build tag. If both
// pass, the fake is a faithful stand-in; if real-side starts failing,
// LeetCode changed something user-visible.
package contracttest

import (
	"context"
	"testing"

	"leetcode-anki/internal/leetcode"
)

// API is the subset of *leetcode.Client surface the contract exercises.
// Both *leetcode.Client and *leetcodefake.Fake satisfy it structurally;
// no production interface needs to exist on either side.
type API interface {
	MyFavoriteLists(ctx context.Context) ([]leetcode.FavoriteList, error)
	FavoriteQuestionList(ctx context.Context, slug string, skip, limit int) (*leetcode.FavoriteQuestionListResult, error)
	ProblemDetail(ctx context.Context, titleSlug string) (*leetcode.ProblemDetail, error)
	SubmissionList(ctx context.Context, slug, nextKey string, limit int) ([]leetcode.Submission, string, error)
	UserProgress(ctx context.Context, skip, limit int) ([]leetcode.ProgressQuestion, int, error)
	InterpretSolution(ctx context.Context, slug, lang, qid, code, in string) (*leetcode.RunResult, error)
	Submit(ctx context.Context, slug, lang, qid, code string) (*leetcode.SubmitResult, error)
	UpdateSubmissionNote(ctx context.Context, submissionID, note string, tagIDs []int, flagType string) error
}

// PassingSolution is a known-accepted solution for the fixture's problem.
// The same Code is reused by the InterpretSolution and Submit subtests so
// re-runs against the live API are content-deduped by LeetCode's judge
// (same hash → same verdict, no new judge work).
type PassingSolution struct {
	Lang  string
	Code  string
	Input string // sample stdin for InterpretSolution, e.g. "[2,7,11,15]\n9"
}

// Fixture names the test-account state both the fake and the real client
// must satisfy before ContractTest can pass. The real client's account
// is set up to match this fixture (see the live-contract docs); the fake
// is seeded to match.
type Fixture struct {
	KnownListSlug    string // a list the account owns; must contain ≥1 question
	KnownProblemSlug string // a problem in that list (e.g. "two-sum")
	KnownQuestionID  string // numeric question_id for KnownProblemSlug; required by run/submit
	PassingSolution  PassingSolution
	NoteText         string // fixed value the note-update subtest writes; idempotent across runs
}

// ContractTest runs the invariant suite against api. The asymmetry between
// fake and real lives in the *setup*, not the assertions: both must arrive
// at api with state that satisfies the invariants below.
//
// Subtests run sequentially (no t.Parallel): UpdateSubmissionNote consumes
// the SubmissionID produced by Submit, so the order is load-bearing.
func ContractTest(t *testing.T, api API, fx Fixture) {
	t.Helper()
	ctx := context.Background()

	t.Run("MyFavoriteLists/has-content", func(t *testing.T) {
		lists, err := api.MyFavoriteLists(ctx)
		if err != nil {
			t.Fatalf("MyFavoriteLists: %v", err)
		}
		if len(lists) == 0 {
			t.Fatal("MyFavoriteLists returned 0 lists; fixture requires ≥1")
		}
		for i, l := range lists {
			if l.Slug == "" {
				t.Errorf("lists[%d].Slug is empty", i)
			}
			if l.Name == "" {
				t.Errorf("lists[%d].Name is empty", i)
			}
		}
	})

	t.Run("FavoriteQuestionList/known-list-has-questions", func(t *testing.T) {
		res, err := api.FavoriteQuestionList(ctx, fx.KnownListSlug, 0, 50)
		if err != nil {
			t.Fatalf("FavoriteQuestionList(%q): %v", fx.KnownListSlug, err)
		}
		if res == nil || len(res.Questions) == 0 {
			t.Fatalf("list %q is empty; fixture requires ≥1 question", fx.KnownListSlug)
		}
		valid := map[string]bool{"Easy": true, "Medium": true, "Hard": true}
		for i, q := range res.Questions {
			if q.TitleSlug == "" {
				t.Errorf("questions[%d].TitleSlug is empty", i)
			}
			if !valid[q.Difficulty] {
				t.Errorf("questions[%d].Difficulty = %q, want Easy/Medium/Hard", i, q.Difficulty)
			}
		}
	})

	t.Run("ProblemDetail/has-content-and-snippets", func(t *testing.T) {
		d, err := api.ProblemDetail(ctx, fx.KnownProblemSlug)
		if err != nil {
			t.Fatalf("ProblemDetail(%q): %v", fx.KnownProblemSlug, err)
		}
		if d == nil {
			t.Fatal("ProblemDetail returned nil")
		}
		if d.QuestionID == "" {
			t.Error("QuestionID is empty")
		}
		if d.TitleSlug != fx.KnownProblemSlug {
			t.Errorf("TitleSlug = %q, want %q", d.TitleSlug, fx.KnownProblemSlug)
		}
		if d.Content == "" {
			t.Error("Content is empty")
		}
		if len(d.CodeSnippets) == 0 {
			t.Fatal("no CodeSnippets")
		}
		for i, s := range d.CodeSnippets {
			if s.LangSlug == "" {
				t.Errorf("snippets[%d].LangSlug is empty", i)
			}
			if s.Code == "" {
				t.Errorf("snippets[%d].Code is empty", i)
			}
		}
	})

	t.Run("SubmissionList/decodes", func(t *testing.T) {
		subs, _, err := api.SubmissionList(ctx, fx.KnownProblemSlug, "", 20)
		if err != nil {
			t.Fatalf("SubmissionList(%q): %v", fx.KnownProblemSlug, err)
		}
		// Empty is acceptable on a freshly-set-up account; if non-empty,
		// every item's wire-decode invariants must hold.
		for i, s := range subs {
			if s.Lang == "" {
				t.Errorf("subs[%d].Lang is empty", i)
			}
			if s.OccurredAt.IsZero() {
				t.Errorf("subs[%d].OccurredAt is zero", i)
			}
		}
	})

	t.Run("UserProgress/decodes", func(t *testing.T) {
		items, total, err := api.UserProgress(ctx, 0, 50)
		if err != nil {
			t.Fatalf("UserProgress: %v", err)
		}
		if total < 0 {
			t.Errorf("total = %d, want ≥ 0", total)
		}
		for i, it := range items {
			if it.TitleSlug == "" {
				t.Errorf("items[%d].TitleSlug is empty", i)
			}
		}
	})

	t.Run("InterpretSolution/returns-verdict", func(t *testing.T) {
		res, err := api.InterpretSolution(ctx,
			fx.KnownProblemSlug,
			fx.PassingSolution.Lang,
			fx.KnownQuestionID,
			fx.PassingSolution.Code,
			fx.PassingSolution.Input,
		)
		if err != nil {
			t.Fatalf("InterpretSolution: %v", err)
		}
		if res == nil {
			t.Fatal("InterpretSolution returned nil RunResult")
		}
		// Verdict outcome doesn't matter — even Wrong Answer proves the
		// poll loop and JSON decode worked end-to-end.
	})

	var submissionID string
	t.Run("Submit/returns-submission", func(t *testing.T) {
		res, err := api.Submit(ctx,
			fx.KnownProblemSlug,
			fx.PassingSolution.Lang,
			fx.KnownQuestionID,
			fx.PassingSolution.Code,
		)
		if err != nil {
			t.Fatalf("Submit: %v", err)
		}
		if res == nil {
			t.Fatal("Submit returned nil SubmitResult")
		}
		if res.SubmissionID == "" {
			t.Error("SubmissionID is empty; UpdateSubmissionNote subtest will skip")
		}
		submissionID = res.SubmissionID
	})

	t.Run("UpdateSubmissionNote/round-trip", func(t *testing.T) {
		if submissionID == "" {
			t.Skip("no SubmissionID captured from Submit")
		}
		if err := api.UpdateSubmissionNote(ctx, submissionID, fx.NoteText, nil, ""); err != nil {
			t.Errorf("UpdateSubmissionNote: %v", err)
		}
	})
}
