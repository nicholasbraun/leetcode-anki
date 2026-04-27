package leetcode

import (
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

func TestDoGraphQL_AppendsRawResponseToDebugLog(t *testing.T) {
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

// Once debug.log passes debugLogMaxBytes, further appends are dropped
// rather than rotated — the user is expected to `rm` the file to start
// a new collection window. Without this cap an unattended LEETCODE_DEBUG
// run could fill the cache partition.
func TestAppendDebugLog_StopsWritingPastCap(t *testing.T) {
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
	// Pre-seed the log past the cap. Anything written after this should
	// be dropped, so the file size must be unchanged.
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
