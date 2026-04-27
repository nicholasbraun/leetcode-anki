package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"leetcode-anki/internal/editor"
	"leetcode-anki/internal/leetcode"
)

// Problem is the TUI-side name for a Problem-list row. The underlying
// wire-format type is leetcode.Question (CONTEXT.md keeps "Question"
// confined to the LeetCode-client boundary); the alias lets TUI source
// read in the canonical domain vocabulary without an extra translation.
type Problem = leetcode.Question

// LeetcodeClient is the subset of *leetcode.Client the TUI depends on.
// Screens take this interface (not the concrete client) so tests can drive
// the model with canned responses without a live LeetCode session.
type LeetcodeClient interface {
	MyFavoriteLists(ctx context.Context) ([]leetcode.FavoriteList, error)
	FavoriteQuestionList(ctx context.Context, slug string, skip, limit int) (*leetcode.FavoriteQuestionListResult, error)
	ProblemDetail(ctx context.Context, titleSlug string) (*leetcode.ProblemDetail, error)
	InterpretSolution(ctx context.Context, slug, lang, questionID, code, dataInput string) (*leetcode.RunResult, error)
	Submit(ctx context.Context, slug, lang, questionID, code string) (*leetcode.SubmitResult, error)
}

// SolutionCache is the on-disk store for scaffolded solution files. The TUI
// depends on this interface (not editor's package-level functions) so the
// edit flow can be exercised in tests without writing to a real cache.
type SolutionCache interface {
	Scaffold(titleSlug, langSlug, snippet string) (string, error)
	Read(path string) (string, error)
	ExistingPath(titleSlug, langSlug string) string
	SlugsWith() (map[string]bool, error)
	HasAny(titleSlug string) bool
}

// Editor spawns the user's editor on a solution file. Wraps the
// tea.ExecProcess dance so tests can stub Open and emit a canned
// editor.EditorDoneMsg without forking a real editor.
type Editor interface {
	Open(path string) tea.Cmd
}

var (
	_ LeetcodeClient = (*leetcode.Client)(nil)
	_ SolutionCache  = (*editor.Cache)(nil)
	_ Editor         = (*editor.Runner)(nil)
)
