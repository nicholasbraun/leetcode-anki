package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
)

func TestIndicator_IdleByDefault(t *testing.T) {
	i := NewIndicator()
	if i.Active() {
		t.Fatal("new indicator should be idle")
	}
	if got := i.Elapsed(); got != 0 {
		t.Fatalf("idle elapsed = %v, want 0", got)
	}
	if got := i.View(); got != "" {
		t.Fatalf("idle View() = %q, want empty", got)
	}
	if got := i.Inline(); got != "" {
		t.Fatalf("idle Inline() = %q, want empty", got)
	}
}

func TestIndicator_StartActivates(t *testing.T) {
	i := NewIndicator()
	cmd := i.Start(KindNeutral, "loading lists")
	if !i.Active() {
		t.Fatal("Start should activate the indicator")
	}
	if cmd == nil {
		t.Fatal("Start should return a non-nil tick cmd")
	}
	if got := i.Inline(); !strings.Contains(got, "loading lists") {
		t.Fatalf("Inline()=%q, want it to contain label", got)
	}
}

func TestIndicator_StopDeactivates(t *testing.T) {
	i := NewIndicator()
	i.Start(KindRun, "running")
	i.Stop()
	if i.Active() {
		t.Fatal("Stop should deactivate")
	}
	if got := i.Elapsed(); got != 0 {
		t.Fatalf("Stop should zero Elapsed, got %v", got)
	}
	if got := i.View(); got != "" {
		t.Fatalf("View after Stop = %q, want empty", got)
	}
}

func TestIndicator_StartWhileActiveRelabels(t *testing.T) {
	i := NewIndicator()
	i.Start(KindNeutral, "loading lists")
	cmd := i.Start(KindNeutral, "loading problems")
	if cmd != nil {
		t.Fatal("Start while active should return nil cmd (already ticking)")
	}
	if got := i.Inline(); !strings.Contains(got, "loading problems") {
		t.Fatalf("Inline()=%q, want updated label", got)
	}
}

func TestIndicator_RunIncludesElapsedClock(t *testing.T) {
	i := NewIndicator()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	i.now = func() time.Time { return base }
	i.Start(KindRun, "running")
	i.now = func() time.Time { return base.Add(4 * time.Second) }

	if got := i.Inline(); !strings.Contains(got, "0:04") {
		t.Fatalf("Inline()=%q, want it to contain 0:04 elapsed", got)
	}
}

func TestIndicator_SubmitIncludesElapsedClock(t *testing.T) {
	i := NewIndicator()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	i.now = func() time.Time { return base }
	i.Start(KindSubmit, "submitting")
	i.now = func() time.Time { return base.Add(83 * time.Second) }

	if got := i.Inline(); !strings.Contains(got, "1:23") {
		t.Fatalf("Inline()=%q, want it to contain 1:23 elapsed", got)
	}
}

func TestIndicator_NeutralOmitsElapsedClock(t *testing.T) {
	i := NewIndicator()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	i.now = func() time.Time { return base }
	i.Start(KindNeutral, "loading lists")
	i.now = func() time.Time { return base.Add(5 * time.Second) }

	// KindNeutral is for cold-load full-screen takeovers — the wait isn't
	// actionable, so we don't want a ticking clock there.
	if got := i.Inline(); strings.Contains(got, "0:05") {
		t.Fatalf("Inline()=%q, neutral kind should not show elapsed clock", got)
	}
}

func TestIndicator_UpdateRoutesOwnTickMsg(t *testing.T) {
	i := NewIndicator()
	i.Start(KindRun, "running")

	tick := i.sp.Tick().(spinner.TickMsg)
	handled, cmd := i.Update(tick)
	if !handled {
		t.Fatal("own TickMsg should be handled")
	}
	if cmd == nil {
		t.Fatal("active indicator should re-tick on its own TickMsg")
	}
}

func TestIndicator_UpdateIgnoresForeignTickMsg(t *testing.T) {
	a := NewIndicator()
	b := NewIndicator()
	a.Start(KindRun, "a")
	b.Start(KindSubmit, "b")

	bTick := b.sp.Tick().(spinner.TickMsg)
	handled, _ := a.Update(bTick)
	if handled {
		t.Fatal("indicator A should not consume B's TickMsg")
	}
}

func TestIndicator_UpdateIgnoresUnrelatedMsg(t *testing.T) {
	i := NewIndicator()
	i.Start(KindRun, "running")
	handled, cmd := i.Update("some other msg")
	if handled || cmd != nil {
		t.Fatalf("non-tick msg should not be handled, got handled=%v cmd!=nil=%v", handled, cmd != nil)
	}
}

func TestIndicator_StaleTickAfterStopHaltsLoop(t *testing.T) {
	// After Stop(), a pending TickMsg from the previous run can still arrive.
	// The indicator must swallow it WITHOUT re-ticking, so the animation
	// loop terminates instead of running forever in the background.
	i := NewIndicator()
	i.Start(KindRun, "running")
	tick := i.sp.Tick().(spinner.TickMsg)
	i.Stop()

	handled, cmd := i.Update(tick)
	if !handled {
		t.Fatal("stale tick for our own ID should still be marked handled")
	}
	if cmd != nil {
		t.Fatal("stale tick after Stop must not re-arm the tick loop")
	}
}

func TestFormatElapsed(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "0:00"},
		{4 * time.Second, "0:04"},
		{59 * time.Second, "0:59"},
		{60 * time.Second, "1:00"},
		{83 * time.Second, "1:23"},
		{605 * time.Second, "10:05"},
	}
	for _, c := range cases {
		if got := formatElapsed(c.in); got != c.want {
			t.Errorf("formatElapsed(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
