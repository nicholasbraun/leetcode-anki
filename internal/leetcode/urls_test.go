package leetcode

import "testing"

// validSlug rejects anything outside [a-z0-9-] today, so these helpers never
// see a "weird" slug in production. The escape-pinning test guards against
// the slug regex being loosened later: if someone ever lets `?` or `/` in,
// PathEscape catches it before it can split a URL into a path + injected
// query string.
func TestURLBuildersEscapeSlugs(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"problemRef", problemRefURL("two-sum"), "https://leetcode.com/problems/two-sum/"},
		{"interpret", interpretURL("two-sum"), "https://leetcode.com/problems/two-sum/interpret_solution/"},
		{"submit", submitURL("two-sum"), "https://leetcode.com/problems/two-sum/submit/"},
		{"submissionCheck", submissionCheckURL("12345"), "https://leetcode.com/submissions/detail/12345/check/"},
		{"submissionsRef", submissionsRefURL("two-sum"), "https://leetcode.com/problems/two-sum/submissions/"},
		{"problemListRef", problemListRefURL("top-100"), "https://leetcode.com/problem-list/top-100/"},

		// If validSlug is ever relaxed, PathEscape must still neutralise the
		// query-injection vector. "?evil=1" would otherwise turn the URL into
		// a different request (path "/" + query string evil=1).
		{"escapesQuery", problemRefURL("a?evil=1"), "https://leetcode.com/problems/a%3Fevil=1/"},
		{"escapesSlash", interpretURL("a/b"), "https://leetcode.com/problems/a%2Fb/interpret_solution/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}
