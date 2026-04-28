package tui

import (
	"context"
	"testing"

	"leetcode-anki/internal/editor"
	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/leetcode/leetcodefake"
)

// In Review Mode, pressing 'e' must NOT open the canonical solution.<ext>
// (which contains the answer). Instead it scaffolds a fresh temp file
// seeded with the language snippet and opens that.
func TestEdit_InReviewMode_OpensTmpNotSolution(t *testing.T) {
	cache := newFakeCache()
	ed := newFakeEditor()
	canonicalPath := cache.writeSolution("two-sum", "golang", "func twoSum() { /* canonical answer */ }")

	m := onProblemScreen("two-sum", cache, ed, &leetcodefake.Fake{})
	m.reviewMode = true

	_, cmd := m.Update(keyEdit)
	if cmd == nil {
		t.Fatal("expected edit key to return a tea.Cmd")
	}
	if len(ed.openCalls) != 1 {
		t.Fatalf("Open called %d times, want 1: %v", len(ed.openCalls), ed.openCalls)
	}
	openedPath := ed.openCalls[0]
	if openedPath == canonicalPath {
		t.Errorf("Review Mode opened the canonical Solution path %q — must open a separate attempt", canonicalPath)
	}
	if m.problem.attemptPath != openedPath {
		t.Errorf("attemptPath = %q, want %q", m.problem.attemptPath, openedPath)
	}
	got, err := cache.Read(openedPath)
	if err != nil {
		t.Fatalf("Read(%q) failed: %v", openedPath, err)
	}
	if got != "package main\n" {
		t.Errorf("attempt seeded with %q, want the language snippet (%q) — never the canonical answer", got, "package main\n")
	}
}

// Pressing 'e' twice in Review Mode on the same Problem must open the same
// attempt path so the user can resume their in-progress attempt.
func TestEdit_InReviewMode_ResumesTmpWithinProblem(t *testing.T) {
	cache := newFakeCache()
	ed := newFakeEditor()
	m := onProblemScreen("two-sum", cache, ed, &leetcodefake.Fake{})
	m.reviewMode = true

	_, _ = m.Update(keyEdit)
	_, _ = m.Update(keyEdit)

	if len(ed.openCalls) != 2 {
		t.Fatalf("Open calls = %d, want 2", len(ed.openCalls))
	}
	if ed.openCalls[0] != ed.openCalls[1] {
		t.Errorf("attempt path changed between presses: %q vs %q", ed.openCalls[0], ed.openCalls[1])
	}
	if got := len(cache.attemptScaffoldCalls); got != 1 {
		t.Errorf("ScaffoldAttemptTmp called %d times, want 1 (idempotent within Problem)", got)
	}
}

// Regression: Explore Mode 'e' still opens the canonical Solution.
func TestEdit_InExploreMode_StillOpensSolution(t *testing.T) {
	cache := newFakeCache()
	ed := newFakeEditor()
	m := onProblemScreen("two-sum", cache, ed, &leetcodefake.Fake{})

	_, cmd := m.Update(keyEdit)
	if cmd == nil {
		t.Fatal("expected edit key to return a tea.Cmd")
	}
	wantPath := "/fake/two-sum/solution.golang"
	if len(ed.openCalls) != 1 || ed.openCalls[0] != wantPath {
		t.Errorf("Open calls = %v, want [%q]", ed.openCalls, wantPath)
	}
	if m.problem.attemptPath != "" {
		t.Errorf("attemptPath = %q in Explore Mode, want empty", m.problem.attemptPath)
	}
}

// Run in Review Mode reads the attempt file, NOT the canonical Solution.
func TestRun_InReviewMode_ReadsTmpFile(t *testing.T) {
	cache := newFakeCache()
	ed := newFakeEditor()
	cache.writeSolution("two-sum", "golang", "CANONICAL_BODY")

	gotCode := make(chan string, 1)
	fc := &leetcodefake.Fake{
		RunResult: &leetcode.RunResult{State: "SUCCESS"},
		RunHook: func(_ context.Context, _, _, _, code, _, _ string) (*leetcode.RunResult, error) {
			gotCode <- code
			return &leetcode.RunResult{State: "SUCCESS"}, nil
		},
	}
	m := onProblemScreen("two-sum", cache, ed, fc)
	m.reviewMode = true

	// Scaffold the attempt and overwrite its content with something
	// distinguishable from the canonical body.
	_, _ = m.Update(keyEdit)
	cache.writeAttempt(m.problem.attemptPath, "ATTEMPT_BODY")

	_, cmd := m.Update(keyRun)
	if cmd == nil {
		t.Fatal("expected run key to return a tea.Cmd")
	}
	drainBatch(cmd)

	select {
	case code := <-gotCode:
		if code != "ATTEMPT_BODY" {
			t.Errorf("Run sent code %q, want %q (canonical body must not leak into Run)", code, "ATTEMPT_BODY")
		}
	default:
		t.Fatal("Run hook never invoked")
	}
}

