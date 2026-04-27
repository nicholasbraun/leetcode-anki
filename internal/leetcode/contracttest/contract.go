// Package contracttest defines a single suite of shape-invariant
// assertions over the *leetcode.Client surface. The same suite runs
// against an in-memory fake (every fast `go test ./...`) and against
// the real LeetCode API behind the `integration` build tag. If both
// pass, the fake is a faithful stand-in; if real-side starts failing,
// LeetCode changed something user-visible.
package contracttest

import (
	"context"
	"strings"
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
	InterpretSolution(ctx context.Context, slug, lang, qid, code, in, meta string) (*leetcode.RunResult, error)
	Submit(ctx context.Context, slug, lang, qid, code string) (*leetcode.SubmitResult, error)
	UpdateSubmissionNote(ctx context.Context, submissionID, note string, tagIDs []int, flagType string) error
}

// PassingSolution is a known-accepted solution for the fixture's problem.
// The same Code is reused by the InterpretSolution and Submit subtests so
// re-runs against the live API are content-deduped by LeetCode's judge
// (same hash → same verdict, no new judge work).
//
// MetaData is the problem's wire-format MetaData JSON (a `{"name", "params", "return"}`
// blob). InterpretSolution forwards it so per-case input splitting can find
// the parameter count; "" is acceptable but degrades the per-case input view.
type PassingSolution struct {
	Lang     string
	Code     string
	Input    string // sample stdin for InterpretSolution, e.g. "[2,7,11,15]\n9"
	MetaData string
}

// Fixture names the test-account state both the fake and the real client
// must satisfy before ContractTest can pass. The real client's account
// is set up to match this fixture (see the live-contract docs); the fake
// is seeded to match.
//
// Note that the list slug is *not* part of the fixture: the default
// "Favorite Questions" list LeetCode gives every account has an opaque,
// account-specific slug, so ContractTest discovers it at runtime by
// picking the first list with ≥1 question. KnownProblemSlug must be a
// member of whichever list that turns out to be.
type Fixture struct {
	KnownProblemSlug string // a problem in some list (e.g. "two-sum")
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

	// Discovered in MyFavoriteLists/has-content, consumed by
	// FavoriteQuestionList/known-list-has-questions. Subtests are
	// sequential so a closure-captured value is safe here.
	var listSlugWithContent string

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
			if listSlugWithContent == "" && l.QuestionCount > 0 {
				listSlugWithContent = l.Slug
			}
		}
		if listSlugWithContent == "" {
			t.Fatal("no list with QuestionCount > 0; add a problem to one list")
		}
	})

	t.Run("FavoriteQuestionList/known-list-has-questions", func(t *testing.T) {
		if listSlugWithContent == "" {
			t.Skip("MyFavoriteLists subtest didn't resolve a slug")
		}
		res, err := api.FavoriteQuestionList(ctx, listSlugWithContent, 0, 50)
		if err != nil {
			t.Fatalf("FavoriteQuestionList(%q): %v", listSlugWithContent, err)
		}
		if res == nil || len(res.Questions) == 0 {
			t.Fatalf("list %q is empty; fixture requires ≥1 question", listSlugWithContent)
		}
		// LeetCode's GraphQL response varies the difficulty's case:
		// the favorites query returns "EASY" while ProblemDetail returns
		// "Easy". Production styling already case-folds (see
		// tui.difficultyStyle) — match that here.
		valid := map[string]bool{"easy": true, "medium": true, "hard": true}
		for i, q := range res.Questions {
			if q.TitleSlug == "" {
				t.Errorf("questions[%d].TitleSlug is empty", i)
			}
			if !valid[strings.ToLower(q.Difficulty)] {
				t.Errorf("questions[%d].Difficulty = %q, want Easy/Medium/Hard (any case)", i, q.Difficulty)
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
			fx.PassingSolution.MetaData,
		)
		if err != nil {
			t.Fatalf("InterpretSolution: %v", err)
		}
		if res == nil {
			t.Fatal("InterpretSolution returned nil RunResult")
		}
		if len(res.Cases) == 0 {
			t.Error("Cases is empty; per-case verdict materialization broken")
		}
		for i, c := range res.Cases {
			if c.Output == "" {
				t.Errorf("Cases[%d].Output is empty", i)
			}
			if c.Input == "" {
				t.Errorf("Cases[%d].Input is empty; dataInput split lost the input", i)
			}
		}
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
		// LeetCode's SubmissionFlagTypeEnum rejects empty strings — a
		// fresh submission's FlagType is "WHITE" (see SubmissionList
		// fixtures). Production code round-trips this from a prior
		// SubmissionList read; the contract takes the shortcut of using
		// the known default since it's not testing the round-trip
		// pattern itself, just that the mutation accepts a valid input.
		if err := api.UpdateSubmissionNote(ctx, submissionID, fx.NoteText, nil, "WHITE"); err != nil {
			t.Errorf("UpdateSubmissionNote: %v", err)
		}
	})
}
