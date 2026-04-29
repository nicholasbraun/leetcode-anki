package auth

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// LoginFromPaste reads two lines — LEETCODE_SESSION value, then csrftoken
// value — from in and returns them as Credentials. Prompts are written
// to out. Used as the user-facing fallback when browser cookie extraction
// fails or is opted out of.
//
// Rejects empty values, EOF before the csrftoken line, and inputs shaped
// like a Cookie header ("LEETCODE_SESSION=…; csrftoken=…") — the latter
// is a common copy-from-devtools mistake that would otherwise silently
// bake the literal "LEETCODE_SESSION=abc" string into Credentials.Session.
func LoginFromPaste(in io.Reader, out io.Writer) (*Credentials, error) {
	r := bufio.NewReader(in)

	fmt.Fprintln(out, "Open https://leetcode.com in a logged-in browser, then in devtools:")
	fmt.Fprintln(out, "  Application/Storage → Cookies → https://leetcode.com")
	fmt.Fprintln(out)

	session, err := readNonEmptyLine(r, out, "LEETCODE_SESSION value: ")
	if err != nil {
		return nil, fmt.Errorf("read session: %w", err)
	}
	csrf, err := readNonEmptyLine(r, out, "csrftoken value:        ")
	if err != nil {
		return nil, fmt.Errorf("read csrf: %w", err)
	}

	if err := rejectCookieHeaderShape(session, csrf); err != nil {
		return nil, err
	}

	return &Credentials{Session: session, CSRF: csrf}, nil
}

func readNonEmptyLine(r *bufio.Reader, out io.Writer, prompt string) (string, error) {
	fmt.Fprint(out, prompt)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	v := strings.TrimSpace(line)
	if v == "" {
		return "", fmt.Errorf("empty value")
	}
	return v, nil
}

func rejectCookieHeaderShape(session, csrf string) error {
	for _, v := range []string{session, csrf} {
		// "=" alone catches both "LEETCODE_SESSION=abc" and the full header
		// "LEETCODE_SESSION=abc; csrftoken=def". Real cookie values are
		// opaque base64-ish strings without "=" as a separator (they may
		// contain "=" as base64 padding, but never adjacent to a key name).
		if i := strings.IndexByte(v, '='); i > 0 && isLikelyCookieKey(v[:i]) {
			return fmt.Errorf("input looks like a Cookie header (%q); paste the bare value, not the key=value pair", v)
		}
	}
	return nil
}

func isLikelyCookieKey(s string) bool {
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	return len(s) > 0
}
