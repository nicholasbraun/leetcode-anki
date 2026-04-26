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

func TestScaffoldFile_CreatesWithSnippetWhenAbsent(t *testing.T) {
	redirectCacheDir(t)

	path, err := ScaffoldFile("two-sum", "golang", "package main\n")
	if err != nil {
		t.Fatalf("ScaffoldFile: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(body) != "package main\n" {
		t.Errorf("scaffolded body = %q, want %q", body, "package main\n")
	}
}

func TestScaffoldFile_DoesNotOverwriteExisting(t *testing.T) {
	redirectCacheDir(t)

	path, err := ScaffoldFile("two-sum", "golang", "first\n")
	if err != nil {
		t.Fatalf("first ScaffoldFile: %v", err)
	}
	if err := os.WriteFile(path, []byte("user-edit\n"), 0o644); err != nil {
		t.Fatalf("simulate user edit: %v", err)
	}

	if _, err := ScaffoldFile("two-sum", "golang", "second\n"); err != nil {
		t.Fatalf("second ScaffoldFile: %v", err)
	}
	body, _ := os.ReadFile(path)
	if string(body) != "user-edit\n" {
		t.Errorf("second scaffold overwrote user edit: got %q", body)
	}
}

func TestScaffoldFile_RejectsBadSlug(t *testing.T) {
	redirectCacheDir(t)
	if _, err := ScaffoldFile("../escape", "golang", "x"); err == nil {
		t.Error("ScaffoldFile accepted slug with traversal; expected rejection")
	}
}

func TestExistingSolutionPath_MissingFileReturnsEmpty(t *testing.T) {
	redirectCacheDir(t)
	if got := ExistingSolutionPath("two-sum", "golang"); got != "" {
		t.Errorf("ExistingSolutionPath returned %q for missing file", got)
	}
}

func TestExistingSolutionPath_ExistingFileReturnsPath(t *testing.T) {
	redirectCacheDir(t)
	want, err := ScaffoldFile("two-sum", "golang", "package main")
	if err != nil {
		t.Fatalf("ScaffoldFile: %v", err)
	}
	if got := ExistingSolutionPath("two-sum", "golang"); got != want {
		t.Errorf("ExistingSolutionPath = %q, want %q", got, want)
	}
}

func TestExistingSolutionPath_BlankLangReturnsEmpty(t *testing.T) {
	redirectCacheDir(t)
	if got := ExistingSolutionPath("two-sum", ""); got != "" {
		t.Errorf("expected blank lang to short-circuit; got %q", got)
	}
}

func TestExistingSolutionPath_RejectsBadSlug(t *testing.T) {
	redirectCacheDir(t)
	// SolutionPath returns an error for invalid slugs; ExistingSolutionPath
	// must swallow it and return "" rather than propagating.
	if got := ExistingSolutionPath("../escape", "golang"); got != "" {
		t.Errorf("ExistingSolutionPath leaked invalid slug: %q", got)
	}
}

func TestSlugsWithSolutions_EmptyWhenCacheMissing(t *testing.T) {
	redirectCacheDir(t)
	got, err := SlugsWithSolutions()
	if err != nil {
		t.Fatalf("SlugsWithSolutions: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty set, got %v", got)
	}
}

func TestSlugsWithSolutions_FindsScaffoldedSlugs(t *testing.T) {
	redirectCacheDir(t)
	if _, err := ScaffoldFile("two-sum", "golang", "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := ScaffoldFile("3sum", "python3", "y"); err != nil {
		t.Fatal(err)
	}
	got, err := SlugsWithSolutions()
	if err != nil {
		t.Fatalf("SlugsWithSolutions: %v", err)
	}
	if !got["two-sum"] || !got["3sum"] {
		t.Errorf("expected both slugs in set, got %v", got)
	}
}

// An empty subdirectory under leetcode-anki/ shouldn't count as a draft —
// guards against a future failed-write that left an empty dir behind.
func TestSlugsWithSolutions_IgnoresEmptyDir(t *testing.T) {
	dir := redirectCacheDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "leetcode-anki", "valid-slug"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := SlugsWithSolutions()
	if err != nil {
		t.Fatalf("SlugsWithSolutions: %v", err)
	}
	if got["valid-slug"] {
		t.Errorf("empty dir should not be reported: %v", got)
	}
}

func TestHasAnySolution(t *testing.T) {
	redirectCacheDir(t)
	if HasAnySolution("two-sum") {
		t.Error("expected false before scaffold")
	}
	if _, err := ScaffoldFile("two-sum", "golang", "x"); err != nil {
		t.Fatal(err)
	}
	if !HasAnySolution("two-sum") {
		t.Error("expected true after scaffold")
	}
}

func TestHasAnySolution_RejectsBadSlug(t *testing.T) {
	redirectCacheDir(t)
	if HasAnySolution("../escape") {
		t.Error("HasAnySolution accepted bad slug")
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
