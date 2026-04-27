package auth

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// BrowserLogin opens an interactive Chrome window pointed at the LeetCode
// login page and waits for the user to complete login. On detection of a
// successful redirect off /accounts/login/, it extracts the
// LEETCODE_SESSION + csrftoken cookies and returns them. The caller is
// responsible for persisting the result via Save or SaveToPath.
//
// Times out after 5 minutes if the user never completes the login.
func BrowserLogin(ctx context.Context) (*Credentials, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", false),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	bctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	bctx, cancelTimeout := context.WithTimeout(bctx, 5*time.Minute)
	defer cancelTimeout()

	var creds Credentials

	err := chromedp.Run(bctx,
		chromedp.Navigate("https://leetcode.com/accounts/login/"),

		chromedp.ActionFunc(func(ctx context.Context) error {
			fmt.Fprintln(os.Stderr, "Please log in via the browser window...")
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
					var url string
					if err := chromedp.Location(&url).Do(ctx); err != nil {
						continue
					}
					if !strings.Contains(url, "/accounts/login/") {
						fmt.Fprintln(os.Stderr, "Login detected, extracting cookies...")
						return nil
					}
				}
			}
		}),

		chromedp.ActionFunc(func(ctx context.Context) error {
			cookies, err := network.GetCookies().WithURLs([]string{"https://leetcode.com"}).Do(ctx)
			if err != nil {
				return err
			}
			for _, c := range cookies {
				switch c.Name {
				case "LEETCODE_SESSION":
					creds.Session = c.Value
				case "csrftoken":
					creds.CSRF = c.Value
				}
			}
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("browser flow: %w", err)
	}
	if creds.Session == "" || creds.CSRF == "" {
		return nil, fmt.Errorf("missing cookies after login (session-set=%v csrf-set=%v)",
			creds.Session != "", creds.CSRF != "")
	}

	return &creds, nil
}

// GetCredentials returns cached credentials if available, otherwise launches the
// interactive browser login flow and caches the result.
func GetCredentials(ctx context.Context) (*Credentials, error) {
	if c, err := Load(); err == nil {
		return c, nil
	}

	c, err := BrowserLogin(ctx)
	if err != nil {
		return nil, err
	}

	if err := Save(c); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to cache credentials: %v\n", err)
	}

	return c, nil
}

// ForceLogin discards any cached credentials and re-runs the browser flow.
func ForceLogin(ctx context.Context) (*Credentials, error) {
	_ = Delete()
	c, err := BrowserLogin(ctx)
	if err != nil {
		return nil, err
	}
	if err := Save(c); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to cache credentials: %v\n", err)
	}
	return c, nil
}
