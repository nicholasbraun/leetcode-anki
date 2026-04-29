package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func cookie(browser, name, value string) browserCookie {
	return browserCookie{
		Browser: browser,
		Name:    name,
		Value:   value,
		Expires: time.Now().Add(24 * time.Hour),
	}
}

func fakeFinder(cookies []browserCookie, err error) cookieFinder {
	return cookieFinderFunc(func(_ context.Context) ([]browserCookie, error) {
		return cookies, err
	})
}

func TestLoginFromBrowser_BothCookiesFromOneBrowser(t *testing.T) {
	finder := fakeFinder([]browserCookie{
		cookie("firefox", "LEETCODE_SESSION", "sess-ff"),
		cookie("firefox", "csrftoken", "csrf-ff"),
	}, nil)

	got, err := LoginFromBrowser(context.Background(), finder)
	if err != nil {
		t.Fatalf("LoginFromBrowser: %v", err)
	}
	if got.Session != "sess-ff" || got.CSRF != "csrf-ff" {
		t.Errorf("got %+v, want sess-ff/csrf-ff", got)
	}
}

// Both cookies must come from the *same* browser. A session from Firefox
// and a csrftoken from Chrome won't authenticate — LeetCode binds them.
func TestLoginFromBrowser_RefusesMixedBrowsers(t *testing.T) {
	finder := fakeFinder([]browserCookie{
		cookie("firefox", "LEETCODE_SESSION", "sess-ff"),
		cookie("chrome", "csrftoken", "csrf-ch"),
	}, nil)

	if _, err := LoginFromBrowser(context.Background(), finder); err == nil {
		t.Error("expected error when no single browser has both cookies; got nil")
	}
}

// Browser priority is Firefox → Chrome → Safari → Edge → Brave. When
// multiple browsers each have a complete pair, prefer the higher-priority
// one. The user picked this order because Firefox doesn't trigger a
// macOS Keychain prompt; Chrome does.
func TestLoginFromBrowser_PrefersFirefoxOverChrome(t *testing.T) {
	finder := fakeFinder([]browserCookie{
		cookie("chrome", "LEETCODE_SESSION", "sess-ch"),
		cookie("chrome", "csrftoken", "csrf-ch"),
		cookie("firefox", "LEETCODE_SESSION", "sess-ff"),
		cookie("firefox", "csrftoken", "csrf-ff"),
	}, nil)

	got, err := LoginFromBrowser(context.Background(), finder)
	if err != nil {
		t.Fatalf("LoginFromBrowser: %v", err)
	}
	if got.Session != "sess-ff" {
		t.Errorf("got session %q, want firefox's sess-ff (priority order)", got.Session)
	}
}

func TestLoginFromBrowser_FallsThroughToLowerPriorityBrowser(t *testing.T) {
	finder := fakeFinder([]browserCookie{
		// Firefox has only the session; no csrftoken.
		cookie("firefox", "LEETCODE_SESSION", "sess-ff"),
		// Chrome has the complete pair.
		cookie("chrome", "LEETCODE_SESSION", "sess-ch"),
		cookie("chrome", "csrftoken", "csrf-ch"),
	}, nil)

	got, err := LoginFromBrowser(context.Background(), finder)
	if err != nil {
		t.Fatalf("LoginFromBrowser: %v", err)
	}
	if got.Session != "sess-ch" {
		t.Errorf("got session %q, want chrome's sess-ch (firefox pair was incomplete)", got.Session)
	}
}

func TestLoginFromBrowser_NoLeetcodeCookies(t *testing.T) {
	finder := fakeFinder(nil, nil)

	_, err := LoginFromBrowser(context.Background(), finder)
	if err == nil {
		t.Fatal("expected error when no leetcode.com cookies anywhere; got nil")
	}
	if !errors.Is(err, ErrNoBrowserCookies) {
		t.Errorf("err = %v, want ErrNoBrowserCookies (sentinel) so callers can fall through to paste", err)
	}
}

func TestLoginFromBrowser_OnlySessionPresent(t *testing.T) {
	finder := fakeFinder([]browserCookie{
		cookie("firefox", "LEETCODE_SESSION", "sess-ff"),
	}, nil)

	if _, err := LoginFromBrowser(context.Background(), finder); err == nil {
		t.Error("expected error when csrftoken is missing everywhere; got nil")
	}
}

