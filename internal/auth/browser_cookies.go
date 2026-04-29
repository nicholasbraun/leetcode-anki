package auth

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// browserCookie is the minimum we need from a browser-stored cookie.
// Decoupled from kooky.Cookie so the LoginFromBrowser logic can be
// tested without a kooky-shaped fake.
type browserCookie struct {
	Browser string // "firefox", "chrome", "safari", "edge", "brave"
	Name    string
	Value   string
	// Expires is the cookie's wall-clock expiry. Zero means "session
	// cookie" (no expiry — gone when the browser closes), which is treated
	// as valid here.
	Expires time.Time
}

// cookieFinder hands back leetcode.com cookies from the user's local
// browser cookie stores. The production implementation is backed by
// kooky; tests inject a function-shaped fake.
type cookieFinder interface {
	FindLeetcodeCookies(ctx context.Context) ([]browserCookie, error)
}

// ErrNoBrowserCookies is returned when no leetcode.com cookies were
// found in any browser. Callers (main.go) detect this with errors.Is to
// fall through to the paste flow without surfacing a confusing message.
var ErrNoBrowserCookies = errors.New("no leetcode.com cookies found in any local browser")

// browserPriority orders the browsers we try when more than one has a
// complete LEETCODE_SESSION + csrftoken pair. Firefox first because it
// doesn't require the macOS Keychain prompt that Chrome triggers.
var browserPriority = []string{"firefox", "chrome", "safari", "edge", "brave"}

// LoginFromBrowser finds a LEETCODE_SESSION + csrftoken pair from the
// user's local browser cookie stores and returns them as Credentials.
// Both cookies must come from the same browser; a session from one and
// csrftoken from another won't authenticate against leetcode.com.
//
// Returns ErrNoBrowserCookies (sentinel) when no leetcode.com cookies
// exist anywhere — callers can fall through to LoginFromPaste in that
// case. Other errors (a complete pair existed but cookies were stale,
// finder I/O failed, etc.) are returned wrapped and should be surfaced.
func LoginFromBrowser(ctx context.Context, finder cookieFinder) (*Credentials, error) {
	cookies, _ := finder.FindLeetcodeCookies(ctx)
	// kooky aggregates per-store errors (browsers not installed, Safari
	// sandbox without Full Disk Access, Chrome keychain decryption
	// failures for unrelated cookies) but still returns the cookies it
	// managed to read. The errors are noise — every machine has some
	// browsers that don't exist locally. We treat "no leetcode.com
	// cookies came back" as the only signal worth acting on, and fall
	// through silently to paste. The user is about to paste anyway; a
	// wall of "no such file" lines doesn't help them.
	if len(cookies) == 0 {
		return nil, ErrNoBrowserCookies
	}

	now := time.Now()
	type pair struct{ session, csrf string }
	byBrowser := map[string]*pair{}
	for _, c := range cookies {
		if !c.Expires.IsZero() && c.Expires.Before(now) {
			continue
		}
		p, ok := byBrowser[c.Browser]
		if !ok {
			p = &pair{}
			byBrowser[c.Browser] = p
		}
		switch c.Name {
		case "LEETCODE_SESSION":
			p.session = c.Value
		case "csrftoken":
			p.csrf = c.Value
		}
	}

	for _, b := range browserPriority {
		p, ok := byBrowser[b]
		if !ok || p.session == "" || p.csrf == "" {
			continue
		}
		return &Credentials{Session: p.session, CSRF: p.csrf}, nil
	}

	return nil, fmt.Errorf("found leetcode.com cookies but no single browser had a complete (LEETCODE_SESSION + csrftoken) pair that wasn't expired")
}
