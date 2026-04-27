package contracttest

import (
	"fmt"
	"os"
	"testing"

	"leetcode-anki/internal/auth"
)

// LoadTestCreds returns credentials for the dedicated test account, or
// fails the test with setup instructions if none are available.
// Resolution order:
//
//  1. Env vars LEETCODE_TEST_SESSION + LEETCODE_TEST_CSRF (CI path)
//  2. auth.TestCredsPath() on disk (local-dev path; populate via
//     `go run ./cmd/leetcode-test-login`)
//
// Missing creds are a t.Fatal, not a t.Skip: `go test` buffers per-test
// stderr and only flushes it on failure or -v, so a t.Skip message would
// be invisible by default — leaving a fresh clone running `go test
// -tags integration ./...` to wonder why nothing actually ran. Failure
// surfaces the setup pointer reliably; if you don't want to run the
// live contract, simply omit `-tags integration`.
//
// LoadTestCreds never falls back to the user's prod creds — the live
// contract submits code, and that submission must land on the dedicated
// test account, not the developer's personal profile.
func LoadTestCreds(t *testing.T) *auth.Credentials {
	t.Helper()
	creds, missingMsg := resolveTestCreds()
	if creds != nil {
		return creds
	}
	t.Fatal(missingMsg)
	return nil // unreachable; t.Fatal aborts
}

// resolveTestCreds is the testable core of LoadTestCreds: it returns
// either the resolved credentials or a human-readable string explaining
// how to populate them.
func resolveTestCreds() (*auth.Credentials, string) {
	if sess, csrf := os.Getenv("LEETCODE_TEST_SESSION"), os.Getenv("LEETCODE_TEST_CSRF"); sess != "" && csrf != "" {
		return &auth.Credentials{Session: sess, CSRF: csrf}, ""
	}
	path, err := auth.TestCredsPath()
	if err != nil {
		return nil, fmt.Sprintf("test creds path: %v", err)
	}
	c, err := auth.LoadFromPath(path)
	if err != nil {
		return nil, fmt.Sprintf("no test creds at %s (%v); populate via `go run ./cmd/leetcode-test-login`", path, err)
	}
	return c, ""
}
