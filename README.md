# leetcode-anki

A terminal UI for working through LeetCode problems and retaining
what you've solved through spaced-repetition reviews. Walk a problem list, read
the description, write a solution in your `$EDITOR`, run it against the examples,
submit for a verdict. Once a problem is accepted it joins the SR rotation and
shows up in **Review Mode** when due.

## About

I built this for three reasons:

- **Stay sharp on DSA in the LLM era.** When an LLM can one-shot most of the
  coding work I do day-to-day, the part that decays fastest is the part I no
  longer practice: choosing the right data structure, recognising the shape of
  a problem, reasoning about complexity without an autocomplete in the way.
  LeetCode is the cheapest way to keep that muscle alive — and spaced
  repetition is the cheapest way to keep what I've already learned from
  fading.
- **Use Neovim, not the LeetCode web editor.** I don't want to write code in a
  browser textarea. I don't want to copy-paste solutions back and forth. The
  TUI hands the file to `$EDITOR` (Neovim, in my case) and ships the result
  straight to LeetCode's `interpret_solution` and `/submit/` endpoints — same
  verdicts, same submission history, no context switch.
- **Learn to work effectively with agentic coding tools.** This is also a
  deliberate exploration of how to drive Claude Code productively while still
  shipping code I'd be happy to put in front of a reviewer at work. The
  conventions in [CLAUDE.md](CLAUDE.md) — TDD on every change, atomic
  conventional commits, explicit per-commit approval, comments-on-the-WHY,
  interfaces at every external boundary, a [CONTEXT.md](CONTEXT.md) ubiquitous
  language — are the guardrails that keep an agent's output at production
  quality instead of "demoware that compiles."

## Setup

### Prerequisites

- Go 1.26.1+ (see [`go.mod`](go.mod))
- Google Chrome / Chromium installed locally — used by `chromedp` for the
  browser-based login flow
- A LeetCode account
- `$VISUAL` or `$EDITOR` set (falls back to `vi`)

### Build and run

```sh
git clone https://github.com/nicholasbraun/leetcode-anki && cd leetcode-anki
go run ./cmd/leetcode-anki        # run from source
# or
go build ./cmd/leetcode-anki      # produce ./leetcode-anki binary
./leetcode-anki
```

### First-run authentication

On first run a Chrome window opens at `https://leetcode.com/accounts/login/`.
Log in normally (password, OAuth, whatever your account uses); once the URL
leaves `/accounts/login/` the binary scrapes the `LEETCODE_SESSION` and
`csrftoken` cookies and caches them at:

```
$XDG_CONFIG_HOME/leetcode-anki/creds.json     # 0600
# or on macOS: ~/Library/Application Support/leetcode-anki/creds.json
```

To force re-auth (e.g. cookies expired):

```sh
leetcode-anki --logout
```

### On-disk layout

| Path                                                | Contents                                                                                 |
| --------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `$UserConfigDir/leetcode-anki/creds.json`           | session + CSRF cookies (mode 0600)                                                       |
| `$UserConfigDir/leetcode-anki/sr.json`              | spaced-repetition cache (per-slug submission timeline)                                   |
| `$UserCacheDir/leetcode-anki/<slug>/solution.<ext>` | scaffolded solution files; never overwritten on re-scaffold so resumed work is preserved |
| `$UserCacheDir/leetcode-anki/debug.log`             | raw GraphQL responses, only when `LEETCODE_DEBUG=1`                                      |

## Usage

The TUI walks four screens: `lists → problems → problem → result`.

### Keys

| Key                              | Action                                               |
| -------------------------------- | ---------------------------------------------------- |
| `↑`/`k`, `↓`/`j`                 | move                                                 |
| `enter`                          | select                                               |
| `esc` / `backspace`              | back (also cancels an in-flight run/submit)          |
| `q` / `ctrl+c`                   | quit                                                 |
| `e`                              | edit current problem's solution in `$EDITOR`         |
| `r`                              | run against example testcases (`interpret_solution`) |
| `s`                              | submit (`/submit/`)                                  |
| `l`                              | cycle language for the current problem               |
| `n` / `p`                        | next / previous problem in the current list          |
| `pgup`/`ctrl+u`, `pgdn`/`ctrl+d` | scroll the problem-list preview pane                 |
| `v`                              | enter Review Mode from the lists screen              |
| `?`                              | help                                                 |

