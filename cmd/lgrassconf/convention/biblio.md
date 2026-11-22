## Bookkeeping is the model's own job

Writing under `biblio/` never waits for the user. It is the model's own responsibility, the same category as running tests after a code change: scratchpad task directories and every file in them (marking Plans items done, recording decisions, the activity log), book chapters and their `toc.md` line, laws, handovers with their whiteboard rows and memory pointers, and archiving a finished task. Deciding whether finished work warrants a chapter is on the model too.

What needs the user: scaffolding `biblio/` for the first time, `.gitignore`, adding the autoload trigger, and any change to the core idea of a PRD, meaning its Problem, Expectation or Objective, a settled design decision, or dropping a Plans item. Reading and reporting is always fine. A user's instruction to hold off overrides this.

## Why two tiers

The agent's startup pointer loads every session: Claude Code uses project memory; Codex uses a project-conditioned entry in the user's global instructions. Everything in `biblio/` loads only when something — the model or the user — opens that file. Detail belongs in the second kind of place, because a project's knowledge grows over time and an always-loaded tier can't afford to grow with it.

That gives a hard law: **memory never stores knowledge, only pointers to it, no exceptions.** A memory file that contains the actual fact, decision, or design writeup instead of a one-or-two-sentence pointer is a bug in how the project is being run, not a style choice.

## Tier 1 — an auto-loading pointer, native to whichever agent this is

Every agent using this convention needs exactly one thing here: a pointer in its own automatically loaded, user-local instructions, scoped to this project. Do not put the pointer in a committed repository onboarding file (`CLAUDE.md`, `AGENTS.md`, or equivalent). If another agent genuinely has no native auto-load mechanism, tell the user rather than improvising one.

- **Claude Code** uses project memory (`~/.claude/projects/<project-slug>/memory/`). Mechanism, format, and bootstrap check: `references/claude-code-setup.md`.
- **Codex** uses a path-conditioned entry in the user's global `~/.codex/AGENTS.md`. Mechanism, format, and bootstrap check: `references/codex-setup.md`.
- **Cursor** uses a user-level `sessionStart` hook (`~/.cursor/hooks.json` + `bibliothek-session-start.sh`) with a path registry. Mechanism, format, and bootstrap check: `references/cursor-setup.md`.

### A different agent

Claude Code's `type` split, `[[name]]` links, and `MEMORY.md` format and Codex's global `AGENTS.md` location are agent-specific. For another agent, find its own user-local mechanism that loads at session start, scope the pointer to this project, and use the autoload-trigger wording in Bootstrapping. Never substitute a committed onboarding file; if no such mechanism exists, tell the user.

## Tier 2 — `biblio/` (in the project root)

Four subfolders, each with a distinct job. Default to gitignoring the whole `biblio/` directory — it's the AI-collaboration workspace, not the shipped product — unless the user says otherwise.

