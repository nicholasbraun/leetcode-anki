package auth

import (
	"context"
	"fmt"
	"io"

	"github.com/browserutils/kooky"

	// Register only the browsers in browserPriority. Importing
	// browser/all pulls in IE/browsh/dillo/lynx/etc. and a long tail
	// of transitive deps (ese parser, sqlite drivers) we don't need.
	_ "github.com/browserutils/kooky/browser/brave"
	_ "github.com/browserutils/kooky/browser/chrome"
	_ "github.com/browserutils/kooky/browser/edge"
	_ "github.com/browserutils/kooky/browser/firefox"
	_ "github.com/browserutils/kooky/browser/safari"
)

// KookyFinder is the production cookieFinder. It uses the
// browserutils/kooky library to read leetcode.com cookies from every
// supported browser's local cookie store (Firefox SQLite, Chrome's
// OS-keyring-encrypted SQLite, Safari's binarycookies file, etc.) without
// launching a browser process.
type KookyFinder struct {
	// DebugLog, if non-nil, receives a diagnostic dump of every cookie
	// store kooky enumerated and every leetcode.com cookie it returned
	// — including ones that don't form a usable pair. Used to debug
	// "browser extraction fell through to paste even though I'm logged
	// in" situations: most often a wrong-Firefox-profile or
	// expired-cookie issue.
	DebugLog io.Writer
}

// FindLeetcodeCookies returns every LEETCODE_SESSION and csrftoken cookie
// for any leetcode.com host across every registered browser. The result
// is unsorted — callers (LoginFromBrowser) apply the browser priority
// order.
func (k KookyFinder) FindLeetcodeCookies(ctx context.Context) ([]browserCookie, error) {
	if k.DebugLog != nil {
		k.dumpDiscovery(ctx)
	}

	cookies, err := kooky.ReadCookies(ctx,
		kooky.Valid,
		kooky.DomainHasSuffix("leetcode.com"),
	)
	if k.DebugLog != nil {
		k.dumpCookies(cookies, err)
	}
	if err != nil && len(cookies) == 0 {
		return nil, err
	}

	out := make([]browserCookie, 0, len(cookies))
	for _, c := range cookies {
		if c == nil || c.Browser == nil {
			continue
		}
		if c.Name != "LEETCODE_SESSION" && c.Name != "csrftoken" {
			continue
		}
		out = append(out, browserCookie{
			Browser: c.Browser.Browser(),
			Name:    c.Name,
			Value:   c.Value,
			Expires: c.Expires,
		})
	}
	return out, nil
}

// dumpDiscovery lists every cookie store kooky's registered finders
// turned up. This catches the most common "logged into Firefox but
// extraction fails" cause: kooky reads the wrong Firefox profile (the
// stale Default=1 one in profiles.ini, not the active per-install one).
func (k KookyFinder) dumpDiscovery(ctx context.Context) {
	fmt.Fprintln(k.DebugLog, "## cookie stores discovered by kooky")
	stores := kooky.FindAllCookieStores(ctx)
	if len(stores) == 0 {
		fmt.Fprintln(k.DebugLog, "  (none)")
		return
	}
	for _, s := range stores {
		fmt.Fprintf(k.DebugLog, "  browser=%s profile=%s default=%t path=%s\n",
			s.Browser(), s.Profile(), s.IsDefaultProfile(), s.FilePath())
	}
}

// dumpCookies records every leetcode.com cookie kooky returned, plus the
// aggregated per-store error so the user can see why some browsers were
// skipped. Cookie *values* are length-only — they're live credentials.
func (k KookyFinder) dumpCookies(cookies kooky.Cookies, err error) {
	fmt.Fprintln(k.DebugLog, "\n## leetcode.com cookies returned by kooky (Valid filter applied)")
	if len(cookies) == 0 {
		fmt.Fprintln(k.DebugLog, "  (none)")
	}
	for _, c := range cookies {
		if c == nil {
			continue
		}
		browser := "(nil)"
		profile := ""
		if c.Browser != nil {
			browser = c.Browser.Browser()
			profile = c.Browser.Profile()
		}
		fmt.Fprintf(k.DebugLog, "  browser=%s profile=%s name=%s domain=%s value-len=%d expires=%s\n",
			browser, profile, c.Name, c.Domain, len(c.Value), c.Expires)
	}
	if err != nil {
		fmt.Fprintf(k.DebugLog, "\n## kooky aggregated error\n%v\n", err)
	}
}