### Explore Mode vs Review Mode

- **Explore Mode** (default on lists screen): the full problem list.
- **Review Mode** (`v` from lists): only problems currently due for review. The
  candidate set comes from your global LeetCode submission progress; due-date
  is computed locally by the SR scheduler.

A problem enters the SR rotation on its **first accepted submit** — runs do
not count. Subsequent attempts in any language fold into the same rotation
(the problem is the SR unit, not the `(problem, language)` pair).

### Grading an Accepted submit

When a submit comes back **Accepted**, a modal opens on the result screen
asking how confidently you solved it. Each option shows its predicted next
due-date so you can see what you're picking:

| Key | Rating                                  |
| --- | --------------------------------------- |
| `1` | Again                                   |
| `2` | Hard                                    |
| `3` | Good (default — `enter` without moving) |
| `4` | Easy                                    |

`↑/↓` move the cursor, `enter` commits the highlighted choice, `esc`
dismisses the modal (no rating recorded — the scheduler treats that as
"Good"). The chosen rating is written to LeetCode's submission record as an
`[anki:N]` tag in the submission note (`updateSubmissionNote`), so your
grading history travels with your account, not this binary. Wipe the local
SR cache and Review Mode rebuilds the timeline from those notes.

## Architecture

### Project layout

```
cmd/leetcode-anki/main.go        single binary entry; wires auth → client → cache → reviews → TUI
cmd/leetcode-test-login/main.go  populates test-account creds for the live contract suite
internal/auth/                   chromedp browser login + creds cache (incl. test-creds path)
internal/leetcode/               GraphQL + REST client (questions, lists, run, submit, submissions, notes)
internal/leetcode/leetcodefake/  method-level fake of *Client; shared by unit + contract tests
internal/leetcode/contracttest/  shape-invariant contract suite + LoadTestCreds for live runs
internal/render/                 HTML → markdown for problem descriptions
internal/editor/                 solution scaffolding + tea.ExecProcess editor invocation
internal/sr/                     spaced-repetition scheduler (SM-2) + on-disk cache
internal/tui/                    Bubble Tea root model + four screens
```

### Key decisions

**LeetCode is the source of truth for SR state.**
The user's submission history (with optional `[anki:N]` notes) is the
authoritative timeline; this binary stores only a memoized cache of
per-problem submission lists at `$UserConfigDir/leetcode-anki/sr.json`. Wipe
the cache and the next Review Mode entry rebuilds it from LeetCode.

**Interfaces at every external boundary.**
The TUI depends on `LeetcodeClient`, `SolutionCache`, `Editor`, and
`sr.Reviews` interfaces — never the concrete types. The LeetCode client
itself depends on a small `httpDoer { Do(*http.Request) (*http.Response,
error) }` rather than `*http.Client`. Production wiring satisfies these with
real implementations; tests inject fakes. See `internal/tui/client.go` and
`internal/leetcode/client.go`.

**All auth headers centralised in `client.setHeaders`.**
LeetCode requires `Cookie`, `x-csrftoken`, and (for run/submit) `Referer`
on every request. Centralising header attachment means individual API
methods cannot forget one and silently 401. Don't bypass it.

**SM-2 scheduler hidden behind a `Reviews` facade.**
`internal/sr/scheduler.go` defines a small `scheduler` interface taking a
`[]review` and returning `time.Time`. Today's implementation is SM-2
(`sm2.go`); swapping it (FSRS, Anki-modern, etc.) does not touch the TUI or
the cache layer. The conversion from `leetcode.Submission` (Accepted +
optional `[anki:N]` tag) into scheduler-quality 0–5 lives in `reviews.go`,
not the scheduler itself.

**Solutions are scaffolded but never overwritten.**
`Cache.Scaffold` creates a solution file from the language's starter snippet
on first edit. If a file already exists at the path, scaffolding is a no-op.
This makes resuming work after `q` safe by default.

