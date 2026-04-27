# leetcode-anki

A Go TUI (Bubble Tea) for working through LeetCode problems: lists the logged-in user's favorites, walks through problems one by one, opens `$EDITOR` on a scaffolded solution file, and runs / submits via LeetCode's `interpret_solution` and `/submit/` endpoints.

The canonical domain vocabulary lives in [CONTEXT.md](CONTEXT.md). Read it before naming anything new and follow the resolutions there (`Problem` not Question, `Problem List` not FavoriteList, `Solution` not Draft, `Verdict` not Result, plus the spaced-repetition terms `Review` / `Review Mode` / `Explore Mode`). When a domain term is missing or fuzzy, sharpen it in `CONTEXT.md` rather than coining a synonym in code.

## Layout

- `cmd/leetcode-anki/main.go` — single binary entry. Flags: `--logout` (forces re-auth via chromedp).
- `internal/auth/` — chromedp browser login + creds cache at `os.UserConfigDir()/leetcode-anki/creds.json` (mode `0600`).
- `internal/leetcode/` — GraphQL + REST client. **All headers (`Cookie`, `x-csrftoken`, `Referer`) centralized in `client.setHeaders`** — never set them anywhere else. Run/submit poll `/submissions/detail/{id}/check/` until `state == "SUCCESS"`.
- `internal/render/html.go` — HTML → markdown for problem descriptions.
- `internal/editor/` — solution scaffolding under `os.UserCacheDir()/leetcode-anki/<slug>/solution.<ext>` + `tea.ExecProcess` editor invocation. Existing files are never overwritten so users can resume.
- `internal/tui/` — Bubble Tea root + four screens: `lists → problems → problem → result`. Sub-models compose under one root model with a `screen` enum.

## Build / run

```
go run ./cmd/leetcode-anki    # full TUI
go test ./...                 # tests
go vet ./... && go build ./...
```

## Working notes (non-obvious)

- LeetCode's GraphQL schema churns. `MyFavoriteLists` merges `myCreatedFavoriteList` and `favoritesLists.allFavorites` because either may be the source of a given list. When fields disappear (e.g. `nameTranslated` was removed from `TopicTagNode`), update both queries.
- For run/submit, `question_id` is the hidden numeric ID (`ProblemDetail.QuestionID`), **not** `questionFrontendId`.
- POSTs to `interpret_solution` and `submit` require `Referer: https://leetcode.com/problems/{slug}/`. Already handled in `client.setHeaders`; don't bypass.
- Bubble Tea + `tea.ExecProcess`: never wrap an exec command in `tea.Sequence` to set state first. The state-setting message is not reliably delivered to `Update` before exec takes the terminal. Set state synchronously in `Update`, then return only the exec `tea.Cmd`.
- Problem-screen language picker reads from `ProblemDetail.CodeSnippets`. Scaffold path uses `editor.Ext(langSlug)`; unknown langs fall back to `.txt`.

## Conventions

### Commit discipline

- **Never commit without explicit user approval.** Stage the change, show the diff, propose the commit message, and wait for the user to say "commit" (or equivalent). This holds even when commits were pre-planned in a plan file — surface each one for approval before running `git commit`. Don't auto-commit because "the plan listed it."
- **Commits MUST be atomic** — one logical change per commit. The project must be in a working state at every commit: `go vet ./...`, `go build ./...`, and `go test ./...` all pass.
- **Use conventional commits** for the subject: `feat:`, `fix:`, `refactor:`, `chore:`, `test:`, `docs:`, `perf:`, `style:`, `ci:`. Optional scope: `feat(tui): ...`, `fix(leetcode): ...`.
- Subject ≤ 72 chars, imperative mood ("add X", not "added X"). Body explains *why* when not obvious from the diff.
- Never `--amend` a commit that's already been pushed. Make a new commit instead.
- Don't bundle unrelated cleanups into a feature commit; split them.

### Comments

