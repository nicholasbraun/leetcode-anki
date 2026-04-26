# leetcode-anki

A TUI for working through LeetCode problems and retaining what you've solved through spaced-repetition Reviews. Walk a Problem List, read the prompt, write a Solution, Run it against the examples, Submit for a Verdict. Once a Problem is Accepted, it joins the SR rotation and shows up in Review Mode when due.

## Language

### Problems and lists

**Problem**:
A LeetCode coding exercise the user is working on, identified by its title slug (e.g. `two-sum`).
_Avoid_: Question (the LeetCode GraphQL schema's name — keep it confined to the LeetCode-client boundary; never use it in the TUI or in user-facing copy).

**Problem List**:
A curated set of Problems the user navigates. Includes both lists the user authored and lists saved/starred from elsewhere; the distinction is invisible at this level.
_Avoid_: Favorite List (LeetCode schema name; schema-only, like Question), List (acceptable as TUI shorthand only when context is unambiguous).

### Solutions

**Solution**:
The user's local file for a Problem in a chosen language. Every Solution lives at exactly one path and starts life as the language's starter snippet, scaffolded on first edit. Whether it's been edited or still matches the snippet is not a domain stage — the code does not distinguish.
_Avoid_: Draft, scaffold (as a noun).

**Scaffold** (verb):
Create a Solution from the Problem's language snippet if one does not already exist. Idempotent — scaffolding an existing Solution is a no-op so resumed work is never overwritten.

### Running and submitting

**Run** (verb):
Execute the user's Solution against the Problem's example test cases (LeetCode's `interpret_solution` endpoint). Trial-only — Run Verdicts do not affect SR rotation.

**Submit** (verb):
Execute the user's Solution against LeetCode's full grader (LeetCode's `/submit/` endpoint). The first Accepted Submit puts a Problem into SR rotation.

**Verdict**:
The outcome of a Run or a Submit (Accepted, Wrong Answer, Compile Error, Runtime Error, Time Limit Exceeded, etc.).
_Avoid_: Result (overloaded with "the result of any function call"; less domain-specific).

### Spaced repetition

**Review** (noun + verb):
A single instance of revisiting a previously-solved Problem on its due date — the user re-attempts and Submits. The first Accepted Submit on a Problem also counts as that Problem's first Review (the scheduler's baseline).

**Review Mode**:
The TUI mode where the Problem List is filtered to Problems currently due for Review.
_Avoid_: Anki Mode (the repo name is historical/origin-flavor; the runtime mode names should not couple our domain to a specific third-party tool).

**Explore Mode**:
The TUI mode that shows the full Problem List regardless of due-state. Today's default behavior.

## Relationships

- A **Problem List** contains many **Problems**.
- A **Problem** has zero or many **Solutions** — one per language attempted.
- A **Solution** belongs to exactly one **Problem** and one language.
- A **Run** and a **Submit** each produce exactly one **Verdict**.
- A **Problem** accumulates **Reviews**; the SR scheduler derives the next due-date from that history.
- A Problem's first Accepted **Submit Verdict** is also that Problem's first **Review**.

## Example dialogue

> **You:** "I just finished two-sum in Go — when does it show up in Review Mode?"
> **Future-you:** "Once you Submit and get an Accepted Verdict, the Problem joins the SR rotation. The first Accepted Submit is the first Review, so the scheduler picks the first due-date from there. Run Verdicts don't count — only Submit."
>
> **You:** "What if I open two-sum again later in Python?"
> **Future-you:** "You'll get a second Solution under the same Problem. The Problem is the SR unit, not the (Problem, language) pair, so a Python Accepted Submit would be a Review on the same Problem — same rotation, same scheduler."

## Flagged ambiguities

- "Question" (LeetCode schema) and **Problem** (TUI / domain) named the same thing — resolved: **Problem** is canonical; `Question` survives only as a name for the wire-format type at the client boundary.
- "Draft" / "hasDraft" / "hasLocalDraft" / `scaffoldPath` named the same thing as **Solution** — resolved: **Solution** is canonical. "Scaffold" survives only as a verb. The TUI's "✎ In progress" badge is UX text for "Problem has a Solution," not a separate domain stage.
- "Favorite List" (LeetCode schema) and "List" (TUI shorthand) named the same thing — resolved: **Problem List** is canonical; "Favorite List" is schema-only; "List" survives as TUI-side shorthand when context is clear.
- "Result" (struct names like `RunResult` / `SubmitResult`) and **Verdict** named the same thing — resolved: **Verdict** is canonical at the domain level; "Result" remains acceptable as a Go type-name suffix.
