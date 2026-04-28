package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExt_KnownAndUnknown(t *testing.T) {
	cases := map[string]string{
		"golang":  "go",
		"python3": "py",
		"cpp":     "cpp",
		"":        "txt",
		"klingon": "txt",
	}
	for in, want := range cases {
		if got := Ext(in); got != want {
			t.Errorf("Ext(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSolutionPath_RejectsTraversal(t *testing.T) {
	bad := []string{
		"../etc",
		"two-sum/../../etc",
		"..",
		"/absolute",
		"two sum",       // space
		"Two-Sum",       // uppercase
		"two-sum\x00x",  // NUL
		"-leading-dash", // hyphen at start
		"",
	}
	for _, slug := range bad {
		if _, err := SolutionPath(slug, "golang"); err == nil {
			t.Errorf("SolutionPath(%q) returned no error; expected rejection", slug)
		}
	}
}

func TestSolutionPath_AcceptsValidSlug(t *testing.T) {
	good := []string{"two-sum", "3sum", "valid-anagram", "a"}
	for _, slug := range good {
		got, err := SolutionPath(slug, "golang")
		if err != nil {
			t.Errorf("SolutionPath(%q) errored: %v", slug, err)
			continue
		}
		if !strings.HasSuffix(got, filepath.Join(slug, "solution.go")) {
			t.Errorf("SolutionPath(%q) = %q, missing expected suffix", slug, got)
		}
	}
}

// scaffold tests redirect UserCacheDir via $XDG_CACHE_HOME (Linux) and
// HOME (macOS fallback). t.Setenv handles cleanup.
func redirectCacheDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir) // macOS picks ~/Library/Caches under HOME
	return dir
}

func TestScaffold_CreatesWithSnippetWhenAbsent(t *testing.T) {
	redirectCacheDir(t)

	c := NewCache()
	path, err := c.Scaffold("two-sum", "golang", "package main\n")
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(body) != "package main\n" {
		t.Errorf("scaffolded body = %q, want %q", body, "package main\n")
	}
}

func TestScaffold_DoesNotOverwriteExisting(t *testing.T) {
	redirectCacheDir(t)

	c := NewCache()
	path, err := c.Scaffold("two-sum", "golang", "first\n")
	if err != nil {
		t.Fatalf("first Scaffold: %v", err)
	}
	if err := os.WriteFile(path, []byte("user-edit\n"), 0o644); err != nil {
		t.Fatalf("simulate user edit: %v", err)
	}

	if _, err := c.Scaffold("two-sum", "golang", "second\n"); err != nil {
		t.Fatalf("second Scaffold: %v", err)
	}
	body, _ := os.ReadFile(path)
	if string(body) != "user-edit\n" {
		t.Errorf("second scaffold overwrote user edit: got %q", body)
	}
}

func TestScaffold_RejectsBadSlug(t *testing.T) {
	redirectCacheDir(t)
	c := NewCache()
	if _, err := c.Scaffold("../escape", "golang", "x"); err == nil {
		t.Error("Scaffold accepted slug with traversal; expected rejection")
	}
}

// Each Review-Mode attempt must live in its own directory. Go treats
// every .go file in a directory as part of one package, so two
// attempts in the same dir (current session + leftover, or two
// different Problems back-to-back) would collide on duplicate function
// names and surface as gopls errors in the user's editor.
func TestScaffoldAttemptTmp_GivesEachAttemptOwnDir(t *testing.T) {
	c := NewCache()
	a, err := c.ScaffoldAttemptTmp("golang", "package main\nfunc twoSum() {}\n")
	if err != nil {
		t.Fatalf("first ScaffoldAttemptTmp: %v", err)
	}
	b, err := c.ScaffoldAttemptTmp("golang", "package main\nfunc twoSum() {}\n")
	if err != nil {
		t.Fatalf("second ScaffoldAttemptTmp: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(filepath.Dir(a))
		os.RemoveAll(filepath.Dir(b))
	})

	if filepath.Dir(a) == filepath.Dir(b) {
		t.Errorf("two attempts share a parent directory %q — Go would treat them as duplicate package definitions", filepath.Dir(a))
	}
	body, err := os.ReadFile(a)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", a, err)
	}
	if string(body) != "package main\nfunc twoSum() {}\n" {
		t.Errorf("attempt body = %q, want the seeded snippet", body)
	}
	if !strings.HasSuffix(a, ".go") {
		t.Errorf("attempt path %q missing .go extension for golang lang", a)
	}
}

