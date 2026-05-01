package tui

import (
	"context"
	"strings"
	"sync"
	"testing"

	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/leetcode/leetcodefake"
)

const twoSumMeta = `{"name":"twoSum","params":[{"name":"nums","type":"integer[]"},{"name":"target","type":"integer"}],"return":{"type":"integer[]"}}`

// withCustomCases wires a Model on the problem screen for two-sum and seeds
// the fakeCases store with the given Customs. Returns the Model, the fake
// LeetCode client (so tests can install RunHook), and the fakeCases.
func withCustomCases(t *testing.T, customs []string) (*Model, *leetcodefake.Fake, *fakeCases) {
	t.Helper()
	cache := newFakeCache()
	ed := newFakeEditor()
	fc := &leetcodefake.Fake{}
	fcases := newFakeCases()
	for _, in := range customs {
		_ = fcases.Add("two-sum", in)
	}
	m := NewModel(context.Background(), fc, cache, ed, fcases, newFakeReviews())
	m.width, m.height = 140, 40
	m.currentProblem = problemDetailFor("two-sum")
	m.currentProblem.MetaData = twoSumMeta
	m.problem = newProblemView(cache, 100, 30)
	m.problem.chosenLang = "golang"
	m.screen = screenProblem
	m.solutionSlugs = map[string]bool{}
	m.problem.solutionPath = cache.writeSolution("two-sum", "golang", "package main\n")
	return m, fc, fcases
}

// captureRunArgs holds whatever dataInput / metaData / slug InterpretSolution
// was invoked with. RunHook returns canned RunResult once the args are recorded.
type captureRunArgs struct {
	mu        sync.Mutex
	called    bool
	slug      string
	dataInput string
	metaData  string
}

func (c *captureRunArgs) hook(canned *leetcode.RunResult) func(ctx context.Context, slug, lang, qid, code, in, meta string) (*leetcode.RunResult, error) {
	return func(_ context.Context, slug, _, _, _, in, meta string) (*leetcode.RunResult, error) {
		c.mu.Lock()
		c.called = true
		c.slug = slug
		c.dataInput = in
		c.metaData = meta
		c.mu.Unlock()
		return canned, nil
	}
}

// runCodeCmd must merge Examples with Customs using "\n" so the next Run
// reproduces the user-promoted failing input alongside the LeetCode-supplied
// Examples.
func TestRunCodeCmd_MergesExamplesAndCustoms(t *testing.T) {
	m, fc, fcases := withCustomCases(t, []string{"[4,5]\n9"})

	cap := &captureRunArgs{}
	fc.RunHook = cap.hook(&leetcode.RunResult{CorrectAnswer: true})

	_, cmd := m.Update(keyRun)
	if cmd == nil {
		t.Fatal("expected run key to schedule a tea.Cmd")
	}
	drainBatch(cmd)

	if !cap.called {
		t.Fatal("RunHook was never invoked")
	}
	if cap.slug != "two-sum" {
		t.Errorf("slug = %q, want two-sum", cap.slug)
	}
	want := "[2,7,11,15]\n9\n[4,5]\n9"
	if cap.dataInput != want {
		t.Errorf("dataInput = %q, want %q", cap.dataInput, want)
	}
	if got := fcases.listCalls; len(got) != 1 || got[0] != "two-sum" {
		t.Errorf("Cases.List calls = %v, want [two-sum]", got)
	}
}

// With no Customs registered, dataInput must equal the Problem's Examples
// exactly — no spurious trailing newline that would shift case counts.
func TestRunCodeCmd_NoCustoms_PassesExamplesUnmodified(t *testing.T) {
	m, fc, _ := withCustomCases(t, nil)
	cap := &captureRunArgs{}
	fc.RunHook = cap.hook(&leetcode.RunResult{CorrectAnswer: true})

	_, cmd := m.Update(keyRun)
	drainBatch(cmd)

	if cap.dataInput != "[2,7,11,15]\n9" {
		t.Errorf("dataInput = %q, want %q (no trailing newline, no merge)", cap.dataInput, "[2,7,11,15]\n9")
	}
}

// runResultMsg must carry the Problem's Example count (1 for two-sum's
// fixture) so the result screen can mark Custom cases with the star glyph.
func TestRunCodeCmd_PopulatesExampleCount(t *testing.T) {
	m, fc, _ := withCustomCases(t, []string{"[4,5]\n9"})
	fc.RunResult = &leetcode.RunResult{CorrectAnswer: true, Cases: []leetcode.RunCase{
		{Index: 0, Input: "[2,7,11,15]\n9", Output: "[0,1]", Expected: "[0,1]", Pass: true},
		{Index: 1, Input: "[4,5]\n9", Output: "[]", Expected: "[]", Pass: true},
	}}

	_, cmd := m.Update(keyRun)
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	msg, ok := extractMsg[runResultMsg](cmd)
	if !ok {
		t.Fatal("expected runResultMsg in dispatch batch")
	}
	if msg.exampleCount != 1 {
		t.Errorf("exampleCount = %d, want 1", msg.exampleCount)
	}
}

// On a Run with 2 Examples + 1 Custom, the rendered grid must show the
// star glyph only on case 3 (the Custom one).
func TestRenderCaseGrid_StarsOnlyCustoms(t *testing.T) {
	cases := []leetcode.RunCase{
		{Index: 0, Input: "ex1", Output: "a", Expected: "a", Pass: true},
		{Index: 1, Input: "ex2", Output: "b", Expected: "b", Pass: true},
		{Index: 2, Input: "cu1", Output: "c", Expected: "c", Pass: true},
	}
	got := renderCaseGrid(cases, 2, 50)

	// The grid contains lines with "case 1", "case 2", "case 3". Each line
	// should carry the star only when the case is at index >= exampleCount.
	for _, line := range strings.Split(got, "\n") {
		switch {
		case strings.Contains(line, "case 1") || strings.Contains(line, "case 2"):
			if strings.Contains(line, "★") {
				t.Errorf("Example case has star glyph: %q", line)
			}
		case strings.Contains(line, "case 3"):
			if !strings.Contains(line, "★") {
				t.Errorf("Custom case missing star glyph: %q", line)
			}
		}
	}
}

// Footer legend "★  custom" surfaces only when the Run carries at least
// one Custom case so the indicator never lies about what's on screen.
func TestRunResultFooter_CustomLegend(t *testing.T) {
	cache := newFakeCache()
	fc := &leetcodefake.Fake{}
	m := NewModel(context.Background(), fc, cache, newFakeEditor(), newFakeCases(), newFakeReviews())
	m.width, m.height = 140, 40
	m.currentProblem = problemDetailFor("two-sum")
	m.result = resultView{
		kind:         resultRun,
		exampleCount: 2,
		run: &leetcode.RunResult{
			CorrectAnswer: true,
			Cases: []leetcode.RunCase{
				{Index: 0, Pass: true},
				{Index: 1, Pass: true},
				{Index: 2, Pass: true},
			},
		},
	}
	m.screen = screenResult

	if !strings.Contains(viewResultView(m), "custom") {
		t.Errorf("expected '★ custom' legend in footer when Customs present:\n%s", viewResultView(m))
	}

	// And: no Customs -> no legend.
	m.result.exampleCount = 3
	if strings.Contains(viewResultView(m), "custom") {
		t.Errorf("legend should not appear when no Customs are present:\n%s", viewResultView(m))
	}
}
