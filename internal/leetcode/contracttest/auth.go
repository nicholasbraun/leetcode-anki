package contracttest

import (
	"os"
	"testing"

	"leetcode-anki/internal/auth"
)

// LoadTestCreds returns credentials for the dedicated test account, or
// calls t.Skip with setup instructions if none are available. Resolution
// order:
//
//  1. Env vars LEETCODE_TEST_SESSION + LEETCODE_TEST_CSRF (CI path)
//  2. auth.TestCredsPath() on disk (local-dev path; populate via
//     `go run ./cmd/leetcode-test-login`)
//
// LoadTestCreds never falls back to the user's prod creds — the live
// contract submits code, and that submission must land on the dedicated
// test account, not the developer's personal profile.
func LoadTestCreds(t *testing.T) *auth.Credentials {
	t.Helper()

	if sess, csrf := os.Getenv("LEETCODE_TEST_SESSION"), os.Getenv("LEETCODE_TEST_CSRF"); sess != "" && csrf != "" {
		return &auth.Credentials{Session: sess, CSRF: csrf}
	}

	path, err := auth.TestCredsPath()
	if err != nil {
		t.Skipf("test creds path: %v", err)
	}
	c, err := auth.LoadFromPath(path)
	if err != nil {
		t.Skipf("no test creds at %s (%v); populate via `go run ./cmd/leetcode-test-login`", path, err)
	}
	return c
}
