package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"leetcode-anki/internal/editor"
	"leetcode-anki/internal/sr"
)

// fakeCache is an in-memory SolutionCache for tests. It records every method
// call so assertions can inspect what the TUI requested, and persists
// scaffolded "files" by (slug, lang) so subsequent Read/ExistingPath calls
// behave like a real on-disk cache without touching the filesystem.
type fakeCache struct {
	mu    sync.Mutex
	files map[string]string // key: slug+"::"+lang
	paths map[string]string // canonical path per (slug, lang)

	scaffoldCalls []scaffoldCall
	readCalls     []string
}

type scaffoldCall struct {
	Slug, Lang, Snippet string
}

func newFakeCache() *fakeCache {
	return &fakeCache{
		files: map[string]string{},
		paths: map[string]string{},
	}
}

func (c *fakeCache) key(slug, lang string) string { return slug + "::" + lang }

func (c *fakeCache) Scaffold(slug, lang, snippet string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scaffoldCalls = append(c.scaffoldCalls, scaffoldCall{slug, lang, snippet})
	k := c.key(slug, lang)
	if path, ok := c.paths[k]; ok {
		return path, nil
	}
	path := fmt.Sprintf("/fake/%s/solution.%s", slug, lang)
	c.paths[k] = path
	c.files[k] = snippet
	return path, nil
}

func (c *fakeCache) Read(path string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readCalls = append(c.readCalls, path)
	for k, p := range c.paths {
		if p == path {
			return c.files[k], nil
		}
	}
	return "", fmt.Errorf("fakeCache: not found %q", path)
}

func (c *fakeCache) ExistingPath(slug, lang string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if path, ok := c.paths[c.key(slug, lang)]; ok {
		return path
	}
	return ""
}

func (c *fakeCache) SlugsWith() (map[string]bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]bool{}
	for k := range c.paths {
		if i := strings.Index(k, "::"); i >= 0 {
			out[k[:i]] = true
		}
	}
	return out, nil
}

func (c *fakeCache) HasAny(slug string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.paths {
		if strings.HasPrefix(k, slug+"::") {
			return true
		}
	}
	return false
}

// writeSolution seeds the cache as if the user had already saved a file at
// (slug, lang). Returns the canonical path.
func (c *fakeCache) writeSolution(slug, lang, content string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := c.key(slug, lang)
	path, ok := c.paths[k]
	if !ok {
		path = fmt.Sprintf("/fake/%s/solution.%s", slug, lang)
		c.paths[k] = path
	}
	c.files[k] = content
	return path
}

// fakeEditor stubs Editor.Open. Open returns a tea.Cmd that emits whatever
// EditorDoneMsg was queued (default: success with the path the editor was
// opened on). Tests can override Err via QueueError.
type fakeEditor struct {
	mu        sync.Mutex
	openCalls []string
	queued    *editor.EditorDoneMsg
}

func newFakeEditor() *fakeEditor { return &fakeEditor{} }

func (e *fakeEditor) Open(path string) tea.Cmd {
	e.mu.Lock()
	e.openCalls = append(e.openCalls, path)
	msg := editor.EditorDoneMsg{Path: path}
	if e.queued != nil {
		msg = *e.queued
	}
	e.mu.Unlock()
	return func() tea.Msg { return msg }
}

func (e *fakeEditor) queueDone(msg editor.EditorDoneMsg) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.queued = &msg
}

// fakeReviews stubs sr.Reviews for TUI tests. Records every Record call so
// assertions can verify the verdict-detection site invokes SR with the
// right slug, submission ID, and rating.
type fakeReviews struct {
	mu      sync.Mutex
	records []recordCall
	status  sr.Status
	err     error
}

type recordCall struct {
	slug, submissionID string
	rating             int
	at                 time.Time
}

func newFakeReviews() *fakeReviews { return &fakeReviews{} }

func (f *fakeReviews) Record(_ context.Context, slug, submissionID string, rating int, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records = append(f.records, recordCall{slug: slug, submissionID: submissionID, rating: rating, at: at})
	return f.err
}

func (f *fakeReviews) Status(_ context.Context, _ string, _ time.Time) (sr.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status, f.err
}
