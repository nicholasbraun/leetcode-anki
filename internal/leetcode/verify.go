package leetcode

import (
	"context"
	"encoding/json"
	"fmt"
)

const verifyQuery = `query userStatus { userStatus { isSignedIn } }`

// Verify confirms the configured credentials still authenticate against
// leetcode.com. Used post-login to fail fast on a bad paste or stale
// cookie before writing creds.json.
func (c *Client) Verify(ctx context.Context) error {
	raw, err := c.doGraphQL(ctx, "userStatus", verifyQuery, nil, BaseURL)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	var resp struct {
		UserStatus struct {
			IsSignedIn bool `json:"isSignedIn"`
		} `json:"userStatus"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("verify: decode: %w", err)
	}
	if !resp.UserStatus.IsSignedIn {
		return fmt.Errorf("verify: leetcode.com reports the session is not signed in")
	}
	return nil
}