**Bubble Tea + `tea.ExecProcess`: set state in `Update` synchronously.**
Wrapping an exec command in `tea.Sequence` to set state first does not
reliably deliver the state-setting message before exec takes the terminal —
set state in `Update`, then return only the exec `tea.Cmd`.

**Fakes-and-contracts for the LeetCode client.**
A single contract suite (`internal/leetcode/contracttest`) of shape-invariant
assertions runs against both a method-level fake
(`internal/leetcode/leetcodefake`) on every `go test ./...` and the real
LeetCode API behind `-tags integration`. When LeetCode's GraphQL schema
drifts, the live side fails and the fake stays passing as a regression
marker — the early-warning system the working notes below have always
needed.

### LeetCode API gotchas

- The GraphQL schema churns. `MyFavoriteLists` merges
  `myCreatedFavoriteList` and `favoritesLists.allFavorites` because either
  may be the source of a given list. When fields disappear (e.g.
  `nameTranslated` was removed from `TopicTagNode`), update both queries.
- For run/submit, `question_id` is the hidden numeric ID
  (`ProblemDetail.QuestionID`), **not** `questionFrontendId`.
- POSTs to `interpret_solution` and `submit` require
  `Referer: https://leetcode.com/problems/{slug}/`.
- Run/submit poll `/submissions/detail/{id}/check/` until
  `state == "SUCCESS"`.

## Known limitations

### Credentials at rest

The `LEETCODE_SESSION` and `csrftoken` cookies are persisted as a 0600
JSON file at `$UserConfigDir/leetcode-anki/creds.json`. `Save` writes
the file with mode 0600 (parent dir 0700) and `Load` refuses to read
the file if its mode is wider than 0600, so another local user can't
silently consume the session — but the cookies themselves are still
plaintext on disk.

A backlog item is to persist via the OS keyring
(macOS Keychain / Linux Secret Service / Windows Credential Manager)
with the 0600 file kept as a fallback when the keyring is unavailable.
The reason it isn't done yet is the macOS UX: Keychain prompts the
user for permission on first read, and unsigned binaries get re-prompted,
which is a meaningful regression for a TUI workflow. If you want to
harden now, manually move the cookies into Keychain and unset the
file — the next auth flow will recreate the file unless the keyring
backend lands.

## Development

```sh
go test ./...                  # unit tests
go vet ./...
go build ./...
```

`LEETCODE_DEBUG=1` enables raw GraphQL response logging to
`$UserCacheDir/leetcode-anki/debug.log` — useful when LeetCode's schema
shifts and a query starts returning `null` for a previously-populated field.

### Live contract test against LeetCode

`internal/leetcode/contracttest/` runs the same shape-invariant assertions
against a fake (every fast `go test ./...`) and the real LeetCode API
(behind `-tags integration`). The live side needs a **dedicated test
account** so writes don't leak into your personal profile.

One-time setup:

1. Create a fresh leetcode.com account (don't reuse your personal one).
2. Add the "Two Sum" problem to its Favorite Questions list.
3. `go run ./cmd/leetcode-test-login` and complete the browser login as
   the test account. Cookies land in
   `<UserConfigDir>/leetcode-anki/test-creds.json` (mode 0600).

Run:

```sh
go test -tags integration ./internal/leetcode/...
```

The GitHub Actions workflow at `.github/workflows/live-contract.yml`
runs the live contract on PRs and pushes to `main` that touch
`internal/leetcode/**` or `internal/auth/**`, plus a daily 07:00 UTC
cron. CI auth comes from `LEETCODE_TEST_SESSION` / `LEETCODE_TEST_CSRF`
repo secrets — see [CLAUDE.md](CLAUDE.md) for the secret-refresh
process when LeetCode's session cookie expires.

### Conventions

- **TDD**: new features and fixes ship red → green → refactor, with tests in
  the same commit as the code.
- **Atomic conventional commits**: one logical change per commit; `go vet`,
  `go build`, and `go test ./...` must pass at every commit. Subject ≤ 72
  chars, imperative mood. See [CLAUDE.md](CLAUDE.md) for the full rules.
- **Comments explain WHY, not WHAT.** Exported symbols get a one-line godoc;
  comments belong on the interface side, not buried in implementations. No
  PR-flavored comments (`// added for X`, `// fix for #123`) — those go in
  the commit message.
