// leetcode-test-login runs the chromedp browser login flow and writes the
// resulting credentials to the dedicated test-account creds file. Run it
// once per fresh test account and again whenever the session cookie
// expires; the live contract test suite reads creds from this path.
//
// Usage: go run ./cmd/leetcode-test-login
package main

import (
	"context"
	"fmt"
	"os"

	"leetcode-anki/internal/auth"
)

func main() {
	ctx := context.Background()

	path, err := auth.TestCredsPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "test creds path: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Logging in as the test account — credentials will be written to:\n  %s\n\n", path)

	creds, err := auth.BrowserLogin(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "browser login: %v\n", err)
		os.Exit(1)
	}

	if err := auth.SaveToPath(creds, path); err != nil {
		fmt.Fprintf(os.Stderr, "save: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Wrote test creds to %s\n", path)
}
