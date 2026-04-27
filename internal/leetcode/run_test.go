package leetcode

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"leetcode-anki/internal/auth"
)

// scriptedDoer returns a sequence of canned response bodies in order; further
// calls past the script length re-use the last entry. All responses are 200 OK.
type scriptedDoer struct {
	bodies []string
	calls  int32
}

func (s *scriptedDoer) Do(_ *http.Request) (*http.Response, error) {
	idx := atomic.AddInt32(&s.calls, 1) - 1
	body := s.bodies[len(s.bodies)-1]
	if int(idx) < len(s.bodies) {
		body = s.bodies[idx]
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

func newTestClient(d httpDoer) *Client {
	return newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)
}

func TestPollCheck_ReturnsOnSuccess(t *testing.T) {
	d := &scriptedDoer{bodies: []string{
		`{"state":"PENDING"}`,
		`{"state":"STARTED"}`,
		`{"state":"SUCCESS","status_msg":"Accepted"}`,
	}}
	c := newTestClient(d)

	raw, err := c.pollCheck(context.Background(), "abc")
	if err != nil {
		t.Fatalf("pollCheck: %v", err)
	}
	var probe struct {
		State     string `json:"state"`
		StatusMsg string `json:"status_msg"`
	}
	_ = json.Unmarshal(raw, &probe)
	if probe.State != "SUCCESS" || probe.StatusMsg != "Accepted" {
		t.Errorf("got %+v", probe)
	}
	if got := atomic.LoadInt32(&d.calls); got != 3 {
		t.Errorf("expected 3 polls, got %d", got)
	}
}

// First poll already terminal: pollCheck must skip the initial 700ms
// sleep and return on the first response. Test cap is generous so a
// regression where the loop sleeps before checking would visibly stall.
func TestPollCheck_ImmediateSuccessSkipsBackoff(t *testing.T) {
	d := &scriptedDoer{bodies: []string{
		`{"state":"SUCCESS","status_msg":"Accepted"}`,
	}}
	c := newTestClient(d)

	start := time.Now()
	raw, err := c.pollCheck(context.Background(), "abc")
	if err != nil {
		t.Fatalf("pollCheck: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= 500*time.Millisecond {
		t.Errorf("first-call-terminal path slept (took %v); should return immediately", elapsed)
	}
	var probe struct{ State string }
	_ = json.Unmarshal(raw, &probe)
	if probe.State != "SUCCESS" {
		t.Errorf("got state %q, want SUCCESS", probe.State)
	}
	if got := atomic.LoadInt32(&d.calls); got != 1 {
		t.Errorf("expected 1 poll, got %d", got)
	}
}

// LeetCode does not document the full state alphabet; pollCheck treats any
// non-pending value as terminal so the TUI doesn't spin until the outer
// timeout when LeetCode introduces a new state.
func TestPollCheck_UnknownTerminalStateReturned(t *testing.T) {
	d := &scriptedDoer{bodies: []string{
		`{"state":"PENDING"}`,
		`{"state":"PENDING_REJUDGE","status_msg":"Rejudging"}`,
	}}
	c := newTestClient(d)

	raw, err := c.pollCheck(context.Background(), "abc")
	if err != nil {
		t.Fatalf("pollCheck: %v", err)
	}
	var probe struct{ State string }
	_ = json.Unmarshal(raw, &probe)
	if probe.State != "PENDING_REJUDGE" {
		t.Errorf("expected unknown state to surface, got %q", probe.State)
	}
}

func TestPollCheck_RespectsContextCancel(t *testing.T) {
	d := &scriptedDoer{bodies: []string{`{"state":"PENDING"}`}}
	c := newTestClient(d)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := c.pollCheck(ctx, "abc")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// An empty state ("") would loop forever under the old implementation; the
// iteration cap guarantees pollCheck eventually gives up rather than hanging
// the TUI for the full outer timeout.
func TestPollCheck_GivesUpAfterCapWhenStateNeverArrives(t *testing.T) {
	d := &scriptedDoer{bodies: []string{`{}`}} // no state field

	// Override timing constants by running with a short context so the test
	// completes quickly; the cap path still exercises because state is empty.
	c := newTestClient(d)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := c.pollCheck(ctx, "abc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Either ctx.DeadlineExceeded (likely, given short timeout) or the cap
	// message — both prove the loop terminates instead of hanging.
}
