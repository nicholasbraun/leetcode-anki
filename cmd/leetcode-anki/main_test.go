package main

import (
	"os"
	"testing"
)

func TestCredsFromEnv_ReturnsValuesWhenBothSet(t *testing.T) {
	t.Setenv("LEETCODE_SESSION", "sess-value")
	t.Setenv("LEETCODE_CSRF", "csrf-value")

	c := credsFromEnv()
	if c == nil {
		t.Fatal("credsFromEnv returned nil; expected creds")
	}
	if c.Session != "sess-value" {
		t.Errorf("Session = %q, want %q", c.Session, "sess-value")
	}
	if c.CSRF != "csrf-value" {
		t.Errorf("CSRF = %q, want %q", c.CSRF, "csrf-value")
	}
}

func TestCredsFromEnv_NilWhenEitherMissing(t *testing.T) {
	t.Setenv("LEETCODE_SESSION", "sess-value")
	t.Setenv("LEETCODE_CSRF", "")
	if c := credsFromEnv(); c != nil {
		t.Errorf("expected nil when CSRF empty; got %+v", c)
	}

	t.Setenv("LEETCODE_SESSION", "")
	t.Setenv("LEETCODE_CSRF", "csrf-value")
	if c := credsFromEnv(); c != nil {
		t.Errorf("expected nil when SESSION empty; got %+v", c)
	}
}

// credsFromEnv must clear the env vars after capturing them so editor
// subprocesses launched via tea.ExecProcess (and any plugins they load)
// don't see the live session cookie in os.Environ().
func TestCredsFromEnv_ClearsEnvVarsAfterCapture(t *testing.T) {
	t.Setenv("LEETCODE_SESSION", "sess-value")
	t.Setenv("LEETCODE_CSRF", "csrf-value")

	_ = credsFromEnv()

	if v := os.Getenv("LEETCODE_SESSION"); v != "" {
		t.Errorf("LEETCODE_SESSION not cleared: %q", v)
	}
	if v := os.Getenv("LEETCODE_CSRF"); v != "" {
		t.Errorf("LEETCODE_CSRF not cleared: %q", v)
	}
}
