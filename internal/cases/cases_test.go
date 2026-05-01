package cases

import (
	"os"
	"path/filepath"
	"testing"
)

// newDiskCasesAt returns a DiskCases rooted at dir for tests; the production
// constructor uses os.UserCacheDir, which a unit test must not touch.
func newDiskCasesAt(dir string) *DiskCases { return &DiskCases{root: dir} }

func TestAddThenList(t *testing.T) {
	c := newDiskCasesAt(t.TempDir())
	if err := c.Add("two-sum", "[1,2]\n3"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got, err := c.List("two-sum")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0] != "[1,2]\n3" {
		t.Errorf("got %#v, want [\"[1,2]\\n3\"]", got)
	}
}

func TestAddDedupesIdenticalInputs(t *testing.T) {
	c := newDiskCasesAt(t.TempDir())
	for i := 0; i < 2; i++ {
		if err := c.Add("two-sum", "[1,2]\n3"); err != nil {
			t.Fatalf("Add #%d: %v", i, err)
		}
	}
	got, _ := c.List("two-sum")
	if len(got) != 1 {
		t.Errorf("expected 1 entry after dedupe, got %d: %#v", len(got), got)
	}
}

func TestRemoveValidIndex(t *testing.T) {
	c := newDiskCasesAt(t.TempDir())
	for _, in := range []string{"a", "b", "c"} {
		_ = c.Add("two-sum", in)
	}
	if err := c.Remove("two-sum", 1); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	got, _ := c.List("two-sum")
	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Errorf("after Remove(1): got %#v, want [a c]", got)
	}
}

func TestRemoveOutOfRange(t *testing.T) {
	c := newDiskCasesAt(t.TempDir())
	_ = c.Add("two-sum", "a")
	if err := c.Remove("two-sum", 5); err == nil {
		t.Error("Remove(5) on 1-entry list should error")
	}
	if err := c.Remove("two-sum", -1); err == nil {
		t.Error("Remove(-1) should error")
	}
}

func TestListMissingFileReturnsEmpty(t *testing.T) {
	c := newDiskCasesAt(t.TempDir())
	got, err := c.List("two-sum")
	if err != nil {
		t.Fatalf("List on missing file: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d entries, want empty", len(got))
	}
}

// Corrupt JSON returns an error on List and Add does not silently overwrite
// the file — the user should be able to inspect/repair the JSON without the
// app destroying their input first.
func TestCorruptJSONReturnsErrorAndPreservesFile(t *testing.T) {
	dir := t.TempDir()
	c := newDiskCasesAt(dir)
	path := filepath.Join(dir, "two-sum", "custom_testcases.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := c.List("two-sum"); err == nil {
		t.Error("List should error on corrupt JSON")
	}
	if err := c.Add("two-sum", "x"); err == nil {
		t.Error("Add should error rather than overwrite corrupt file")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "{not json" {
		t.Errorf("file was overwritten: %q", got)
	}
}

func TestAddCreatesFileMode0600(t *testing.T) {
	dir := t.TempDir()
	c := newDiskCasesAt(dir)
	if err := c.Add("two-sum", "x"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	path := filepath.Join(dir, "two-sum", "custom_testcases.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestAllMethodsRejectInvalidSlugs(t *testing.T) {
	c := newDiskCasesAt(t.TempDir())
	bad := []string{"", "..", "../etc", "two/sum", "two sum", "Two-Sum", "/etc"}
	for _, slug := range bad {
		t.Run(slug, func(t *testing.T) {
			if _, err := c.List(slug); err == nil {
				t.Errorf("List(%q) returned nil error", slug)
			}
			if err := c.Add(slug, "x"); err == nil {
				t.Errorf("Add(%q) returned nil error", slug)
			}
			if err := c.Remove(slug, 0); err == nil {
				t.Errorf("Remove(%q) returned nil error", slug)
			}
		})
	}
}
