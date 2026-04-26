package sr

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadCache_MissingFileReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "sr.json")
	c, err := loadCache(path)
	if err != nil {
		t.Fatalf("loadCache: %v", err)
	}
	if len(c.Slugs) != 0 {
		t.Errorf("expected empty cache, got %d entries", len(c.Slugs))
	}
}

func TestSaveAndLoad_RoundTripsSubmissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sr.json")
	at := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	original := &cacheData{
		Version: currentCacheVersion,
		Slugs: map[string]slugEntry{
			"two-sum": {
				FetchedAt: at,
				Submissions: []cachedSubmission{
					{ID: "1988694277", OccurredAt: at, Accepted: true, Notes: "TEST\n[anki:3]", FlagType: "WHITE"},
					{ID: "1988662844", OccurredAt: at.Add(-time.Hour), Accepted: true, Notes: "", FlagType: "WHITE"},
				},
			},
		},
	}
	if err := original.save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := loadCache(path)
	if err != nil {
		t.Fatalf("loadCache: %v", err)
	}
	got, ok := loaded.Slugs["two-sum"]
	if !ok {
		t.Fatal("two-sum entry missing after round-trip")
	}
	if len(got.Submissions) != 2 {
		t.Fatalf("got %d submissions, want 2", len(got.Submissions))
	}
	if got.Submissions[0].Notes != "TEST\n[anki:3]" {
		t.Errorf("notes = %q", got.Submissions[0].Notes)
	}
	if !got.FetchedAt.Equal(at) {
		t.Errorf("FetchedAt = %v, want %v", got.FetchedAt, at)
	}
}

// File mode 0600 matches the auth/store.go pattern. The cache contains
// LeetCode submission history — not secrets, but private to the user; no
// reason for other accounts on the box to be able to read it.
func TestSave_FileMode0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sr.json")
	c := &cacheData{Version: currentCacheVersion, Slugs: map[string]slugEntry{}}
	if err := c.save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v, want 0600", info.Mode().Perm())
	}
}

// A future-version file means the user installed a newer leetcode-anki
// that wrote a schema we don't understand. Reject rather than silently
// truncating their history on load.
func TestLoadCache_RejectsHigherVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sr.json")
	if err := os.WriteFile(path, []byte(`{"version":999,"slugs":{}}`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := loadCache(path); err == nil {
		t.Fatal("expected error from higher version")
	}
}

func TestLoadCache_RejectsCorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sr.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := loadCache(path); err == nil {
		t.Fatal("expected error from corrupt JSON")
	}
}
