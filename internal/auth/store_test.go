package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// redirectStores points UserConfigDir at a temp directory and chdirs into
// another temp directory so legacy `.leetcode-creds.json` lookups happen in
// isolation. t.Cleanup restores the working dir.
func redirectStores(t *testing.T) (configDir, workDir string) {
	t.Helper()
	configDir = t.TempDir()
	workDir = t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir) // Linux UserConfigDir
	t.Setenv("HOME", configDir)            // macOS UserConfigDir = ~/Library/Application Support

	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	return configDir, workDir
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

func TestLoad_MigratesLegacy(t *testing.T) {
	_, workDir := redirectStores(t)

	// Plant a legacy file in CWD.
	legacy := &Credentials{Session: "old-session", CSRF: "old-csrf"}
	data, _ := json.Marshal(legacy)
	if err := os.WriteFile(filepath.Join(workDir, ".leetcode-creds.json"), data, 0o600); err != nil {
		t.Fatalf("plant legacy: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load (with legacy fallback): %v", err)
	}
	if got.Session != "old-session" || got.CSRF != "old-csrf" {
		t.Errorf("legacy migration didn't carry credentials: %+v", got)
	}

	// Migration should have written the canonical location, so a second Load
	// returns the same data without the legacy file.
	_ = os.Remove(filepath.Join(workDir, ".leetcode-creds.json"))
	got2, err := Load()
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if got2.Session != "old-session" {
		t.Errorf("post-migration Load lost session")
	}
}

func TestLoad_NoFilesReturnsError(t *testing.T) {
	redirectStores(t)
	if _, err := Load(); err == nil {
		t.Error("expected error when no creds anywhere; got nil")
	}
}

func TestMigrateLegacy_IgnoresEmptyCreds(t *testing.T) {
	_, workDir := redirectStores(t)

	// Legacy file exists but has empty fields — must not be returned.
	if err := os.WriteFile(filepath.Join(workDir, ".leetcode-creds.json"),
		[]byte(`{"session":"","csrf":""}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	c, err := migrateLegacy()
	if err != nil {
		t.Fatalf("migrateLegacy: %v", err)
	}
	if c != nil {
		t.Errorf("expected nil creds for empty legacy file; got %+v", c)
	}
}

func TestDelete_AbsentFileIsNoError(t *testing.T) {
	redirectStores(t)
	if err := Delete(); err != nil {
		t.Errorf("Delete on missing file should be a no-op; got %v", err)
	}
}
