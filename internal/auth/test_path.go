package auth

import (
	"os"
	"path/filepath"
)

// TestCredsPath returns the canonical path for the dedicated-test-account
// credentials file. Collocated with the prod creds file under one config
// dir; distinct filename so the prod cache and the live-contract cache
// can't collide. Read by the integration-tagged contract tests (via
// contracttest.LoadTestCreds) and populated by hand — paste the
// LEETCODE_SESSION + csrftoken values from a logged-in browser session
// into a JSON file at this path.
func TestCredsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "leetcode-anki", "test-creds.json"), nil
}
