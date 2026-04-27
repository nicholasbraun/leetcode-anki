package auth

import (
	"os"
	"path/filepath"
	"testing"
)

// redirectStores points UserConfigDir at a temp directory so creds reads
// and writes don't touch the user's real config dir.
func redirectStores(t *testing.T) {
	t.Helper()
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir) // Linux UserConfigDir
	t.Setenv("HOME", configDir)            // macOS UserConfigDir = ~/Library/Application Support
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	redirectStores(t)
	want := &Credentials{Session: "sess", CSRF: "csrf"}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Session != want.Session || got.CSRF != want.CSRF {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestSave_FileMode0600(t *testing.T) {
	redirectStores(t)
	if err := Save(&Credentials{Session: "s", CSRF: "c"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	p, _ := cachePath()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("creds file mode = %o, want 600", info.Mode().Perm())
	}
}

func TestLoad_NoFilesReturnsError(t *testing.T) {
	redirectStores(t)
	if _, err := Load(); err == nil {
		t.Error("expected error when no creds anywhere; got nil")
	}
}

// A creds file with permissions wider than 0600 means another local user
// (or a backup-restore that didn't preserve mode) can read the live
// LEETCODE_SESSION cookie. Refuse to consume the file rather than silently
// using a credential the user thinks is private.
func TestLoad_RefusesWideCredsPermissions(t *testing.T) {
	redirectStores(t)
	if err := Save(&Credentials{Session: "s", CSRF: "c"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	p, _ := cachePath()
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	if _, err := Load(); err == nil {
		t.Error("expected Load to refuse a 0644 creds file; got nil")
	}
}

func TestLoad_AcceptsTightCredsPermissions(t *testing.T) {
	redirectStores(t)
	if err := Save(&Credentials{Session: "s", CSRF: "c"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	p, _ := cachePath()
	// 0400 (read-only owner) is tighter than 0600 — must still be accepted.
	if err := os.Chmod(p, 0o400); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	if _, err := Load(); err != nil {
		t.Errorf("Load with 0400 creds file: %v", err)
	}
}

func TestDelete_AbsentFileIsNoError(t *testing.T) {
	redirectStores(t)
	if err := Delete(); err != nil {
		t.Errorf("Delete on missing file should be a no-op; got %v", err)
	}
}

func TestSaveToPath_AndLoadFromPath_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "creds.json")
	want := &Credentials{Session: "sess", CSRF: "csrf"}

	if err := SaveToPath(want, path); err != nil {
		t.Fatalf("SaveToPath: %v", err)
	}
	got, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	if got.Session != want.Session || got.CSRF != want.CSRF {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestSaveToPath_FileMode0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	if err := SaveToPath(&Credentials{Session: "s", CSRF: "c"}, path); err != nil {
		t.Fatalf("SaveToPath: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %o, want 600", info.Mode().Perm())
	}
}

func TestLoadFromPath_RefusesWidePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	if err := SaveToPath(&Credentials{Session: "s", CSRF: "c"}, path); err != nil {
		t.Fatalf("SaveToPath: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	if _, err := LoadFromPath(path); err == nil {
		t.Error("LoadFromPath: expected refusal of 0644 file; got nil")
	}
}

func TestLoadFromPath_MissingFileIsError(t *testing.T) {
	if _, err := LoadFromPath(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("LoadFromPath: expected error for missing file; got nil")
	}
}

// TestCredsPath must point to a sibling of the prod creds file, not a
// nested directory — the project keeps a single config dir per app.
func TestTestCredsPath_SiblingOfProdCreds(t *testing.T) {
	redirectStores(t)
	prod, err := cachePath()
	if err != nil {
		t.Fatalf("cachePath: %v", err)
	}
	test, err := TestCredsPath()
	if err != nil {
		t.Fatalf("TestCredsPath: %v", err)
	}
	if filepath.Dir(prod) != filepath.Dir(test) {
		t.Errorf("test creds dir %q != prod creds dir %q", filepath.Dir(test), filepath.Dir(prod))
	}
	if filepath.Base(test) == filepath.Base(prod) {
		t.Errorf("test creds path collides with prod creds path: %q", test)
	}
}
