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
	m := NewModel(context.Background(), fc, cache, ed, newFakeCases(), newFakeReviews())
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
	if got.Slug != "two-sum" || got.Lang != "golang" {
		t.Errorf("Scaffold slug/lang = %+v, want {two-sum, golang, ...}", got)
	}
	if !strings.HasPrefix(got.Snippet, "package main\n") {
		t.Errorf("Scaffold snippet should start with starter code; got %q", got.Snippet)
	}
	if !strings.Contains(got.Snippet, "// Find two numbers that sum to target.") {
		t.Errorf("Scaffold snippet should include commented description line; got %q", got.Snippet)
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

// Regression for paid problems and any other cases where ProblemDetail.Content
// is empty: the scaffolded file must match the bare starter snippet exactly,
// no trailing blank lines or stray comment markers.
func TestEditFlow_EmptyDescription_ScaffoldsBareSnippet(t *testing.T) {
	cache := newFakeCache()
	ed := newFakeEditor()
	m := onProblemScreen("two-sum", cache, ed, &leetcodefake.Fake{})
	m.currentProblem.Content = ""

	_, _ = m.Update(keyEdit)

	if len(cache.scaffoldCalls) != 1 {
		t.Fatalf("Scaffold called %d times, want 1", len(cache.scaffoldCalls))
	}
	if got := cache.scaffoldCalls[0].Snippet; got != "package main\n" {
		t.Errorf("Scaffold snippet = %q, want bare starter snippet %q", got, "package main\n")
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

	// Mirror the production Edit flow: Scaffold runs and sets solutionPath
	// before the editor is invoked. The scaffolded file survives even if
	// $EDITOR exits non-zero, so hasSolution should still be marked.
	path := cache.writeSolution("two-sum", "golang", "package main\n")
	m.problem.solutionPath = path

	editorErr := errors.New("editor exited 1")
	_, _ = m.Update(editor.EditorDoneMsg{Path: path, Err: editorErr})

	if m.err == nil {
		t.Fatal("m.err = nil, want editor error to surface")
	}
	if !strings.Contains(m.err.Error(), "editor exited 1") {
		t.Errorf("m.err = %v, want it to contain editor's error", m.err)
	}
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
