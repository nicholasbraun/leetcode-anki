package sr

import (
	"fmt"
	"regexp"
	"strings"
)

// tagRE matches [anki:N] with N in 1-4. The regex is intentionally strict
// on N (no leading zeros, no out-of-range digits) so a typo like [anki:0]
// or [anki:5] is treated as "no rating" rather than silently misparsed.
var tagRE = regexp.MustCompile(`\[anki:([1-4])\]`)

// parseTag extracts the SR rating embedded in a submission note. Returns
// (0, false) when no valid tag is present — caller should fall back to
// implicit "Accepted = Good" rating.
func parseTag(note string) (int, bool) {
	m := tagRE.FindStringSubmatch(note)
	if m == nil {
		return 0, false
	}
	return int(m[1][0] - '0'), true
}

// stripTag removes every [anki:N] from the note, plus any trailing
// whitespace that the removal exposes. Used before re-applying a fresh
// tag so the note doesn't accumulate prior grades.
func stripTag(note string) string {
	out := tagRE.ReplaceAllString(note, "")
	return strings.TrimRight(out, " \t\n\r")
}

// applyTag returns the user's note with [anki:N] appended on a new line,
// having stripped any prior tag first. The user's free-text prefix is
// preserved so SR writes don't clobber human notes.
func applyTag(note string, rating int) string {
	base := stripTag(note)
	tag := fmt.Sprintf("[anki:%d]", rating)
	if base == "" {
		return tag
	}
	return base + "\n" + tag
}
