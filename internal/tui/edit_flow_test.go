package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"leetcode-anki/internal/editor"
	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/leetcode/leetcodefake"
)

// keyEdit is the synthetic key event that updateProblemView interprets as
// "edit" — see keys.Edit which binds rune 'e'.
var keyEdit = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}

// keyRun is bound to 'r'.
var keyRun = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}

func problemDetailFor(slug string) *leetcode.ProblemDetail {
	return &leetcode.ProblemDetail{
		QuestionID:         "1",
		QuestionFrontendID: "1",
		Title:              "Two Sum",
		TitleSlug:          slug,
		Difficulty:         "Easy",
		Content:            "<p>Find two numbers that sum to target.</p>",
		ExampleTestcases:   "[2,7,11,15]\n9",
		CodeSnippets: []leetcode.CodeSnippet{
			{Lang: "Go", LangSlug: "golang", Code: "package main\n"},
		},
	}
}

// onProblemScreen builds a Model parked on the problem detail screen for
// the given slug, ready to receive Edit/Run/Submit key events.
func onProblemScreen(slug string, cache SolutionCache, ed Editor, fc *leetcodefake.Fake) *Model {
	m := NewModel(context.Background(), fc, cache, ed, newFakeReviews())
	m.width, m.height = 140, 40
	m.currentProblem = problemDetailFor(slug)
	m.problem = newProblemView(cache, 100, 30)
	m.problem.chosenLang = "golang"
	m.screen = screenProblem
	m.solutionSlugs = map[string]bool{}
	return m
}

func TestEditFlow_ScaffoldsAndOpens(t *testing.T) {
	cache := newFakeCache()
	ed := newFakeEditor()
	m := onProblemScreen("two-sum", cache, ed, &leetcodefake.Fake{})

	_, cmd := m.Update(keyEdit)
	if cmd == nil {
		t.Fatal("expected edit key to return a tea.Cmd")
	}

	if len(cache.scaffoldCalls) != 1 {
		t.Fatalf("Scaffold called %d times, want 1: %v", len(cache.scaffoldCalls), cache.scaffoldCalls)
	}
	got := cache.scaffoldCalls[0]
	if got.Slug != "two-sum" || got.Lang != "golang" || got.Snippet != "package main\n" {
		t.Errorf("Scaffold args = %+v, want {two-sum, golang, package main\\n}", got)
	}

	if len(ed.openCalls) != 1 {
		t.Fatalf("Open called %d times, want 1: %v", len(ed.openCalls), ed.openCalls)
	}
	wantPath := "/fake/two-sum/solution.golang"
	if ed.openCalls[0] != wantPath {
		t.Errorf("Open path = %q, want %q", ed.openCalls[0], wantPath)
	}
	if m.problem.solutionPath != wantPath {
		t.Errorf("solutionPath = %q, want %q", m.problem.solutionPath, wantPath)
	}
}

func TestEditorDoneMsg_MarksSolution(t *testing.T) {
	cache := newFakeCache()
	ed := newFakeEditor()
	m := onProblemScreen("two-sum", cache, ed, &leetcodefake.Fake{})

	// Seed the problems list so the row-glyph sync path runs.
	loadFakeProblems(t, m, []Problem{
		{QuestionFrontendID: "1", Title: "Two Sum", TitleSlug: "two-sum"},
	})
	// loadFakeProblems flips m.screen to screenProblems; restore the detail screen.
	m.screen = screenProblem

	path := cache.writeSolution("two-sum", "golang", "package main\n")
	m.problem.solutionPath = path

	_, _ = m.Update(editor.EditorDoneMsg{Path: path})

	if !m.problem.hasSolution {
		t.Error("problemView.hasSolution = false, want true after EditorDoneMsg")
	}
	if !m.solutionSlugs["two-sum"] {
		t.Errorf("solutionSlugs[two-sum] = false, want true")
	}
	pi, ok := m.problems.Items()[0].(problemItem)
	if !ok {
		t.Fatal("item 0 not a problemItem")
	}
	if !pi.hasSolution {
		t.Error("problems list row hasSolution = false, want true")
	}
	if m.err != nil {
		t.Errorf("m.err = %v, want nil on successful edit", m.err)
	}
}

func TestEditorDoneMsg_WithError(t *testing.T) {
	cache := newFakeCache()
	ed := newFakeEditor()
	m := onProblemScreen("two-sum", cache, ed, &leetcodefake.Fake{})

	editorErr := errors.New("editor exited 1")
	_, _ = m.Update(editor.EditorDoneMsg{Path: "/fake/two-sum/solution.golang", Err: editorErr})

	if m.err == nil {
		t.Fatal("m.err = nil, want editor error to surface")
	}
	if !strings.Contains(m.err.Error(), "editor exited 1") {
		t.Errorf("m.err = %v, want it to contain editor's error", m.err)
	}
	// Scaffold writes the file before Open is invoked, so the Solution
	// survives an editor crash. hasSolution should still be set.
	if !m.problem.hasSolution {
		t.Error("hasSolution = false on EditorDoneMsg with error; the scaffolded file still exists on disk")
	}
}

func TestRun_ReadsLatestSolution(t *testing.T) {
	cache := newFakeCache()
	ed := newFakeEditor()
	fc := &leetcodefake.Fake{}
	m := onProblemScreen("two-sum", cache, ed, fc)

	path := cache.writeSolution("two-sum", "golang", "package main\nfunc twoSum() {}\n")
	m.problem.solutionPath = path

	_, cmd := m.Update(keyRun)
	if cmd == nil {
		t.Fatal("expected run key to return a tea.Cmd")
	}
	if !m.load.Active() || m.load.kind != KindRun {
		t.Errorf("expected run indicator active after pressing run, got active=%v kind=%v", m.load.Active(), m.load.kind)
	}

	// Drain the cmd; the work leaf reads the solution then calls
	// InterpretSolution. Tick cmds emitted by the indicator return immediately.
	drainBatch(cmd)

	if len(cache.readCalls) != 1 || cache.readCalls[0] != path {
		t.Errorf("Read calls = %v, want [%q]", cache.readCalls, path)
	}
}
