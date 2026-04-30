package leetcode

import (
	"context"
	"encoding/json"
	"fmt"
)

const verifyQuery = `query userStatus { userStatus { isSignedIn isPremium } }`

// UserStatus is the LeetCode-side view of the authenticated session.
// Returned by Verify and threaded through to features that gate on the
// user's plan (today: skipping paid Problems from Review-Mode
// recommendations for free users).
type UserStatus struct {
	IsSignedIn bool
	// IsPremium is the active LeetCode subscription flag. Defaults to
	// false when the wire field is missing (schema drift) — fail-closed
	// avoids over-recommending paid Problems to free users.
	IsPremium bool
}

// Verify confirms the configured credentials still authenticate against
// leetcode.com and returns the user's plan status. Used post-login to
// fail fast on a bad paste or stale cookie before writing creds.json,
// and at startup to capture IsPremium for downstream filtering.
func (c *Client) Verify(ctx context.Context) (UserStatus, error) {
	raw, err := c.doGraphQL(ctx, "userStatus", verifyQuery, nil, BaseURL)
	if err != nil {
		return UserStatus{}, fmt.Errorf("verify: %w", err)
	}
	var resp struct {
		UserStatus struct {
			IsSignedIn bool `json:"isSignedIn"`
			IsPremium  bool `json:"isPremium"`
		} `json:"userStatus"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return UserStatus{}, fmt.Errorf("verify: decode: %w", err)
	}
	if !resp.UserStatus.IsSignedIn {
		return UserStatus{}, fmt.Errorf("verify: leetcode.com reports the session is not signed in")
	}
	return UserStatus{
		IsSignedIn: resp.UserStatus.IsSignedIn,
		IsPremium:  resp.UserStatus.IsPremium,
	}, nil
}
