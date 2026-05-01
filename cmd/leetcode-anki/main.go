package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"leetcode-anki/internal/auth"
	"leetcode-anki/internal/cases"
	"leetcode-anki/internal/editor"
	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/sr"
	"leetcode-anki/internal/tui"
)

func main() {
	logout := flag.Bool("logout", false, "delete cached credentials, then exit")
	reviewDue := flag.Int("review-due", 2, "Review Mode: max overdue Reviews per session (0 to omit)")
	reviewNew := flag.Int("review-new", 1, "Review Mode: max new Problems per session (0 to omit)")
	flag.Parse()

	// Clamp negatives to zero — `make([]X, 0, -1)` panics, and "minus
	// one due" has no meaningful interpretation either way.
	*reviewDue = clampNonNegative(*reviewDue)
	*reviewNew = clampNonNegative(*reviewNew)

	ctx := context.Background()

	if *logout {
		if err := auth.Delete(); err != nil {
			fmt.Fprintf(os.Stderr, "logout: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "Cached credentials deleted. Re-run leetcode-anki to log in again.")
		return
	}

	creds, status, err := resolveCreds(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "auth: %v\n", err)
		if path := auth.LoginDebugLogPath(); path != "" {
			fmt.Fprintf(os.Stderr, "Diagnostics: %s\n", path)
		}
		os.Exit(1)
	}

	client := leetcode.NewClient(creds)
	cache := editor.NewCache()
	runner := editor.NewRunner()

	customCases, err := cases.NewDiskCases()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cases: %v\n", err)
		os.Exit(1)
	}

	reviews, err := sr.Open(client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sr: %v\n", err)
		os.Exit(1)
	}

	if err := tui.Run(ctx, client, cache, runner, customCases, reviews, *reviewDue, *reviewNew, status.IsPremium); err != nil {
		fmt.Fprintf(os.Stderr, "tui: %v\n", err)
		os.Exit(1)
	}
}

func clampNonNegative(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// resolveCreds picks a credential pair for the session, verifying it
// against leetcode.com:
//
//  1. Env vars LEETCODE_SESSION + LEETCODE_CSRF (no disk write — useful
//     in CI and on shared machines).
//  2. Cached creds.json from a previous successful login.
//  3. Interactive login TUI (browser-extract or paste).
//
// Each step that produces a credential pair runs Verify before
// accepting it. A network-unreachable failure (offline, DNS down) is
// treated as "valid enough" so users without internet aren't stuck in
// the login screen — the first real API call will surface a useful
// error later.
func resolveCreds(ctx context.Context) (*auth.Credentials, leetcode.UserStatus, error) {
	if c := credsFromEnv(); c != nil {
		if status, accepted := tryVerify(ctx, c); accepted {
			return c, status, nil
		}
		fmt.Fprintln(os.Stderr, "warning: LEETCODE_SESSION/LEETCODE_CSRF env vars are set but failed verification; falling through to cached / interactive login")
	}

	if c, err := auth.Load(); err == nil {
		if status, accepted := tryVerify(ctx, c); accepted {
			return c, status, nil
		}
		// Stale cache. Delete so a future run doesn't repeat the same
		// failed verify before reaching the login TUI.
		_ = auth.Delete()
	}

	c, err := auth.RunLoginTUI(ctx, verifyForLoginTUI)
	if err != nil {
		return nil, leetcode.UserStatus{}, err
	}
	if err := auth.Save(c); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to cache credentials: %v\n", err)
	}
	// Re-Verify post-login to capture UserStatus (verifyForLoginTUI
	// discards it). Failures fall back to zero UserStatus — same
	// fail-closed treatment as the env / cache paths' offline behavior.
	status, _ := tryVerify(ctx, c)
	return c, status, nil
}

func credsFromEnv() *auth.Credentials {
	sess, csrf := os.Getenv("LEETCODE_SESSION"), os.Getenv("LEETCODE_CSRF")
	if sess == "" || csrf == "" {
		return nil
	}
	// Clear the env so child processes (notably $EDITOR via tea.ExecProcess
	// and any plugins it loads — LSPs, AI assistants, telemetry) don't
	// inherit the live session cookie via os.Environ().
	os.Unsetenv("LEETCODE_SESSION")
	os.Unsetenv("LEETCODE_CSRF")
	return &auth.Credentials{Session: sess, CSRF: csrf}
}

// tryVerify validates creds via a short-timeout Verify call. Returns
// the captured UserStatus (zero value on network failure or rejection)
// alongside an accepted flag — true when the creds are accepted OR when
// the failure looks network-related (so an offline run doesn't bounce
// the user into the login screen for no reason). Returns false only
// when leetcode.com affirmatively rejected the session.
func tryVerify(parent context.Context, c *auth.Credentials) (leetcode.UserStatus, bool) {
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	status, err := leetcode.NewClient(c).Verify(ctx)
	if err == nil {
		return status, true
	}
	if isNetworkError(err) {
		// Treat as accepted: we couldn't reach leetcode.com to ask. The
		// app will fail on its first real call if the cookies are
		// actually bad; the user can re-login then. Better than
		// stranding them in the login screen with no internet. The
		// zero-value UserStatus here means downstream code treats the
		// user as non-premium until the first successful Verify — a
		// premium user offline temporarily loses paid recommendations,
		// which is benign.
		return leetcode.UserStatus{}, true
	}
	return leetcode.UserStatus{}, false
}

// isNetworkError reports whether err is a "couldn't reach the server"
// rather than a "server rejected us" failure. We can't always tell
// them apart cleanly — Verify returns wrapped errors and the
// underlying transport doesn't expose a typed marker — so this is a
// best-effort heuristic over net.Error and DNS errors.
func isNetworkError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}

// verifyForLoginTUI is the closure RunLoginTUI calls to validate freshly
// captured cookies. Lives here (in main) so internal/auth doesn't need
// to import internal/leetcode. The auth package only cares whether
// creds work; the post-login UserStatus refresh happens in resolveCreds.
func verifyForLoginTUI(ctx context.Context, c *auth.Credentials) error {
	verifyCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := leetcode.NewClient(c).Verify(verifyCtx)
	return err
}
