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

**Example Test Case**:
A canonical test input LeetCode supplies on a Problem (sourced from `ProblemDetail.ExampleTestcases`). Read-only — the user cannot edit or remove them.

**Custom Test Case**:
A test input the user adds to a Problem, typically by promoting the failing input returned with a Wrong-Answer Submit Verdict. Mutable user state: can be added, removed, and persisted across TUI runs. A Run feeds both Example and Custom Test Cases to `interpret_solution`.
_Avoid_: Extra Test Case, Pinned Test Case, Added Test Case (less aligned with LeetCode's own web vocabulary, which calls these "custom testcases" in the Testcase pane).

**Run** (verb):
Execute the user's Solution against the Problem's Example and Custom Test Cases (LeetCode's `interpret_solution` endpoint). Trial-only — Run Verdicts do not affect SR rotation.

**Submit** (verb):
Execute the user's Solution against LeetCode's full grader (LeetCode's `/submit/` endpoint). The first Accepted Submit puts a Problem into SR rotation.

**Verdict**:
The outcome of a Run or a Submit (Accepted, Wrong Answer, Compile Error, Runtime Error, Time Limit Exceeded, etc.).
_Avoid_: Result (overloaded with "the result of any function call"; less domain-specific).

### Spaced repetition

**Review** (noun + verb):
A single instance of revisiting a previously-solved Problem on its due date — the user re-attempts and Submits. The first Accepted Submit on a Problem also counts as that Problem's first Review (the scheduler's baseline).

**Review Mode**:
The TUI mode where the Problem List is filtered to Problems currently due for Review and within today's Daily Quota for that list. Once the quota is exhausted for a list, Review Mode keeps that list's queue empty until local midnight (Quota Exhausted state).
_Avoid_: Anki Mode (the repo name is historical/origin-flavor; the runtime mode names should not couple our domain to a specific third-party tool).

**Explore Mode**:
The TUI mode that shows the full Problem List regardless of due-state. Today's default behavior.

**Daily Quota**:
The maximum amount of SR work the user takes on per Problem List per day. Composed of two independent buckets — the **Review Quota** and the **New Quota**. The quota window runs local midnight to local midnight. Each bucket's remaining count is `bucket size - bucket consumed today`, clamped to zero. When a bucket is zero, Review Mode stops surfacing items of that kind for that list, even if the list still has due or new candidates left.
_Avoid_: Daily Cap, MaxDue cap (the per-Session caps that preceded this feature were also called "caps"; using "cap" risks conflating per-day with per-session semantics). The implementation-level fields `SessionConfig.MaxDue` / `MaxNew` keep their names but are now the bucket sizes for the day, not the size of one Session.

**Review Quota**:
The Daily Quota bucket for previously-Accepted Problems. One slot is consumed by each Accepted Submit today on a slug that was Tracked-and-Due at the moment of AC. Voluntary re-ACs of Problems that were not yet due do not consume it.

**New Quota**:
The Daily Quota bucket for never-Accepted Problems. One slot is consumed by today's first-ever Accepted Submit on a slug — the same event that transitions the Problem to Tracked.

**Quota Exhausted**:
A Problem List's Review Mode state when both quotas for the day are zero and at least one Problem in the list is still due or still new. Distinct from a list with genuinely no work to surface. The TUI distinguishes the two so the user can tell "you're done for the day" from "this list is empty."

## Relationships

- A **Problem List** contains many **Problems**.
- A **Problem** has zero or many **Solutions** — one per language attempted.
- A **Solution** belongs to exactly one **Problem** and one language.
- A **Run** and a **Submit** each produce exactly one **Verdict**.
- A **Problem** accumulates **Reviews**; the SR scheduler derives the next due-date from that history.
- A Problem's first Accepted **Submit Verdict** is also that Problem's first **Review**.
- A **Problem** carries zero or many **Custom Test Cases**. They feed the next **Run** alongside the Problem's **Example Test Cases**.
- A **Problem List** has one **Daily Quota** per day, composed of one **Review Quota** and one **New Quota**; both reset at local midnight.
- A **Review** consumes one slot of the Problem List's **Review Quota** for the day on which it is Accepted.
- A first Accepted **Submit** on a Problem consumes one slot of the **New Quota** for the list it was reached from, and adds the Problem to SR rotation.

## Example dialogue

> **You:** "I just finished two-sum in Go — when does it show up in Review Mode?"
> **Future-you:** "Once you Submit and get an Accepted Verdict, the Problem joins the SR rotation. The first Accepted Submit is the first Review, so the scheduler picks the first due-date from there. Run Verdicts don't count — only Submit."
>
> **You:** "What if I open two-sum again later in Python?"
> **Future-you:** "You'll get a second Solution under the same Problem. The Problem is the SR unit, not the (Problem, language) pair, so a Python Accepted Submit would be a Review on the same Problem — same rotation, same scheduler."
>
> **You:** "I cleared today's reviews on my main list. Why does Review Mode show nothing on it now, even though I know I have more due?"
> **Future-you:** "You hit the list's Daily Quota — Review Quota and New Quota both reset at local midnight. The remaining due Problems aren't going anywhere; they'll surface tomorrow. The TUI labels this state Quota Exhausted to distinguish it from a list with no due work at all."

## Flagged ambiguities

- "Question" (LeetCode schema) and **Problem** (TUI / domain) named the same thing — resolved: **Problem** is canonical; `Question` survives only as a name for the wire-format type at the client boundary.
- "Draft" / "hasDraft" / "hasLocalDraft" / `scaffoldPath` named the same thing as **Solution** — resolved: **Solution** is canonical. "Scaffold" survives only as a verb. The TUI's "✎ In progress" badge is UX text for "Problem has a Solution," not a separate domain stage.
- "Favorite List" (LeetCode schema) and "List" (TUI shorthand) named the same thing — resolved: **Problem List** is canonical; "Favorite List" is schema-only; "List" survives as TUI-side shorthand when context is clear.
- "Result" (struct names like `RunResult` / `SubmitResult`) and **Verdict** named the same thing — resolved: **Verdict** is canonical at the domain level; "Result" remains acceptable as a Go type-name suffix.
- "Daily cap" / "MaxDue cap" / per-Session cap and **Daily Quota** named the same thing — resolved: **Daily Quota** is canonical for the per-day budget. `SessionConfig.MaxDue` and `MaxNew` keep their Go field names but their meaning shifts from per-load caps to per-day quota sizes; callers must read them through the quota lens.