func TestLoginFromBrowser_SkipsExpiredCookies(t *testing.T) {
	expired := browserCookie{
		Browser: "firefox", Name: "LEETCODE_SESSION", Value: "stale",
		Expires: time.Now().Add(-1 * time.Hour),
	}
	finder := fakeFinder([]browserCookie{
		expired,
		cookie("firefox", "csrftoken", "csrf-ff"),
		cookie("chrome", "LEETCODE_SESSION", "sess-ch"),
		cookie("chrome", "csrftoken", "csrf-ch"),
	}, nil)

	got, err := LoginFromBrowser(context.Background(), finder)
	if err != nil {
		t.Fatalf("LoginFromBrowser: %v", err)
	}
	// Firefox's session is expired, so it doesn't have a complete fresh
	// pair. Chrome wins despite being lower priority.
	if got.Session != "sess-ch" {
		t.Errorf("got %q, want chrome's sess-ch (firefox session was expired)", got.Session)
	}
}

// kooky's aggregated errors are noise on most machines (no Brave,
// no Edge, Safari sandbox, etc.). When no leetcode cookies came back
// the user is going to paste anyway, so suppress the error wall and
// just fall through with the sentinel.
func TestLoginFromBrowser_NoCookiesAndErrorStillReturnsSentinel(t *testing.T) {
	finder := fakeFinder(nil, errors.New("kooky exploded"))

	_, err := LoginFromBrowser(context.Background(), finder)
	if !errors.Is(err, ErrNoBrowserCookies) {
		t.Errorf("err = %v, want ErrNoBrowserCookies even when finder errored", err)
	}
}

// kooky's ReadCookies returns BOTH cookies and an aggregated error
// containing every per-store failure: browsers the user doesn't have
// installed, profiles that don't exist, the macOS Safari sandbox
// (`operation not permitted` without Full Disk Access), Chrome's
// keychain decryption failures for non-leetcode cookies, etc. None of
// that should defeat extraction when the leetcode cookies we *do* care
// about came back fine. Regression: a logged-in Firefox profile was
// being ignored because Brave-not-installed errors aborted the flow.
func TestLoginFromBrowser_IgnoresFinderErrorsWhenCookiesPresent(t *testing.T) {
	finder := cookieFinderFunc(func(_ context.Context) ([]browserCookie, error) {
		return []browserCookie{
			cookie("firefox", "LEETCODE_SESSION", "sess-ff"),
			cookie("firefox", "csrftoken", "csrf-ff"),
		}, errors.New("Brave-Browser/Local State: no such file or directory")
	})

	got, err := LoginFromBrowser(context.Background(), finder)
	if err != nil {
		t.Fatalf("LoginFromBrowser must use the cookies it got, even with side errors: %v", err)
	}
	if got.Session != "sess-ff" || got.CSRF != "csrf-ff" {
		t.Errorf("got %+v, want firefox cookies extracted despite finder error", got)
	}
}

// Cookie.Expires == zero time means "session cookie" (browser-only,
// expires when the browser closes). LeetCode's csrftoken is sometimes
// returned this way. Treat zero as "not expired" — it only expires
// when the browser session ends, not on a wall-clock check.
func TestLoginFromBrowser_TreatsZeroExpiryAsValid(t *testing.T) {
	finder := fakeFinder([]browserCookie{
		{Browser: "firefox", Name: "LEETCODE_SESSION", Value: "sess", Expires: time.Now().Add(time.Hour)},
		{Browser: "firefox", Name: "csrftoken", Value: "csrf", Expires: time.Time{}},
	}, nil)

	got, err := LoginFromBrowser(context.Background(), finder)
	if err != nil {
		t.Fatalf("LoginFromBrowser: %v", err)
	}
	if got.CSRF != "csrf" {
		t.Errorf("got csrf=%q, want %q (zero expiry must not be treated as expired)", got.CSRF, "csrf")
	}
}

// cookieFinderFunc lets the tests pass a function literal where a
// cookieFinder is required.
type cookieFinderFunc func(context.Context) ([]browserCookie, error)

func (f cookieFinderFunc) FindLeetcodeCookies(ctx context.Context) ([]browserCookie, error) {
	return f(ctx)
}
