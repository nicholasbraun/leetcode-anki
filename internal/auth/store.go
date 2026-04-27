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
//
// Side effect: if the canonical creds file is missing but a legacy
// `.leetcode-creds.json` exists in the current working directory, Load
// migrates it (writing the canonical file) and returns the migrated value.
// Callers should treat a non-nil error as "no credentials"; they don't need
// to distinguish missing-file from migrate-failed.
//
// Load also refuses to read the file if its mode is wider than 0600. We
// always Save with 0600, so a wider mode means a backup restore, a chmod
// typo, or another process has loosened the file — at which point another
// local user could be reading the live LEETCODE_SESSION cookie. Forcing
// re-auth is safer than silently consuming the suspect credential.
func Load() (*Credentials, error) {
	p, err := cachePath()
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			if c, mErr := migrateLegacy(); mErr == nil && c != nil {
				return c, nil
			}
		}
		return nil, err
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return nil, fmt.Errorf("creds file %s has mode %o; expected 0600 — chmod 600 it or delete to re-auth", p, perm)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// migrateLegacy attempts a one-time copy from the old CWD-relative
// `.leetcode-creds.json` to the new UserConfigDir location. Returns nil, nil
// if the legacy file isn't present.
func migrateLegacy() (*Credentials, error) {
	const legacy = ".leetcode-creds.json"
	data, err := os.ReadFile(legacy)
	if err != nil {
		return nil, err
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if c.Session == "" || c.CSRF == "" {
		return nil, nil
	}
	if err := Save(&c); err != nil {
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
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
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
