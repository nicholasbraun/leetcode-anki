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
func TestVerify_SignedInReturnsNil(t *testing.T) {
	d := &capturingDoer{resp: `{"data":{"userStatus":{"isSignedIn":true}}}`}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	if err := c.Verify(context.Background()); err != nil {
		t.Errorf("Verify with isSignedIn=true: %v, want nil", err)
	}
}

func TestVerify_NotSignedInReturnsError(t *testing.T) {
	d := &capturingDoer{resp: `{"data":{"userStatus":{"isSignedIn":false}}}`}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	if err := c.Verify(context.Background()); err == nil {
		t.Error("Verify with isSignedIn=false: got nil, want error")
	}
}

// Verify must use the exact GraphQL field LeetCode exposes. If the field
// path drifts (e.g. userStatus moves under a different root), Verify
// silently passes garbage. Pin the query.
func TestVerify_SendsUserStatusQuery(t *testing.T) {
	d := &capturingDoer{resp: `{"data":{"userStatus":{"isSignedIn":true}}}`}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	if err := c.Verify(context.Background()); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	body := string(d.body)
	if !strings.Contains(body, "userStatus") || !strings.Contains(body, "isSignedIn") {
		t.Errorf("request body missing userStatus/isSignedIn: %s", body)
	}
}

func TestVerify_SurfacesTransportError(t *testing.T) {
	d := &fixedDoer{status: 403, body: ""}
	c := newClientWithDoer(&auth.Credentials{Session: "s", CSRF: "c"}, d)

	if err := c.Verify(context.Background()); err == nil {
		t.Error("Verify against 403: got nil, want error")
	}
}