A gitignored directory is invisible to most search tools by default (`rg --files`, `git ls-files`, agents' own file search), so an empty search is not evidence `biblio/` is missing. List the path directly (`ls biblio/`, or `rg --no-ignore` / `fd -I`) before concluding that.

### `biblio/scratchpad/`
Working space, editable by both the user and the model. One subdirectory per task (`biblio/scratchpad/<task-slug>/...`), never a bare file dropped at the root, even for a single file. It holds planning, debugging, shaping an undecided idea, a decided task's build record. A handover's Status never carries a plan; the plan lives in the task's `prd.md`.

A task directory is that task's folder. The names below are reserved for their purpose, with `activities.md` on every task. Any other important file the work needs can be added beside them.
- `prd.md` — work that will be built, edited in place as decisions firm up. Four parts: **Problem** (what is wrong or missing), **Expectation** (what should be true or possible once it's solved), **Objective** (what the work sets out to achieve, and what is out of scope), **Plans** (implementation details: design decisions, then the phases in build order, each a `- [ ]` item flipped to `- [x]` the moment it is done and verified). The marks are the progress record: anyone resuming reads them to know what is done.
- `debug.md` — chasing a fault: symptom, what's been ruled out, current best theory, rewritten in place, plus the root cause once found.
- `recon.md` — an investigation with no build: synthesized findings, rewritten in place as understanding firms up, not a log. Graduates into a book chapter once settled, or the task archives with no chapter if the answer was "not worth pursuing."
- `notes.md` — an idea not yet confirmed or decided; loose is fine. Once it's decided, the task adds a `prd.md` beside it.
- `activities.md` — a dated, append-only log of what actually happened: a decision reached and how, code shipped, a correction made. Not a transcript of every tool call. The one place chronological narrative lives, scoped to this task.

**Pointed at a scratchpad**, read it, say back what you understood rather than treating it as instructions, and continue from its marks. A PRD with no Plans gets them drafted. When a PRD is written or updated, say where it is and give a view, such as whether to split it. If the user asks to just show a plan, that's chat only, no file.

**Offer to split.** When a PRD's Plans hold a sub-thread big enough to be built and tracked on its own, offer the user to split it into its own scratchpad task with its own `prd.md`, whose Problem or Plans point at the parent's section (`biblio/scratchpad/<parent-slug>/prd.md`, section name) instead of restating it.

Not indexed by `toc.md` and not read automatically, but not disposable: once a task closes, its directory, `activities.md` included, moves into `biblio/scratchpad/archive/` as the permanent record, alongside its handover's archived copy (see `biblio/handover/` below).

### `biblio/laws/`
One file per standing behavioral rule, written once, directly here — no matching memory file, nothing to keep in sync. A law only actually governs every agent touching this project if it lives somewhere all of them can read — a plain file in the repo, not any one agent's own memory feature. Plain markdown, no YAML needed since nothing but a human or an agent's own eyes ever parses it:

```markdown
# <Law title>

<the rule, stated as an instruction>

**Why:** <rationale>
**How to apply:** <concrete guidance>
```

A law's content must be as terse and timeless as the rule itself: no dates, no "recurred on X,"
no session narrative — just the distilled rule, a general Why, and a concrete How-to-apply, a
handful of lines total. Distill any recurrence history behind the correction into that shape on the
way in; don't transplant a narrative.

Unlike a book, a law isn't read on demand — it's small and binding for the whole project. `biblio/laws/summary.md` is the read target: one line per law (the rule itself, pulled from each file's own opening line) plus a pointer to the full file, kept current the same way `biblio/books/toc.md` is. Every agent reads it in full before doing anything, unless it already arrived this session -- opening an individual law's file only when its one-liner isn't enough (its Why or How-to-apply).

A complete `# Laws summary` block in session context before the first response proves delivery: it is already read, so do not reopen the file. A configured hook or pointer is not proof. If the block is absent, incomplete, or its source is uncertain, open the file in full.

This is also why `type: feedback` memory entries are pointer-only from the start (see Tier 1): Claude Code's own memory is a fixed, mixed-purpose surface the harness can truncate once full, so it can't be the durable home for a growing, open-ended list of standing rules -- only `biblio/laws/` can be. Once a project's `SessionStart` hook is *confirmed* actually delivering `summary.md` (verified firing, not just installed), `biblio/laws/` is the load-bearing copy for every standing rule it holds. A `feedback`-type memory file that survives from before that confirmation -- migrated into `biblio/laws/<slug>.md` but never removed from memory -- is drift, not a second, more-detailed copy: delete the memory file and its `MEMORY.md` line once its content exists in `biblio/laws/`.

### `biblio/handover/`
One file per paused or delegated task, meant to be picked up cold by another session. A small task finished in one session needs none. **A handover is never written for work that's already finished with nothing left to pick up — that is a book chapter, full stop, no exceptions.**

**Scope is one task, not a program.** A handover names a single, narrow thread of work — never an umbrella ("X rework", "X initiative") meant to absorb everything touching some broad area. The instant a distinct sub-thread inside a handover grows its own book chapter, its own build state, or work that would still matter after the original task closes, it splits into its own handover file — immediately, not once the original has become unscannable. A handover whose Status section is listing several unrelated decisions, or whose Log is narrating more than one thread, has already missed the split point. Fixed shape, always:

```markdown
# <Task> — handover

<one-line summary, plus a pointer to the task's prd.md and to the matching biblio/books/ chapter if one exists>

## Status — this section is authoritative, the Log below is history only

<current state only, rewritten in place as things change — never append a new status
on top of an old one>

## Log — history only, not authoritative

See `biblio/scratchpad/<task-slug>/activities.md`.
```

A handover points at a `prd.md` only, never at another scratchpad file, and never restates what the PRD holds: Status says where to resume, what is next and what is blocked; what is done lives in the PRD's marks. One source of truth per fact, so nothing goes stale. A task of another type that needs picking up cold gets a `prd.md` first. A handover never carries dated narrative itself — that lives in its task's `activities.md` instead. Read Status to know where things stand; open the Log's `activities.md` only to understand how it got there. Never track live/transient state (branch name, deploy status) here either — same rule as memory.

Once a handover's work is actually implemented, it stops being active — but it is never deleted, only soft-deleted: move the file into `biblio/handover/archive/`, Status section left as the final state ("Done, implemented <date>"), and move that task's `biblio/scratchpad/<task-slug>/` directory into `biblio/scratchpad/archive/` in the same action. `biblio/handover/` and `biblio/scratchpad/` (excluding their own `archive/` subfolders) hold only work that's still in flight, nothing else; that's what makes them useful to scan cold. Both `archive/` folders are the record-keeping copy, kept indefinitely.

**`biblio/handover/whiteboard.md`**, required: a numbered priority index over every active handover, for a model acting as PM across several others without opening each file cold. One row per handover, in priority order:

```
1. task-slug [tag,tag] -> handover-file-name.md
```

Tags follow the same rule as book tags; `blocked` is a reserved tag for a handover that's mostly done but stalled on a decision or pushback, surfaced here without restating why — that's one hop away, in the handover's own Status. No summary column; that's exactly the drift risk this file exists to avoid. Add a row the moment a handover is created, renumber in place as priority shifts, and drop the row in the same action that archives the handover.

Retire that task's memory pointer in the same action, not after: delete the `type: project` memory file and its `MEMORY.md` line the moment its handover is archived. Memory is precious, always-loaded space for what's still active, not a historical log of finished work -- the archived handover and a book chapter (see below) are already the closure record, so a leftover pointer there is pure weight with no reader that needs it.

### `biblio/books/`
The durable, growing detail — design decisions, domain facts, lore, whatever this project needs remembered precisely and in full — organized as a small library. **Deciding whether a finished piece of work warrants a chapter, and writing it, is on the model, not the user** — a project that's adopted this convention doesn't need to be told to keep its books current any more than it needs to be told to run tests.

- A **book** is a folder: `biblio/books/title_snake_case/`. The title is the whole name, so a book's folder never needs renaming when its content or tags change.
- A **chapter** is one markdown file inside a book: `title_snake_case.md`, named for what it covers. Free-form prose inside (dense is fine — this tier is read on demand, not loaded automatically). A chapter is a frozen snapshot of settled facts and decisions, not a workspace: write it once a task is finished, and touch it again only for a deliberate future rework — never to park a mid-task finding or hypothesis. No deictic/narrative language, no changelog, no dated "X happened, then Y." What's still in progress belongs in the task's `activities.md` instead.
- **Tags** belong to a book and live only on its `toc.md` line, comma-separated. Each tag is one token, lowercase or camelCase, with no spaces, hyphens or underscores, so a compound idea is one camelCase tag (an "IPD approval" book is tagged `ipdApproval`, not `ipd-approval`). Use the terms this project already uses for the thing (a module name, a faction name, a short domain term) instead of a generic category word. The entire point of a tag is to cut the toc down fast, without opening a book; optimize for that, not for taxonomic completeness.
- "Last updated" is never encoded anywhere — the filesystem's own mtime already answers that for free, and baking a live-changing field into a name is what forces every edit to also be a rename.

`biblio/books/toc.md` lists every book, one line each: the book's folder name followed by its bracketed tags, and no description: `ls` of the folder shows its chapter names, which say what each covers. The one allowed addition is a single-sentence warning after the tags, for a book that is obsolete or misleading, naming what to read instead.

```
title_snake_case [tag,camelTag]
title_snake_case [tag,camelTag] warning: obsolete, read other_book instead.
```

Rewritten in place as a book's content changes, never appended to — no dated lines, no session history; that belongs in a task's `activities.md`. A book's tags are revised on its line when its content shifts.

`toc.md` is small enough that any session can read it regardless of the current task. Together with book/chapter naming and tags it is the entire navigation layer for `biblio/` — the fix for a navigation problem is better grouping or tags, never an extra index, relationship map, or log. Read it and open the books it points to before reading source or investigating from first principles; go to the code only for what no book covers.

## Bootstrapping a new or newly-migrated project onto this convention

**The laws summary.** Apply the delivery rule above. If `biblio/laws/summary.md` exists and has not already arrived, open it in full before other work.
