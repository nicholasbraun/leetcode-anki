package auth

import (
	"os"
	"path/filepath"
)

// openTruncatedFile creates or truncates name under
// <UserCacheDir>/leetcode-anki/, returning a writer at mode 0600. The
// caller does not close it — files written here live for the duration
// of the process and the OS reclaims them on exit. Diagnostic-only;
// callers must accept a nil return when the cache dir is unavailable.
func openTruncatedFile(name string) (*os.File, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(cacheDir, "leetcode-anki")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
}

// LoginDebugLogPath returns the absolute path to the login-debug.log
// file. Used by main.go to point users at it when login fails.
func LoginDebugLogPath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cacheDir, "leetcode-anki", "login-debug.log")
}
