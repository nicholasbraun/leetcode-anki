package auth

import (
	"os"
	"path/filepath"
)

// TestCredsPath returns the canonical path for the dedicated-test-account
// credentials file. Collocated with the prod creds file under one config
// dir; distinct filename so the prod cache and the live-contract cache
// can't collide. Used by cmd/leetcode-test-login (writer) and the
// integration-tagged contract tests (reader).
func TestCredsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "leetcode-anki", "test-creds.json"), nil
}
