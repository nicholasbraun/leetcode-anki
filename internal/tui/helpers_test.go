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
	files map[string]string // key: slug+"::"+lang for canonical, or "tmp::"+path for attempts
	paths map[string]string // canonical path per (slug, lang)

	scaffoldCalls        []scaffoldCall
	attemptScaffoldCalls []attemptScaffoldCall
	readCalls            []string

	// nextAttemptSeq names successive attempt files deterministically so
	// tests can spot accidental re-scaffolds.
	nextAttemptSeq int
}

type scaffoldCall struct {
	Slug, Lang, Snippet string
}

type attemptScaffoldCall struct {
	Lang, Snippet string
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

func (c *fakeCache) ScaffoldAttemptTmp(lang, snippet string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.attemptScaffoldCalls = append(c.attemptScaffoldCalls, attemptScaffoldCall{lang, snippet})
	c.nextAttemptSeq++
	path := fmt.Sprintf("/fake/tmp/attempt-%d.%s", c.nextAttemptSeq, lang)
	c.files["tmp::"+path] = snippet
	return path, nil
}

func (c *fakeCache) Read(path string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readCalls = append(c.readCalls, path)
	if content, ok := c.files["tmp::"+path]; ok {
		return content, nil
	}
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

// writeAttempt seeds the contents of an attempt file at path, simulating
// the user having edited the scaffolded attempt in $EDITOR.
func (c *fakeCache) writeAttempt(path, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files["tmp::"+path] = content
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
	mu sync.Mutex

	records []recordCall

	// statusBySlug overrides status per slug; missing keys fall back to status.
	// Lets a single test stage "two-sum is due, three-sum isn't" without
	// rewiring every fakeReviews call site that uses the simpler default.
	statusBySlug map[string]sr.Status
	status       sr.Status

	dueResp []sr.DueProblem
	err     error

	// sessionResp is the canned response from Session. Tests that don't
	// care about Review Mode queueing can leave it zero — Session returns
	// an empty queue and the same shared `err`.
	sessionResp sr.Session

	// sessionCalls records the SessionConfig of each Session invocation so
	// tests can assert what the TUI passed (e.g. that premium slugs were
	// filtered out before SR was called).
	sessionCalls []sr.SessionConfig

	// previewResp is what Preview returns. previewErr only fires when set;
	// otherwise Preview returns previewResp with err == nil so the rating
	// modal can stage canned dates without piggybacking on the shared `err`.
	previewResp [4]time.Time
	previewErr  error
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

func (f *fakeReviews) Status(_ context.Context, slug string, _ time.Time) (sr.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.statusBySlug[slug]; ok {
		return s, f.err
	}
	return f.status, f.err
}

func (f *fakeReviews) Due(_ context.Context, _ time.Time) ([]sr.DueProblem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dueResp, f.err
}

func (f *fakeReviews) Session(_ context.Context, cfg sr.SessionConfig, _ time.Time) (sr.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessionCalls = append(f.sessionCalls, cfg)
	return f.sessionResp, f.err
}

func (f *fakeReviews) Preview(_ context.Context, _ string, _ time.Time) ([4]time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.previewResp, f.previewErr
}

// drainBatch invokes a tea.Cmd whose result is a tea.BatchMsg, running
// each leaf cmd concurrently and waiting for them all to return. Run /
// submit dispatches batch the work cmd with the loading-indicator's
// initial tick cmd, so tests that previously observed the work cmd
// through cmd() now have to unwrap the batch.
func drainBatch(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return
	}
	var wg sync.WaitGroup
	for _, c := range batch {
		if c == nil {
			continue
		}
		wg.Add(1)
		go func(c tea.Cmd) {
			defer wg.Done()
			_ = c()
		}(c)
	}
	wg.Wait()
}

// extractMsg invokes the leaf cmds of cmd (unwrapping a tea.BatchMsg if
// needed) and returns the first message of type T it finds. Lets tests
// that previously asserted on cmd()'s direct result keep their semantic
// when the dispatch is now batched with the loading-indicator's tick.
func extractMsg[T tea.Msg](cmd tea.Cmd) (T, bool) {
	var zero T
	if cmd == nil {
		return zero, false
	}
	msg := cmd()
	if t, ok := msg.(T); ok {
		return t, true
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			sub := c()
			if t, ok := sub.(T); ok {
				return t, true
			}
		}
	}
	return zero, false
}

// startBatch is the async variant: it unwraps the BatchMsg and launches
// each leaf cmd in its own goroutine without waiting. Returns a sync.WaitGroup
// the caller can use to coordinate cleanup. Used by tests that need to
// observe a long-running cmd in flight (e.g. the cancel-flow tests).
func startBatch(cmd tea.Cmd) (*sync.WaitGroup, []tea.Cmd) {
	var wg sync.WaitGroup
	if cmd == nil {
		return &wg, nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return &wg, nil
	}
	leaves := make([]tea.Cmd, 0, len(batch))
	for _, c := range batch {
		if c == nil {
			continue
		}
		leaves = append(leaves, c)
		wg.Add(1)
		go func(c tea.Cmd) {
			defer wg.Done()
			_ = c()
		}(c)
	}
	return &wg, leaves
}
