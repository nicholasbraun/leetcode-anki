# Code Review — `leetcode-anki`

## BLOCKER

### 1. Path traversal via `titleSlug` — `internal/editor/editor.go:51-57, 62-79`
`SolutionPath` does `filepath.Join(dir, "leetcode-anki", titleSlug, "solution."+Ext(langSlug))`, and `titleSlug` is populated straight from a GraphQL response. If LeetCode (or anything intercepting the response) returns a `titleSlug` like `../../../../etc/leetcode-anki` or contains a NUL byte, `ScaffoldFile` will write outside the cache dir. The risk surface is small in practice — LeetCode is trusted, slugs are constrained — but this is the one case where untrusted input touches `os.MkdirAll` + `os.WriteFile` and there's no validation gate.
**Fix:** in `SolutionPath`, validate that `titleSlug` matches `^[a-z0-9][a-z0-9-]*$` (LeetCode's actual slug format); reject anything with `..`, `/`, `\`, or control chars.

---

## MAJOR

### 2. `pollCheck` has no terminal-state escape — `internal/leetcode/run.go:61-88`
The loop only exits on `state == "SUCCESS"` or context cancel. If LeetCode ever returns a different terminal value (`FAILED`, `TIMEOUT`, `PENDING_REJUDGE`, etc.), the loop polls until the 120 s outer timeout. Worse, if the GET call returns an empty `{}` body (auth lapse, weird redirect), `state` is the empty string forever.
**Fix:** track a max number of polls (e.g. 200) or treat any non-empty `state` other than `PENDING`/`STARTED` as terminal and return it; let the caller decide if the unknown state is an error.

### 3. Credentials/error bodies leaked into TUI banner — `internal/leetcode/client.go:72, 112`
On non-2xx status, the full response body is interpolated into the error: `fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))`. That body can contain CSRF echoes, server-side debug, sometimes a `Set-Cookie` shadow in HTML error pages. The error then bubbles to `m.err` and is rendered by `truncateErr` in the TUI (and in stderr on auth failure). Session cookies aren't typically echoed back in response bodies, so this isn't credentials-leak in the strict sense — but it's the single biggest source of "spew weird stuff to the user". Combined with no log redaction, this is the highest-leverage hardening fix.
**Fix:** log only `status %d` to the user; keep the raw body behind a `--debug` flag or write it to a per-session file under `os.UserCacheDir()/leetcode-anki/log/`.

### 4. Parent context is stored but never used — `internal/tui/app.go:28, 60-65, 290-355`
`Model.ctx` is captured in `NewModel` and then ignored. Every `loadListsCmd`/`loadProblemCmd`/`runCodeCmd`/`submitCodeCmd` builds its own `context.Background()` with a fresh timeout. Two consequences: (a) a SIGINT to the parent can't cancel an in-flight submit; (b) pressing `esc` mid-run *does not* cancel the run/submit goroutine — it just changes the screen, and the goroutine keeps running and eventually overwrites state.
**Fix:** derive each cmd's context from `m.ctx`, and store a `cancelInflight context.CancelFunc` on the model so `esc` during run/submit can call it.

### 5. Zero tests — *whole repo*
No `_test.go` files. Highest-leverage units to cover, in priority order:
1. `pollCheck` — feed a fake `doREST` that returns scripted state transitions; cover SUCCESS, ctx-cancel, and the unknown-state case (#2).
2. `MyFavoriteLists` merge/dedup — feed canned GraphQL bodies for both schema variants and assert the union/dedup behavior in `internal/leetcode/lists.go:54-100`.
3. `ScaffoldFile` no-overwrite contract + path validation (#1) — `internal/editor/editor.go:62`.
4. `existingSolutionPath` + `Ext` lookup — `internal/tui/problem_view.go:71-83`, `internal/editor/editor.go:42`.
5. `migrateLegacy` — `internal/auth/store.go:50`.

The HTTP and TUI layers can stay untested for v1; the four units above carry most of the regression risk.

### 6. `Save` warnings invisible in TUI — `internal/auth/browser.go:94-96, 108-110`
`fmt.Fprintf(os.Stderr, "warning: ...")` happens *after* `tea.WithAltScreen()` swaps the buffer. The user never sees it; on exit the alt-screen is torn down and the message is gone with the scrollback. If `Save` fails, the user thinks they're cached and gets a chromedp re-login on every launch.
**Fix:** propagate the error up to the caller; surface it in the TUI banner on next launch, or fail loud with `log.Fatalf` in `cmd/leetcode-anki/main.go` since it happens *before* the TUI starts.

---

## MINOR

### 7. `MyFavoriteLists` swallows JSON decode errors silently — `internal/leetcode/lists.go:53-67`
The `jerr` from `json.Unmarshal` of the first query result is dropped. If LeetCode quietly changes the field shape, you get an empty merge instead of a useful error.
**Fix:** at least debug-log the swallowed `jerr`; or accumulate both errors and return them if the merge ends up empty.

### 8. Glamour renderer rebuilt every problem load — `internal/tui/problem_view.go:36-43`
`glamour.NewTermRenderer(...)` parses styles every call. Cheap but not free; on a slow machine you'll feel it during navigation. **Fix:** cache the renderer on `problemView` keyed by width, rebuild only on `WindowSizeMsg` width change.

### 9. Difficulty matching is half-normalized — `internal/tui/styles.go:38-48`
Hardcoded both `"EASY"` and `"Easy"` cases; a future `"easy"` falls through to dim. **Fix:** `switch strings.ToUpper(d)` once, then bare uppercase cases.

### 10. Hard-coded language preference — `internal/tui/problem_view.go:67-83`
`pickDefaultLang` always prefers golang → python3. No way to set a default. **Fix:** read `LEETCODE_ANKI_LANG` env or `--lang` flag in `cmd/leetcode-anki/main.go:14` and pass through; remember the last-picked language across screens (already works per-problem since the file persists).

### 11. `m.problem.setProblem` errors swallowed during resize — `internal/tui/app.go:88`
Renderer error in the WindowSizeMsg path is `_ = ...`'d. If glamour hits an issue mid-resize the description silently goes blank.
**Fix:** at least set `m.err` so the user sees something.

### 12. `tea.WithMouseCellMotion()` breaks terminal text selection — `internal/tui/app.go:259`
Mouse capture is on. Users on macOS/iTerm can't select text in the description view to copy it.
**Fix:** drop it, or scope it (no clear reason it's needed for this app).

### 13. Help line is not context-aware — `internal/tui/problem_view.go:194`
`e edit · l language · r run · s submit · n next · p prev · esc back · q quit` is shown unconditionally; `r`/`s` aren't valid until a solution exists.
**Fix:** dim or hide them when `pv.scaffoldPath == ""`.

### 14. `loadProblemsCmd` hardcodes `limit=500` — `internal/tui/app.go:306`
Lists with more than 500 problems will silently truncate.
**Fix:** loop with `hasMore`/`skip` until exhausted, or set a sane cap and surface "showing first N of M".

---

## NIT

15. **Dead code:** `auth.Path()` (`store.go:22-24`) is unreferenced. Drop it.
16. **Empty Go fields populated but never displayed:** `ProblemDetail.TopicTags`, `ProblemDetail.Hints` (`types.go:55-56`). Either render them in `problem_view.go` or stop fetching them.
17. **Banner truncation strips ANSI mid-sequence:** `truncateErr` (`app.go:244-249`) does a byte-slice on a string that contains escape codes from upstream errors. Could leave a dangling ESC. Use `lipgloss.Width`-aware truncation or strip codes first.
18. **Inconsistent error-message style:** `cmd/leetcode-anki/main.go:30-37` uses `"auth: %v"` / `"tui: %v"` while internal layers use `"interpret_solution: %w"`. Standardize on lowercase verb-phrase prefixes everywhere.
19. **Missing godoc on exported symbols:** `Client`, `NewClient`, `BaseURL`, `GraphQLURL`, `UserAgent`, `Run`, `NewModel`, `Path` (in auth), `Delete`. Add one-liners.
20. **`UserAgent = "Mozilla/5.0"` is a smell** (`client.go:18`). Fine for now, but if LeetCode tightens UA filtering you'll want a real string.
21. **`Save`'s parent dir is `0o700` but the cache dir is `0o755`** (`editor.go:72`). Solution files contain solved code, not secrets, so this is fine — just note the inconsistency if you ever scaffold creds in there.

---

## Top 5 to fix first

1. **#1 path traversal** — single-line slug validator; cheapest big win.
2. **#3 raw error bodies** — replace with `status %d`; keep verbose body behind a debug flag.
3. **#4 wire `m.ctx` through cmds** — fixes "esc doesn't cancel run/submit" and SIGINT.
4. **#2 pollCheck terminal-state escape** — guards against an indefinite hang.
5. **#5 add the four high-leverage tests** — `pollCheck`, `MyFavoriteLists`, `ScaffoldFile`, `existingSolutionPath`.

## What's good

- Package layout (`internal/{auth,leetcode,render,editor,tui}` + `cmd/leetcode-anki`) is clean and `tui` only depends on `leetcode`/`editor` types it actually needs — no leakage in the wrong direction.
- `Client.setHeaders` is the single source of truth for cookies/CSRF/Referer; centralizing this was the right call and would have prevented several classes of bugs you'd otherwise hit.
- The `MyFavoriteLists` merge-with-fallback shape is *the* correct response to LeetCode's schema churn.
- Edit flow scaffolds a file, opens `$EDITOR`, and reads it back without ceremony. The decision to set `scaffoldPath` synchronously (instead of via `tea.Sequence` + a message) avoids a real Bubble Tea footgun.
- Errors at API boundaries are wrapped with `%w` consistently; `%v` only appears at the very edge in `cmd/`, which is the right convention.
