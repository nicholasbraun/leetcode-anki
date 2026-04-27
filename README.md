# leetcode-anki

A terminal UI (Bubble Tea) for working through LeetCode problems and retaining
what you've solved through spaced-repetition reviews. Walk a problem list, read
the prompt, write a solution in your `$EDITOR`, run it against the examples,
submit for a verdict. Once a problem is accepted it joins the SR rotation and
shows up in **Review Mode** when due.

The canonical domain vocabulary lives in [CONTEXT.md](CONTEXT.md) — read that
first if you intend to contribute.

## Setup

### Prerequisites

- Go 1.26.1+ (see [`go.mod`](go.mod))
- Google Chrome / Chromium installed locally — used by `chromedp` for the
  browser-based login flow
- A LeetCode account
- `$VISUAL` or `$EDITOR` set (falls back to `vi`)

### Build and run

```sh
git clone <repo> leetcode-anki && cd leetcode-anki
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

| Path | Contents |
| --- | --- |
| `$UserConfigDir/leetcode-anki/creds.json` | session + CSRF cookies (mode 0600) |
| `$UserConfigDir/leetcode-anki/sr.json` | spaced-repetition cache (per-slug submission timeline) |
| `$UserCacheDir/leetcode-anki/<slug>/solution.<ext>` | scaffolded solution files; never overwritten on re-scaffold so resumed work is preserved |
| `$UserCacheDir/leetcode-anki/debug.log` | raw GraphQL responses, only when `LEETCODE_DEBUG=1` |

## Usage

The TUI walks four screens: `lists → problems → problem → result`.

### Keys

| Key | Action |
| --- | --- |
| `↑`/`k`, `↓`/`j` | move |
| `enter` | select |
| `esc` / `backspace` | back (also cancels an in-flight run/submit) |
| `q` / `ctrl+c` | quit |
| `e` | edit current problem's solution in `$EDITOR` |
| `r` | run against example testcases (`interpret_solution`) |
| `s` | submit (`/submit/`) |
| `l` | cycle language for the current problem |
| `n` / `p` | next / previous problem in the current list |
| `pgup`/`ctrl+u`, `pgdn`/`ctrl+d` | scroll the problem-list preview pane |
| `v` | enter Review Mode from the lists screen |
| `?` | help |

### Explore Mode vs Review Mode

- **Explore Mode** (default on lists screen): the full problem list.
- **Review Mode** (`v` from lists): only problems currently due for review. The
  candidate set comes from your global LeetCode submission progress; due-date
  is computed locally by the SR scheduler.

A problem enters the SR rotation on its **first accepted submit** — runs do
not count. Subsequent attempts in any language fold into the same rotation
(the problem is the SR unit, not the `(problem, language)` pair).

### Optional grading tags

Submission notes can carry an explicit Anki-style grade so the scheduler does
not have to assume "Accepted == Good":

```
[anki:1]   Again
[anki:2]   Hard
[anki:3]   Good (default; same as no tag)
[anki:4]   Easy
```

Notes are stored on LeetCode's submission record (`updateSubmissionNote`), so
your grading history travels with your account, not this binary.

## Architecture

### Project layout

```
cmd/leetcode-anki/main.go      single binary entry; wires auth → client → cache → reviews → TUI
internal/auth/                 chromedp browser login + creds cache
internal/leetcode/             GraphQL + REST client (questions, lists, run, submit, submissions, notes)
internal/render/               HTML → markdown for problem descriptions
internal/editor/               solution scaffolding + tea.ExecProcess editor invocation
internal/sr/                   spaced-repetition scheduler (SM-2) + on-disk cache
internal/tui/                  Bubble Tea root model + four screens
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

## Development

```sh
go test ./...                  # unit tests
go vet ./...
go build ./...
```

No integration tests exist yet. The `integration` build tag is reserved for
tests that hit real LeetCode — when the first one lands it goes in a
`//go:build integration` file and runs via `go test -tags=integration ./...`,
off by default.

`LEETCODE_DEBUG=1` enables raw GraphQL response logging to
`$UserCacheDir/leetcode-anki/debug.log` — useful when LeetCode's schema
shifts and a query starts returning `null` for a previously-populated field.

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
