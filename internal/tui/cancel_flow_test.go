package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/leetcode/leetcodefake"
)

// blockingFake builds a leetcodefake.Fake whose InterpretSolution and Submit
// hooks block until ctx is cancelled. Returns the fake plus the channels
// the test uses to coordinate: started fires once the call is in flight,
// done fires once it returns, and *err captures the ctx error it observed.
func blockingFake() (fake *leetcodefake.Fake, started, done chan struct{}, err *error) {
	started = make(chan struct{})
	done = make(chan struct{})
	err = new(error)
	fake = &leetcodefake.Fake{}
	fake.RunHook = func(ctx context.Context, _, _, _, _, _, _ string) (*leetcode.RunResult, error) {
		close(started)
		<-ctx.Done()
		*err = ctx.Err()
		close(done)
		return nil, ctx.Err()
	}
	fake.SubmitHook = func(ctx context.Context, _, _, _, _ string) (*leetcode.SubmitResult, error) {
		close(started)
		<-ctx.Done()
		*err = ctx.Err()
		close(done)
		return nil, ctx.Err()
	}
	return fake, started, done, err
}

// keyEsc is the synthetic esc key event.
var keyEsc = tea.KeyMsg{Type: tea.KeyEsc}

// keySubmit is bound to 's'.
var keySubmit = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}

// Pressing esc while a Run is in flight must cancel the request's context
// so the HTTP goroutine returns instead of clobbering Model state with a
// stale runResultMsg later.
func TestEsc_CancelsInflightRun(t *testing.T) {
	cache := newFakeCache()
	bc, started, done, observedErr := blockingFake()
	m := onProblemScreen("two-sum", cache, newFakeEditor(), &leetcodefake.Fake{})
	m.client = bc
	m.problem.solutionPath = cache.writeSolution("two-sum", "golang", "package main\n")

	// Press 'r' to start the run; the returned cmd is what tea would
	// invoke in production. Drive it manually in a goroutine so the
	// fake client can observe its ctx.
	_, cmd := m.Update(keyRun)
	if cmd == nil {
		t.Fatal("expected run key to schedule a tea.Cmd")
	}
	if !m.load.Active() || m.load.kind != KindRun {
		t.Fatalf("expected run indicator active after pressing r, got active=%v kind=%v", m.load.Active(), m.load.kind)
	}
	if m.cancelInflight == nil {
		t.Fatal("cancelInflight was not stored on the model")
	}

	wg, _ := startBatch(cmd)

	// Wait for the request to actually be in-flight before pressing esc;
	// otherwise we'd be racing the cancellation against the goroutine
	// scheduling and could spuriously assert before the request started.
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("InterpretSolution was never invoked")
	}

	// Esc during a run-in-flight must cancel.
	_, escCmd := m.Update(keyEsc)
	if escCmd != nil {
		t.Errorf("esc-during-run must not schedule a follow-up cmd; got %T", escCmd)
	}

	// The blocked InterpretSolution must observe ctx.Canceled and return.
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("InterpretSolution did not return after esc; cancel didn't propagate")
	}
	if !errors.Is(*observedErr, context.Canceled) {
		t.Errorf("InterpretSolution saw %v, want context.Canceled", *observedErr)
	}

	wg.Wait()
}

// Same flow for Submit — the cancel wiring is duplicated between
// runCodeCmd and submitCodeCmd, so a regression in one wouldn't be
// caught by testing the other.
func TestEsc_CancelsInflightSubmit(t *testing.T) {
	cache := newFakeCache()
	bc, started, done, observedErr := blockingFake()
	m := onProblemScreen("two-sum", cache, newFakeEditor(), &leetcodefake.Fake{})
	m.client = bc
	m.problem.solutionPath = cache.writeSolution("two-sum", "golang", "package main\n")

	_, cmd := m.Update(keySubmit)
	if cmd == nil {
		t.Fatal("expected submit key to schedule a tea.Cmd")
	}
	if !m.load.Active() || m.load.kind != KindSubmit {
		t.Fatalf("expected submit indicator active after pressing s, got active=%v kind=%v", m.load.Active(), m.load.kind)
	}

	wg, _ := startBatch(cmd)

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("Submit was never invoked")
	}

	_, _ = m.Update(keyEsc)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Submit did not return after esc")
	}
	if !errors.Is(*observedErr, context.Canceled) {
		t.Errorf("Submit saw %v, want context.Canceled", *observedErr)
	}
	wg.Wait()
}
