package leetcode

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"leetcode-anki/internal/auth"
)

// fixedDoer returns the same status/body for every request.
type fixedDoer struct {
	status int
	body   string
}

func (f *fixedDoer) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: f.status,
		Body:       io.NopCloser(strings.NewReader(f.body)),
		Header:     make(http.Header),
	}, nil
}

// rawBodyDoer returns an arbitrary byte body — useful for size-limit
// tests where building the body as a string would be wasteful.
type rawBodyDoer struct {
	status int
	body   []byte
}

func (r *rawBodyDoer) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: r.status,
		Body:       io.NopCloser(bytes.NewReader(r.body)),
		Header:     make(http.Header),
	}, nil
}

// A response body larger than maxResponseBytes must be rejected before it
// can OOM the process. The 30s client timeout bounds wall time, not bytes.
func TestDoGraphQL_RejectsOversizedBody(t *testing.T) {
	huge := bytes.Repeat([]byte("x"), maxResponseBytes+1)
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, &rawBodyDoer{status: 200, body: huge})

	_, err := c.doGraphQL(context.Background(), "probeOp", "query{probe}", nil, "")
	if err == nil {
		t.Fatal("doGraphQL accepted oversized body; expected error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error %q does not mention size limit", err)
	}
}

func TestDoREST_RejectsOversizedBody(t *testing.T) {
	huge := bytes.Repeat([]byte("x"), maxResponseBytes+1)
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, &rawBodyDoer{status: 200, body: huge})

	var out struct{}
	err := c.doREST(context.Background(), http.MethodGet, "https://leetcode.com/x", nil, &out, "")
	if err == nil {
		t.Fatal("doREST accepted oversized body; expected error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error %q does not mention size limit", err)
	}
}

// A body just under the cap must still decode normally — the limit is a
// hard wall, not a "near-cap warning."
func TestDoGraphQL_AcceptsBodyJustUnderCap(t *testing.T) {
	prefix := []byte(`{"data":{"x":"`)
	suffix := []byte(`"}}`)
	pad := bytes.Repeat([]byte("x"), maxResponseBytes-len(prefix)-len(suffix)-1)
	body := append(append(append([]byte{}, prefix...), pad...), suffix...)
	if len(body) > maxResponseBytes {
		t.Fatalf("test body %d > cap %d", len(body), maxResponseBytes)
	}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, &rawBodyDoer{status: 200, body: body})
	if _, err := c.doGraphQL(context.Background(), "probeOp", "query{probe}", nil, ""); err != nil {
		t.Errorf("doGraphQL rejected body just under cap: %v", err)
	}
}

// resetDebugTruncForTest puts the debug-log truncation flag back into
// "no first-call yet" state so each test starts deterministically,
// regardless of test order or previous tests touching appendDebugLog.
func resetDebugTruncForTest(t *testing.T) {
	t.Helper()
	debugTruncMu.Lock()
	debugTruncated = false
	debugTruncMu.Unlock()
}

// markDebugTruncForTest pretends the first-write truncation has already
// happened, so subsequent appendDebugLog calls take the cap-enforcement
// branch instead of truncating again.
func markDebugTruncForTest(t *testing.T) {
	t.Helper()
	debugTruncMu.Lock()
	debugTruncated = true
	debugTruncMu.Unlock()
}

func TestDoGraphQL_AppendsRawResponseToDebugLog(t *testing.T) {
	resetDebugTruncForTest(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	t.Setenv("LEETCODE_DEBUG", "1")

	const respBody = `{"data":{"probe":{"hello":"world"}}}`
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, &fixedDoer{status: 200, body: respBody})

	if _, err := c.doGraphQL(context.Background(), "probeOp", "query{probe}", nil, ""); err != nil {
		t.Fatalf("doGraphQL: %v", err)
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("UserCacheDir: %v", err)
	}
	logPath := filepath.Join(cacheDir, "leetcode-anki", "debug.log")
	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read debug log at %s: %v", logPath, err)
	}
	got := string(contents)
	if !strings.Contains(got, "probeOp") || !strings.Contains(got, respBody) {
		t.Errorf("debug log missing op name or body. got: %q", got)
	}

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat debug log: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("debug log mode = %v, want 0600", perm)
	}
}

// Once a process has grown debug.log past debugLogMaxBytes, further
// appends are dropped rather than rotated — without this cap an
// unattended LEETCODE_DEBUG run could fill the cache partition. The cap
// applies *after* the first-call truncation, so this test marks the
// truncation as already done before pre-seeding.
func TestAppendDebugLog_StopsWritingPastCap(t *testing.T) {
	markDebugTruncForTest(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("LEETCODE_DEBUG", "1")

	cacheDir, _ := os.UserCacheDir()
	dir := filepath.Join(cacheDir, "leetcode-anki")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "debug.log")
	preSeed := make([]byte, debugLogMaxBytes+1)
	if err := os.WriteFile(logPath, preSeed, 0o600); err != nil {
		t.Fatal(err)
	}

	appendDebugLog("over-cap", []byte(`{"data":"this should not land"}`))

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() != int64(len(preSeed)) {
		t.Errorf("expected file size unchanged at %d, got %d (write past cap leaked through)",
			len(preSeed), info.Size())
	}
}

// The first appendDebugLog call in a process must truncate any
// pre-existing log so each run starts with a fresh window. Without this
// the file would retain the *first* 10 MiB recorded across all runs
// rather than the most recent.
func TestAppendDebugLog_TruncatesPreviousContentOnFirstWrite(t *testing.T) {
	resetDebugTruncForTest(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("LEETCODE_DEBUG", "1")

	cacheDir, _ := os.UserCacheDir()
	dir := filepath.Join(cacheDir, "leetcode-anki")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "debug.log")
	if err := os.WriteFile(logPath, []byte("STALE-FROM-PREVIOUS-RUN\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	appendDebugLog("first", []byte(`{"a":1}`))
	appendDebugLog("second", []byte(`{"b":2}`))

	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	gotStr := string(got)
	if strings.Contains(gotStr, "STALE-FROM-PREVIOUS-RUN") {
		t.Errorf("stale content survived first write; got %q", gotStr)
	}
	want := "first\t{\"a\":1}\nsecond\t{\"b\":2}\n"
	if gotStr != want {
		t.Errorf("debug.log = %q, want %q", gotStr, want)
	}
}

func TestDoGraphQL_NoLogWhenDebugDisabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	t.Setenv("LEETCODE_DEBUG", "")

	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, &fixedDoer{status: 200, body: `{"data":{}}`})
	if _, err := c.doGraphQL(context.Background(), "probeOp", "query{probe}", nil, ""); err != nil {
		t.Fatalf("doGraphQL: %v", err)
	}

	cacheDir, _ := os.UserCacheDir()
	if _, err := os.Stat(filepath.Join(cacheDir, "leetcode-anki", "debug.log")); !os.IsNotExist(err) {
		t.Errorf("expected no debug log file; stat err = %v", err)
	}
}