// Submit in Review Mode reads the attempt file, NOT the canonical Solution.
func TestSubmit_InReviewMode_ReadsTmpFile(t *testing.T) {
	cache := newFakeCache()
	ed := newFakeEditor()
	cache.writeSolution("two-sum", "golang", "CANONICAL_BODY")

	gotCode := make(chan string, 1)
	fc := &leetcodefake.Fake{
		SubmitResult: &leetcode.SubmitResult{State: "SUCCESS"},
		SubmitHook: func(_ context.Context, _, _, _, code string) (*leetcode.SubmitResult, error) {
			gotCode <- code
			return &leetcode.SubmitResult{State: "SUCCESS"}, nil
		},
	}
	m := onProblemScreen("two-sum", cache, ed, fc)
	m.reviewMode = true

	_, _ = m.Update(keyEdit)
	cache.writeAttempt(m.problem.attemptPath, "ATTEMPT_BODY")

	_, cmd := m.Update(keySubmit)
	if cmd == nil {
		t.Fatal("expected submit key to return a tea.Cmd")
	}
	drainBatch(cmd)

	select {
	case code := <-gotCode:
		if code != "ATTEMPT_BODY" {
			t.Errorf("Submit sent code %q, want %q", code, "ATTEMPT_BODY")
		}
	default:
		t.Fatal("Submit hook never invoked")
	}
}

// Editing the attempt in Review Mode must NOT mark the slug as having a
// canonical local Solution. The editor was opened on a temp attempt
// path, not on solution.<ext>; flipping hasSolution / solutionSlugs
// would mislead the lists screen and the in-progress glyph.
func TestEditorDoneMsg_InReviewMode_DoesNotMarkSolution(t *testing.T) {
	cache := newFakeCache()
	ed := newFakeEditor()
	m := onProblemScreen("two-sum", cache, ed, &leetcodefake.Fake{})
	m.reviewMode = true
	// No canonical Solution exists yet — solutionPath stays empty.
	if m.problem.solutionPath != "" {
		t.Fatalf("precondition: solutionPath should be empty, got %q", m.problem.solutionPath)
	}
	if m.problem.hasSolution {
		t.Fatal("precondition: hasSolution should be false")
	}

	_, _ = m.Update(keyEdit)
	attemptPath := m.problem.attemptPath
	if attemptPath == "" {
		t.Fatal("precondition: 'e' should have set attemptPath")
	}

	_, _ = m.Update(editor.EditorDoneMsg{Path: attemptPath})

	if m.problem.hasSolution {
		t.Error("hasSolution flipped to true after editing the attempt — only canonical edits should mark it")
	}
	if m.solutionSlugs["two-sum"] {
		t.Error("solutionSlugs[two-sum] = true after editing the attempt — only canonical edits should mark it")
	}
}

// Regression for the existing flow: an EditorDoneMsg whose Path matches
// the canonical solutionPath does mark hasSolution / solutionSlugs.
func TestEditorDoneMsg_OnSolutionPath_MarksSolution(t *testing.T) {
	cache := newFakeCache()
	ed := newFakeEditor()
	m := onProblemScreen("two-sum", cache, ed, &leetcodefake.Fake{})

	path := cache.writeSolution("two-sum", "golang", "package main\n")
	m.problem.solutionPath = path

	_, _ = m.Update(editor.EditorDoneMsg{Path: path})

	if !m.problem.hasSolution {
		t.Error("hasSolution = false after canonical edit, want true")
	}
	if !m.solutionSlugs["two-sum"] {
		t.Error("solutionSlugs[two-sum] = false after canonical edit, want true")
	}
}

// Pressing 'r' in Review Mode before any 'e' must error with the same
// "nothing to run" guard as Explore Mode — operating on the canonical
// Solution as a fallback would defeat the leak fix.
func TestRun_InReviewMode_NothingToRunWhenNoAttempt(t *testing.T) {
	cache := newFakeCache()
	ed := newFakeEditor()
	cache.writeSolution("two-sum", "golang", "CANONICAL_BODY")

	m := onProblemScreen("two-sum", cache, ed, &leetcodefake.Fake{})
	m.reviewMode = true

	_, cmd := m.Update(keyRun)
	if cmd != nil {
		t.Errorf("expected nil cmd when no attempt scaffolded, got %T", cmd)
	}
	if m.err == nil {
		t.Fatal("m.err = nil, want 'nothing to run' guard")
	}
	if m.load.Active() {
		t.Errorf("loading indicator should not be active when run is rejected")
	}
}
