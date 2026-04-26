package tui

import (
	"context"

	"leetcode-anki/internal/leetcode"
)

// LeetcodeClient is the subset of *leetcode.Client the TUI depends on.
// Screens take this interface (not the concrete client) so tests can drive
// the model with canned responses without a live LeetCode session.
type LeetcodeClient interface {
	MyFavoriteLists(ctx context.Context) ([]leetcode.FavoriteList, error)
	FavoriteQuestionList(ctx context.Context, slug string, skip, limit int) (*leetcode.FavoriteQuestionListResult, error)
	Question(ctx context.Context, titleSlug string) (*leetcode.ProblemDetail, error)
	InterpretSolution(ctx context.Context, slug, lang, questionID, code, dataInput string) (*leetcode.RunResult, error)
	Submit(ctx context.Context, slug, lang, questionID, code string) (*leetcode.SubmitResult, error)
}
