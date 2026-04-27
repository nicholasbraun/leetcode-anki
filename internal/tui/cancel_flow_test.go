package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"leetcode-anki/internal/leetcode"
)

// blockingClient wraps fakeClient: InterpretSolution and Submit block on
// the request context until it's cancelled. Lets the cancel-flow tests
// observe what the in-flight goroutine sees when esc/ctrl+c fires.
type blockingClient struct {
	fakeClient
	started chan struct{}
	done    chan struct{}
	err     error
}

func newBlockingClient() *blockingClient {
	return &blockingClient{
		started: make(chan struct{}),
		done:    make(chan struct{}),
	}
}

func (b *blockingClient) InterpretSolution(ctx context.Context, slug, lang, qid, code, in string) (*leetcode.RunResult, error) {
	close(b.started)
	<-ctx.Done()
	b.err = ctx.Err()
	close(b.done)
	return nil, ctx.Err()
}

func (b *blockingClient) Submit(ctx context.Context, slug, lang, qid, code string) (*leetcode.SubmitResult, error) {
	close(b.started)
	<-ctx.Done()
	b.err = ctx.Err()
	close(b.done)
	return nil, ctx.Err()
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
	bc := newBlockingClient()
	m := onProblemScreen("two-sum", cache, newFakeEditor(), &fakeClient{})
	m.client = bc
	m.problem.solutionPath = cache.writeSolution("two-sum", "golang", "package main\n")

	// Press 'r' to start the run; the returned cmd is what tea would
	// invoke in production. Drive it manually in a goroutine so the
	// fake client can observe its ctx.
	_, cmd := m.Update(keyRun)
	if cmd == nil {
		t.Fatal("expected run key to schedule a tea.Cmd")
	}
	if !m.runLoading {
		t.Fatal("runLoading = false after pressing r")
	}
	if m.cancelInflight == nil {
		t.Fatal("cancelInflight was not stored on the model")
	}

	cmdDone := make(chan tea.Msg, 1)
	go func() { cmdDone <- cmd() }()

	// Wait for the request to actually be in-flight before pressing esc;
	// otherwise we'd be racing the cancellation against the goroutine
	// scheduling and could spuriously assert before the request started.
	select {
	case <-bc.started:
	case <-time.After(time.Second):
		t.Fatal("InterpretSolution was never invoked")
	}

	// Esc during runLoading must cancel.
	_, escCmd := m.Update(keyEsc)
	if escCmd != nil {
		t.Errorf("esc-during-run must not schedule a follow-up cmd; got %T", escCmd)
	}

	// The blocked InterpretSolution must observe ctx.Canceled and return.
	select {
	case <-bc.done:
	case <-time.After(time.Second):
		t.Fatal("InterpretSolution did not return after esc; cancel didn't propagate")
	}
	if !errors.Is(bc.err, context.Canceled) {
		t.Errorf("InterpretSolution saw %v, want context.Canceled", bc.err)
	}

	// Drain the cmd's tea.Msg so the goroutine doesn't leak.
	<-cmdDone
}

// Same flow for Submit — the cancel wiring is duplicated between
// runCodeCmd and submitCodeCmd, so a regression in one wouldn't be
// caught by testing the other.
func TestEsc_CancelsInflightSubmit(t *testing.T) {
	cache := newFakeCache()
	bc := newBlockingClient()
	m := onProblemScreen("two-sum", cache, newFakeEditor(), &fakeClient{})
	m.client = bc
	m.problem.solutionPath = cache.writeSolution("two-sum", "golang", "package main\n")

	_, cmd := m.Update(keySubmit)
	if cmd == nil {
		t.Fatal("expected submit key to schedule a tea.Cmd")
	}
	if !m.submitLoading {
		t.Fatal("submitLoading = false after pressing s")
	}

	cmdDone := make(chan tea.Msg, 1)
	go func() { cmdDone <- cmd() }()

	select {
	case <-bc.started:
	case <-time.After(time.Second):
		t.Fatal("Submit was never invoked")
	}

	_, _ = m.Update(keyEsc)

	select {
	case <-bc.done:
	case <-time.After(time.Second):
		t.Fatal("Submit did not return after esc")
	}
	if !errors.Is(bc.err, context.Canceled) {
		t.Errorf("Submit saw %v, want context.Canceled", bc.err)
	}
	<-cmdDone
}
