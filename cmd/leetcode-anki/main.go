package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"leetcode-anki/internal/auth"
	"leetcode-anki/internal/leetcode"
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

	if err := tui.Run(ctx, client); err != nil {
		fmt.Fprintf(os.Stderr, "tui: %v\n", err)
		os.Exit(1)
	}
}
