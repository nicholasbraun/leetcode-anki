package leetcodefake

import (
	"context"
	"errors"
	"testing"

	"leetcode-anki/internal/leetcode"
)

func TestProblemDetail_SeededReturnsValue(t *testing.T) {
	f := &Fake{Details: map[string]*leetcode.ProblemDetail{
		"two-sum": {TitleSlug: "two-sum"},
	}}
	got, err := f.ProblemDetail(context.Background(), "two-sum")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got.TitleSlug != "two-sum" {
		t.Errorf("TitleSlug = %q, want two-sum", got.TitleSlug)
	}
}

func TestProblemDetail_UnseededReturnsError(t *testing.T) {
	f := &Fake{}
	_, err := f.ProblemDetail(context.Background(), "missing")
	if err == nil {
		t.Fatal("err = nil, want non-nil for unseeded slug")
	}
}

func TestProblemDetail_RecordsCalls(t *testing.T) {
	f := &Fake{Details: map[string]*leetcode.ProblemDetail{"a": {}, "b": {}}}
	_, _ = f.ProblemDetail(context.Background(), "a")
	_, _ = f.ProblemDetail(context.Background(), "b")
	if got, want := f.DetailCalls, []string{"a", "b"}; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("DetailCalls = %v, want %v", got, want)
	}
}

func TestHook_ShadowsSeededData(t *testing.T) {
	sentinel := errors.New("sentinel")
	f := &Fake{
		Details: map[string]*leetcode.ProblemDetail{"two-sum": {TitleSlug: "two-sum"}},
		DetailHook: func(_ context.Context, _ string) (*leetcode.ProblemDetail, error) {
			return nil, sentinel
		},
	}
	_, err := f.ProblemDetail(context.Background(), "two-sum")
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want sentinel — hook should shadow seeded data", err)
	}
}

func TestRun_DefaultsToSeededRunResult(t *testing.T) {
	canned := &leetcode.RunResult{StatusCode: 10}
	f := &Fake{RunResult: canned}
	got, err := f.InterpretSolution(context.Background(), "two-sum", "python3", "1", "code", "input", "")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != canned {
		t.Errorf("got = %v, want canned RunResult", got)
	}
}

// Compile-time assertion: *Fake satisfies the production *leetcode.Client
// surface for every method we expose. If a method signature drifts on
// either side, this fails to build.
var _ interface {
	MyFavoriteLists(context.Context) ([]leetcode.FavoriteList, error)
	FavoriteQuestionList(context.Context, string, int, int) (*leetcode.FavoriteQuestionListResult, error)
	ProblemDetail(context.Context, string) (*leetcode.ProblemDetail, error)
	SubmissionList(context.Context, string, string, int) ([]leetcode.Submission, string, error)
	UserProgress(context.Context, int, int) ([]leetcode.ProgressQuestion, int, error)
	InterpretSolution(context.Context, string, string, string, string, string, string) (*leetcode.RunResult, error)
	Submit(context.Context, string, string, string, string) (*leetcode.SubmitResult, error)
	UpdateSubmissionNote(context.Context, string, string, []int, string) error
} = (*Fake)(nil)