func TestExistingPath_MissingFileReturnsEmpty(t *testing.T) {
	redirectCacheDir(t)
	c := NewCache()
	if got := c.ExistingPath("two-sum", "golang"); got != "" {
		t.Errorf("ExistingPath returned %q for missing file", got)
	}
}

func TestExistingPath_ExistingFileReturnsPath(t *testing.T) {
	redirectCacheDir(t)
	c := NewCache()
	want, err := c.Scaffold("two-sum", "golang", "package main")
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if got := c.ExistingPath("two-sum", "golang"); got != want {
		t.Errorf("ExistingPath = %q, want %q", got, want)
	}
}

func TestExistingPath_BlankLangReturnsEmpty(t *testing.T) {
	redirectCacheDir(t)
	c := NewCache()
	if got := c.ExistingPath("two-sum", ""); got != "" {
		t.Errorf("expected blank lang to short-circuit; got %q", got)
	}
}

func TestExistingPath_RejectsBadSlug(t *testing.T) {
	redirectCacheDir(t)
	c := NewCache()
	// SolutionPath returns an error for invalid slugs; ExistingPath must
	// swallow it and return "" rather than propagating.
	if got := c.ExistingPath("../escape", "golang"); got != "" {
		t.Errorf("ExistingPath leaked invalid slug: %q", got)
	}
}

func TestSlugsWith_EmptyWhenCacheMissing(t *testing.T) {
	redirectCacheDir(t)
	c := NewCache()
	got, err := c.SlugsWith()
	if err != nil {
		t.Fatalf("SlugsWith: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty set, got %v", got)
	}
}

func TestSlugsWith_FindsScaffoldedSlugs(t *testing.T) {
	redirectCacheDir(t)
	c := NewCache()
	if _, err := c.Scaffold("two-sum", "golang", "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Scaffold("3sum", "python3", "y"); err != nil {
		t.Fatal(err)
	}
	got, err := c.SlugsWith()
	if err != nil {
		t.Fatalf("SlugsWith: %v", err)
	}
	if !got["two-sum"] || !got["3sum"] {
		t.Errorf("expected both slugs in set, got %v", got)
	}
}

// An empty subdirectory under leetcode-anki/ shouldn't count as a Solution —
// guards against a future failed-write that left an empty dir behind.
func TestSlugsWith_IgnoresEmptyDir(t *testing.T) {
	dir := redirectCacheDir(t)
	c := NewCache()
	if err := os.MkdirAll(filepath.Join(dir, "leetcode-anki", "valid-slug"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := c.SlugsWith()
	if err != nil {
		t.Fatalf("SlugsWith: %v", err)
	}
	if got["valid-slug"] {
		t.Errorf("empty dir should not be reported: %v", got)
	}
}

func TestHasAny(t *testing.T) {
	redirectCacheDir(t)
	c := NewCache()
	if c.HasAny("two-sum") {
		t.Error("expected false before scaffold")
	}
	if _, err := c.Scaffold("two-sum", "golang", "x"); err != nil {
		t.Fatal(err)
	}
	if !c.HasAny("two-sum") {
		t.Error("expected true after scaffold")
	}
}

func TestHasAny_RejectsBadSlug(t *testing.T) {
	redirectCacheDir(t)
	c := NewCache()
	if c.HasAny("../escape") {
		t.Error("HasAny accepted bad slug")
	}
}

func TestChromaLang(t *testing.T) {
	cases := map[string]string{
		"golang":  "go",
		"python3": "python",
		"rust":    "rust",
		"java":    "java",
		"klingon": "klingon",
	}
	for in, want := range cases {
		if got := ChromaLang(in); got != want {
			t.Errorf("ChromaLang(%q) = %q, want %q", in, got, want)
		}
	}
}
