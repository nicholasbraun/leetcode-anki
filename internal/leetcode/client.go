package leetcode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"leetcode-anki/internal/auth"
)

const (
	// BaseURL is the leetcode.com origin used for REST endpoints and Referer headers.
	BaseURL = "https://leetcode.com"
	// GraphQLURL is the single endpoint all GraphQL queries POST to.
	GraphQLURL = "https://leetcode.com/graphql/"
	// UserAgent is sent on every request. LeetCode rejects requests that claim
	// no UA at all; the value isn't otherwise validated server-side.
	UserAgent = "Mozilla/5.0"
)

// httpDoer is the subset of *http.Client we depend on. Tests inject fakes
// against this interface; production code uses *http.Client.
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client is the LeetCode GraphQL+REST client. It centralises auth headers so
// individual API methods can't forget to attach them.
type Client struct {
	creds *auth.Credentials
	http  httpDoer
}

// NewClient builds a Client that talks to leetcode.com using a default HTTP
// client with a 30s per-request timeout.
func NewClient(creds *auth.Credentials) *Client {
	return &Client{
		creds: creds,
		http:  &http.Client{Timeout: 30 * time.Second},
	}
}

// newClientWithDoer is the test seam for injecting a fake httpDoer.
func newClientWithDoer(creds *auth.Credentials, doer httpDoer) *Client {
	return &Client{creds: creds, http: doer}
}

type graphQLRequest struct {
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables"`
	OperationName string         `json:"operationName"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type graphQLEnvelope struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphQLError  `json:"errors,omitempty"`
}

// doGraphQL POSTs a GraphQL operation and returns the unwrapped `data` payload
// (or an error describing a transport, status, or top-level GraphQL error).
// The caller decodes the data shape it expects.
func (c *Client) doGraphQL(ctx context.Context, opName, query string, vars map[string]any, referer string) (json.RawMessage, error) {
	body := graphQLRequest{Query: query, Variables: vars, OperationName: opName}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, GraphQLURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	c.setHeaders(req, referer)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	appendDebugLog(opName, raw)
	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp.StatusCode, raw)
	}

	var env graphQLEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if len(env.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", env.Errors[0].Message)
	}
	return env.Data, nil
}

// doREST executes a REST request, decoding the JSON response into out if it
// is non-nil and the body is non-empty. Status codes outside 2xx return an
// error built by statusError (no body included).
func (c *Client) doREST(ctx context.Context, method, url string, in any, out any, referer string) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	c.setHeaders(req, referer)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusError(resp.StatusCode, raw)
	}
	if out == nil {
		return nil
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// statusError builds a user-facing error for a non-2xx response.
//
// We deliberately omit the response body. LeetCode's error pages can be many
// KB of HTML and may contain server-side debug, request-correlation IDs, or
// echoes of CSRF tokens — none of which belong in a TUI banner. Callers that
// need the body for diagnostics should enable debug logging instead.
func statusError(code int, body []byte) error {
	if debugLogEnabled() {
		return fmt.Errorf("status %d: %s", code, string(body))
	}
	return fmt.Errorf("status %d", code)
}

// debugLogEnabled reports whether the LEETCODE_DEBUG env var is set.
// When enabled, raw GraphQL response bodies, dropped-row events (bad
// timestamps, etc.), and HTTP non-2xx response bodies are appended to
// `<UserCacheDir>/leetcode-anki/debug.log` (mode 0600), and statusError
// includes the response body in returned errors. Used to diagnose
// LeetCode schema drift. Read at call time (not init) so tests can flip
// the flag with t.Setenv.
//
// SECURITY: debug.log contains live API responses. Those responses
// include Problem descriptions, Solution code submitted by the user, AC
// rates, and request-correlation IDs. Treat the file as user data — do
// not commit it, paste it into bug reports without scrubbing, or enable
// LEETCODE_DEBUG on a shared workstation.
func debugLogEnabled() bool {
	return os.Getenv("LEETCODE_DEBUG") != ""
}

// debugLogMaxBytes caps debug.log so an unattended LEETCODE_DEBUG run
// can't fill the user's cache partition. Once the file passes the cap,
// further writes are dropped (no rotation — the user is expected to
// `rm` the file to start a new collection window).
const debugLogMaxBytes = 10 * 1024 * 1024

// appendDebugLog writes one line `<opName>\t<raw>` to the debug log
// file. No-op when LEETCODE_DEBUG is unset, when the file is at or
// past debugLogMaxBytes, or on any I/O failure: diagnostics must never
// break the request path.
func appendDebugLog(opName string, raw []byte) {
	if !debugLogEnabled() {
		return
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	dir := filepath.Join(cacheDir, "leetcode-anki")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	path := filepath.Join(dir, "debug.log")
	if info, err := os.Stat(path); err == nil && info.Size() >= debugLogMaxBytes {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s\t%s\n", opName, raw)
}

// setHeaders attaches the headers that authenticated leetcode.com endpoints
// require (Cookie, x-csrftoken, Referer, User-Agent). Centralising this here
// means individual API methods cannot forget one and silently 401.
func (c *Client) setHeaders(req *http.Request, referer string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("x-csrftoken", c.creds.CSRF)
	req.Header.Set("Cookie", fmt.Sprintf("LEETCODE_SESSION=%s; csrftoken=%s", c.creds.Session, c.creds.CSRF))
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
}
