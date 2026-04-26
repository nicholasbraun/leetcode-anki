package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"leetcode-anki/internal/auth"
	"leetcode-anki/internal/editor"
	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/sr"
	"leetcode-anki/internal/tui"
)

func main() {
	logout := flag.Bool("logout", false, "delete cached credentials and re-run the browser login")
	flag.Parse()

	ctx := context.Background()

	var (
		creds *auth.Credentials
		err   error
	)
	if *logout {
		creds, err = auth.ForceLogin(ctx)
	} else {
		creds, err = auth.GetCredentials(ctx)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "auth: %v\n", err)
		os.Exit(1)
	}

	client := leetcode.NewClient(creds)
	cache := editor.NewCache()
	runner := editor.NewRunner()

	reviews, err := sr.Open(client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sr: %v\n", err)
		os.Exit(1)
	}

	if err := tui.Run(ctx, client, cache, runner, reviews); err != nil {
		fmt.Fprintf(os.Stderr, "tui: %v\n", err)
		os.Exit(1)
	}
}
