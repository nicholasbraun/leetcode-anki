//go:build integration

package leetcode_test

import (
	"testing"

	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/leetcode/contracttest"
)

// TestContract_Live runs the contract suite against a real *leetcode.Client
// authenticated as the dedicated test account. Fails (with setup
// instructions) when no test creds are available — see CLAUDE.md Tests
// section for the one-time account setup.
//
// Build tag keeps this off the default `go test ./...` path. Run with:
//
//	go test -tags integration ./internal/leetcode/...
//
// Each run submits the fixture's known passing solution to LeetCode's
// judge, which adds an entry to the test account's submission history.
// That's expected — the test account exists to absorb that — but don't
// run this in a tight loop.
func TestContract_Live(t *testing.T) {
	creds := contracttest.LoadTestCreds(t)
	fx := twoSumFixture()
	api := leetcode.NewClient(creds)
	contracttest.ContractTest(t, api, fx)
}
