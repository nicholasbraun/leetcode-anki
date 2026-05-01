// Package cases stores per-Problem Custom Test Case lists on disk.
//
// A Custom Test Case is a test input the user attaches to a Problem so the
// next Run feeds it to interpret_solution alongside the Problem's Example
// Test Cases. See CONTEXT.md for the canonical domain definitions; this
// package is intentionally independent of internal/editor (which is
// Solution-specific) and internal/leetcode (which is wire-only).
package cases

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// Cases is the storage interface the TUI depends on. Method semantics:
//   - List returns the user's Custom Test Cases for slug. A missing file is
//     not an error (returns an empty slice). A corrupt file is an error and
//     the file is not overwritten until the user fixes it.
//   - Add appends input to the slug's list, deduping silently — adding the
//     same input twice is a no-op.
//   - Remove drops the index-th entry (0-based). Out-of-range returns an error.
type Cases interface {
	List(slug string) ([]string, error)
	Add(slug, input string) error
	Remove(slug string, index int) error
}

// currentSchemaVersion is the on-disk schema for casesFile. Bump on any
// breaking shape change; load rejects higher versions to avoid silently
// truncating a future install's inputs.
const currentSchemaVersion = 1

// casesFile is the on-disk layout. Schema-versioned so we can evolve it
// later without writing a migration tool inline with the read path.
type casesFile struct {
	Version int      `json:"version"`
	Cases   []string `json:"cases"`
}

// validSlug mirrors editor.validSlug. Duplicated locally so this package
// has no dependency on internal/editor — Custom Test Cases are independent
// of Solutions and the two should be free to evolve separately.
var validSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// DiskCases is a Cases implementation backed by JSON files under
// <root>/<slug>/custom_testcases.json. Mode 0600 keeps the user's test
// inputs from world-readable cache directories.
type DiskCases struct {
	root string
}

// NewDiskCases returns a DiskCases rooted at os.UserCacheDir()/leetcode-anki,
// matching the rest of the app's on-disk layout.
func NewDiskCases() (*DiskCases, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("user cache dir: %w", err)
	}
	return &DiskCases{root: filepath.Join(dir, "leetcode-anki")}, nil
}

func (d *DiskCases) path(slug string) string {
	return filepath.Join(d.root, slug, "custom_testcases.json")
}

// List returns the user's Custom Test Cases for slug. A missing file
// returns nil with no error so first-time callers don't have to special-case.
func (d *DiskCases) List(slug string) ([]string, error) {
	if !validSlug.MatchString(slug) {
		return nil, fmt.Errorf("invalid slug %q", slug)
	}
	return d.load(slug)
}

// Add appends input to slug's list, deduping silently.
func (d *DiskCases) Add(slug, input string) error {
	if !validSlug.MatchString(slug) {
		return fmt.Errorf("invalid slug %q", slug)
	}
	existing, err := d.load(slug)
	if err != nil {
		return err
	}
	for _, c := range existing {
		if c == input {
			return nil
		}
	}
	existing = append(existing, input)
	return d.save(slug, existing)
}

// Remove drops the index-th entry of slug's list. Out-of-range returns an error.
func (d *DiskCases) Remove(slug string, index int) error {
	if !validSlug.MatchString(slug) {
		return fmt.Errorf("invalid slug %q", slug)
	}
	existing, err := d.load(slug)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(existing) {
		return fmt.Errorf("index %d out of range [0,%d)", index, len(existing))
	}
	existing = append(existing[:index], existing[index+1:]...)
	return d.save(slug, existing)
}

func (d *DiskCases) load(slug string) ([]string, error) {
	data, err := os.ReadFile(d.path(slug))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read cases: %w", err)
	}
	var f casesFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("decode cases: %w", err)
	}
	if f.Version > currentSchemaVersion {
		return nil, fmt.Errorf("cases file version %d newer than supported %d", f.Version, currentSchemaVersion)
	}
	return f.Cases, nil
}

func (d *DiskCases) save(slug string, cases []string) error {
	path := d.path(slug)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir cases: %w", err)
	}
	data, err := json.MarshalIndent(casesFile{Version: currentSchemaVersion, Cases: cases}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode cases: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}
