package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Credentials are the leetcode.com browser cookies the client needs to
// authenticate. Both fields are required; an empty value means we never
// completed the login flow.
type Credentials struct {
	// Session is the value of the LEETCODE_SESSION cookie.
	Session string `json:"session"`
	// CSRF is the value of the csrftoken cookie. It must also be sent in
	// the x-csrftoken header on POSTs.
	CSRF string `json:"csrf"`
}

func cachePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "leetcode-anki", "creds.json"), nil
}

// Load returns the cached credentials, or an error if none are available.
func Load() (*Credentials, error) {
	p, err := cachePath()
	if err != nil {
		return nil, err
	}
	return LoadFromPath(p)
}

// LoadFromPath reads credentials from an arbitrary file path. It refuses
// to read files whose mode is wider than 0600: a wider mode means a backup
// restore, a chmod typo, or another process has loosened the file — at
// which point another local user could be reading the live
// LEETCODE_SESSION cookie. Forcing re-auth is safer than silently
// consuming the suspect credential.
func LoadFromPath(path string) (*Credentials, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return nil, fmt.Errorf("creds file %s has mode %o; expected 0600 — chmod 600 it or delete to re-auth", path, perm)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes c to the canonical creds file, creating the parent directory
// (mode 0700) if needed. The file itself is created with mode 0600.
func Save(c *Credentials) error {
	p, err := cachePath()
	if err != nil {
		return err
	}
	return SaveToPath(c, p)
}

// SaveToPath writes c to an arbitrary file path, creating the parent
// directory (mode 0700) if needed. The file itself is created with mode
// 0600.
func SaveToPath(c *Credentials, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Delete removes the cached creds file. A missing file is not an error, so
// Delete is safe to call as a "make sure we re-auth next time" primitive.
func Delete() error {
	p, err := cachePath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
