package leetcode

import (
	"context"
	"strings"
	"testing"

	"leetcode-anki/internal/auth"
)

// Verify is the post-login probe: it confirms the cookies actually
// authenticate against leetcode.com before main writes them to disk.
// The probe must hit the same auth path as real requests so a session
// that LeetCode rejects is caught at login time, not when the user
// tries to load their problem list.
func TestVerify_SignedInReturnsStatus(t *testing.T) {
	d := &capturingDoer{resp: `{"data":{"userStatus":{"isSignedIn":true,"isPremium":false}}}`}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	status, err := c.Verify(context.Background())
	if err != nil {
		t.Errorf("Verify with isSignedIn=true: %v, want nil", err)
	}
	if !status.IsSignedIn {
		t.Errorf("IsSignedIn = false, want true")
	}
}

func TestVerify_NotSignedInReturnsError(t *testing.T) {
	d := &capturingDoer{resp: `{"data":{"userStatus":{"isSignedIn":false,"isPremium":false}}}`}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	if _, err := c.Verify(context.Background()); err == nil {
		t.Error("Verify with isSignedIn=false: got nil, want error")
	}
}

// Premium status is decoded from the same userStatus blob as IsSignedIn.
// Free and Premium accounts must round-trip distinctly so the TUI can
// gate paid-Problem recommendations on the user's actual plan.
func TestVerify_DecodesIsPremium(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{"premium", `{"data":{"userStatus":{"isSignedIn":true,"isPremium":true}}}`, true},
		{"free", `{"data":{"userStatus":{"isSignedIn":true,"isPremium":false}}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &capturingDoer{resp: tc.body}
			c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)
			status, err := c.Verify(context.Background())
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if status.IsPremium != tc.want {
				t.Errorf("IsPremium = %v, want %v", status.IsPremium, tc.want)
			}
		})
	}
}

// When LeetCode's response omits isPremium (older schema, field renamed,
// undocumented field-removal), the zero value must be false — the
// fail-closed default. A premium-default would over-recommend paid
// Problems to free users.
func TestVerify_MissingIsPremiumDefaultsToFalse(t *testing.T) {
	d := &capturingDoer{resp: `{"data":{"userStatus":{"isSignedIn":true}}}`}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	status, err := c.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if status.IsPremium {
		t.Error("missing isPremium should decode as false, not true (fail-closed)")
	}
}

// Verify must use the exact GraphQL fields LeetCode exposes. If a field
// path drifts (e.g. userStatus moves under a different root, or isPremium
// is renamed), Verify silently passes garbage. Pin the query.
func TestVerify_SendsUserStatusQuery(t *testing.T) {
	d := &capturingDoer{resp: `{"data":{"userStatus":{"isSignedIn":true,"isPremium":false}}}`}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	if _, err := c.Verify(context.Background()); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	body := string(d.body)
	for _, want := range []string{"userStatus", "isSignedIn", "isPremium"} {
		if !strings.Contains(body, want) {
			t.Errorf("request body missing %q: %s", want, body)
		}
	}
}

func TestVerify_SurfacesTransportError(t *testing.T) {
	d := &fixedDoer{status: 403, body: ""}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	if _, err := c.Verify(context.Background()); err == nil {
		t.Error("Verify against 403: got nil, want error")
	}
}
