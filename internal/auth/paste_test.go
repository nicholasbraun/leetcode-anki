package auth

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoginFromPaste_BothCookies(t *testing.T) {
	in := strings.NewReader("session-value\ncsrf-value\n")
	var out bytes.Buffer

	got, err := LoginFromPaste(in, &out)
	if err != nil {
		t.Fatalf("LoginFromPaste: %v", err)
	}
	if got.Session != "session-value" || got.CSRF != "csrf-value" {
		t.Errorf("got %+v, want session=session-value csrf=csrf-value", got)
	}
}

func TestLoginFromPaste_TrimsWhitespace(t *testing.T) {
	in := strings.NewReader("  session-value  \n\tcsrf-value\t\n")
	var out bytes.Buffer

	got, err := LoginFromPaste(in, &out)
	if err != nil {
		t.Fatalf("LoginFromPaste: %v", err)
	}
	if got.Session != "session-value" || got.CSRF != "csrf-value" {
		t.Errorf("got %+v, want trimmed values", got)
	}
}

func TestLoginFromPaste_RejectsEmptySession(t *testing.T) {
	in := strings.NewReader("\ncsrf-value\n")
	var out bytes.Buffer

	if _, err := LoginFromPaste(in, &out); err == nil {
		t.Error("expected error for empty session; got nil")
	}
}

func TestLoginFromPaste_RejectsEmptyCSRF(t *testing.T) {
	in := strings.NewReader("session-value\n\n")
	var out bytes.Buffer

	if _, err := LoginFromPaste(in, &out); err == nil {
		t.Error("expected error for empty csrf; got nil")
	}
}

func TestLoginFromPaste_RejectsTruncatedInput(t *testing.T) {
	in := strings.NewReader("session-value\n") // EOF before csrf
	var out bytes.Buffer

	if _, err := LoginFromPaste(in, &out); err == nil {
		t.Error("expected error when stdin closes before csrf line; got nil")
	}
}

// Pasting a full Cookie header line is a likely user error (they copy
// the whole "LEETCODE_SESSION=…; csrftoken=…" string from devtools'
// Network tab instead of the bare values). Refuse it explicitly so the
// failure mode is "your input looks wrong" instead of "your session got
// the literal string LEETCODE_SESSION=…".
func TestLoginFromPaste_RejectsCookieHeaderShapedInput(t *testing.T) {
	in := strings.NewReader("LEETCODE_SESSION=abc\ncsrftoken=def\n")
	var out bytes.Buffer

	if _, err := LoginFromPaste(in, &out); err == nil {
		t.Error("expected error when input contains key=value cookie shape; got nil")
	}
}

// Prompts must go to the writer the caller passed in, not stdout, so
// callers can suppress them in tests and pipe them anywhere they want
// in production.
func TestLoginFromPaste_PromptsToWriter(t *testing.T) {
	in := strings.NewReader("s\nc\n")
	var out bytes.Buffer

	if _, err := LoginFromPaste(in, &out); err != nil {
		t.Fatalf("LoginFromPaste: %v", err)
	}
	if out.Len() == 0 {
		t.Error("expected prompts to be written to the provided writer; got empty")
	}
}