Follow the `comment` skill rules (based on Ousterhout's *A Philosophy of Software Design*):

- **Comment WHY, not WHAT** — well-named identifiers already say what. Add a comment only when removing it would leave a future reader confused: a hidden constraint, a subtle invariant, a workaround for a specific bug, behavior that would surprise.
- **Exported symbols get one-line godoc** starting with the symbol name (`// Client is …`, `// Submit posts …`).
- Comments belong on the **interface** side (callers care) more than the implementation side. Don't leak implementation details into doc comments.
- **No PR-/task-flavored comments**: not `// added for the X flow`, not `// used by Y`, not `// fix for issue #123`. They rot. That belongs in commit messages.
- Don't restate the code. If the comment paraphrases the next line, delete it.
- Run `/comment` on a file when adding non-trivial new code or before a review pass.

### Tests

- **All new features and fixes MUST be implemented using TDD** — red → green → refactor, as described in the `tdd` skill. Write a failing test first, watch it fail with the expected error, write the minimum code to make it pass, then refactor with the test as the safety net. No "I'll add tests after." If the change is genuinely untestable (e.g. a CSS-only tweak, a one-character typo), say so explicitly before skipping the loop.
- **Use interfaces at every external boundary** so tests can inject fakes. Concretely:
  - HTTP doer: `pollCheck` and `doREST`/`doGraphQL` should depend on a small `httpDoer interface { Do(*http.Request) (*http.Response, error) }`, not `*http.Client` directly. The production `Client` holds a `*http.Client`; tests pass a fake.
  - Filesystem and editor: the TUI depends on `SolutionCache` and `Editor` interfaces (`internal/tui/client.go`), not on package-level functions in `internal/editor`, so the edit flow can be exercised against in-memory fakes without writing to a real cache directory or spawning `$EDITOR`.
  - LeetCode API: the TUI depends on a `LeetcodeClient` interface (subset of `*leetcode.Client`'s public methods), not the concrete type, so screen logic can be exercised with canned responses.
- **`go test ./...` must pass before every commit.** No skipping with `t.Skip` for "I'll fix it later".
- New code carrying real branching logic — state machines, merge/dedup, schema decoders, retry/backoff — ships with tests in the same commit.
- Tests live next to the code (`run_test.go` next to `run.go`). Integration-style tests that hit real LeetCode go in a build-tagged file (`//go:build integration`) and are off by default.

#### Live contract test against LeetCode

The live contract exercises every `*leetcode.Client` method — reads, run, submit, and note updates — against the real LeetCode API. Both fake-side (`TestContract_Fake`, default suite) and live-side (`TestContract_Live`, `integration` build tag) runs share one `contracttest.ContractTest` definition; if the live side starts failing, LeetCode changed something user-visible.

Use a dedicated test account so writes don't leak into your personal profile. One-time setup:

1. Create a fresh leetcode.com account (don't reuse your personal one).
2. Add the "Two Sum" problem to that account's "Favorite Questions" list.
3. Run `go run ./cmd/leetcode-test-login` and complete login as the test account. Cookies land in `<UserConfigDir>/leetcode-anki/test-creds.json`.

Run the live contract:

```
go test -tags integration ./internal/leetcode/...
```

Each run submits the fixture's known passing solution to LeetCode's judge, which adds an entry to the test account's submission history. That's expected — the test account exists to absorb that — but don't run the live contract on a tight loop. Re-run `leetcode-test-login` when the session cookie expires.

CI alternative: set `LEETCODE_TEST_SESSION` and `LEETCODE_TEST_CSRF` env vars; `contracttest.LoadTestCreds` prefers env over the file when both are present.

##### CI: refreshing the GitHub Actions secrets

The `.github/workflows/live-contract.yml` action runs the live contract on every push to `main`, on PRs that touch `internal/leetcode/**` or `internal/auth/**`, and on a daily 07:00 UTC cron. It reads `LEETCODE_TEST_SESSION` and `LEETCODE_TEST_CSRF` from repo secrets.

LeetCode session cookies expire every few weeks, after which the action will fail with auth errors. To refresh:

1. Run `go run ./cmd/leetcode-test-login` locally and complete the browser login as the test account. The tool prints the path it wrote to (`<UserConfigDir>/leetcode-anki/test-creds.json` — on macOS that's `~/Library/Application Support/leetcode-anki/test-creds.json`).
2. `cat` that file to read out the JSON; copy the `session` and `csrf` values.
3. In the GitHub repo: **Settings → Secrets and variables → Actions** → update `LEETCODE_TEST_SESSION` (the `session` value) and `LEETCODE_TEST_CSRF` (the `csrf` value).
4. Re-run the failed workflow.
