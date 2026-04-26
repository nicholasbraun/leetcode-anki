package sr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// currentCacheVersion is the schema version of cacheData. Bump on
// breaking shape changes; loadCache rejects higher versions to avoid
// truncating a future install's history.
const currentCacheVersion = 1

// cacheData is the on-disk shape of the SR snapshot. Held as a single
// JSON blob at os.UserConfigDir()/leetcode-anki/sr.json.
type cacheData struct {
	Version int                  `json:"version"`
	Slugs   map[string]slugEntry `json:"slugs"`
}

// slugEntry is one Problem's cached submission timeline.
type slugEntry struct {
	FetchedAt   time.Time          `json:"fetched_at"`
	Submissions []cachedSubmission `json:"submissions"`
}

// cachedSubmission mirrors the subset of leetcode.Submission the SR
// scheduler and Record-merge flow need. Kept independent so cache schema
// changes don't ripple through the API client.
type cachedSubmission struct {
	ID         string    `json:"id"`
	OccurredAt time.Time `json:"occurred_at"`
	Accepted   bool      `json:"accepted"`
	Notes      string    `json:"notes"`
	FlagType   string    `json:"flag_type"`
}

func loadCache(path string) (*cacheData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &cacheData{Version: currentCacheVersion, Slugs: map[string]slugEntry{}}, nil
		}
		return nil, fmt.Errorf("read cache: %w", err)
	}
	var c cacheData
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("decode cache: %w", err)
	}
	if c.Version > currentCacheVersion {
		return nil, fmt.Errorf("cache version %d is newer than supported %d", c.Version, currentCacheVersion)
	}
	if c.Slugs == nil {
		c.Slugs = map[string]slugEntry{}
	}
	return &c, nil
}

func (c *cacheData) save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir cache: %w", err)
	}
	c.Version = currentCacheVersion
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode cache: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// defaultCachePath is os.UserConfigDir()/leetcode-anki/sr.json. Mirrors
// the auth-creds path layout for consistency.
func defaultCachePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "leetcode-anki", "sr.json"), nil
}
