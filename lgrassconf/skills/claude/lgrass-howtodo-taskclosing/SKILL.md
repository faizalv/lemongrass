---
name: lgrass-howtodo-taskclosing
description: How to close a finished task under the bibliothek convention. Use when a task's work is done, or when the user says close, finish or archive a task.
---

Invoking this skill is the go-ahead for every step below except the ones marked ask. Run them in order. Git is read-only here: `status`, `diff`, `log` and `show` only, never `add`, `commit` or anything that changes the index or history.

Closing means every in-scope objective is finished with full confidence, and the closing then carries no hedges: no "not verified", "should work" or caveat section. Only what the PRD's Objective excludes, or what was clearly decided to leave for later, is out of scope and is not mentioned, except as step 4 directs. A passing idea or a drift is not out of scope: it is ignored, never raised.

If at any step the task is not closable, stop and say so plainly: "I can't close this because <the objective and the concrete evidence>". Refusing is always better than closing with doubt. After refusing, archive nothing and change no whiteboard row or memory pointer.

## 1. Definition of done

- Every item in the PRD's Objective is confirmed finished by evidence, one by one.
- Every item in the PRD's Plans is marked `[x]` and the code shows it. An item marked done that the code does not show is unmarked and counts as unfinished. An item that is done but unmarked gets marked.
- The project's own checks run and pass: build, tests, typecheck and lint as the project defines them.
- A doubt you can settle yourself, by re-running or reading the code, gets settled now. A doubt that cannot be settled means refuse.

## 2. Git against the stated facts

Read `git status`, `git diff` and `git log` for the task's paths. Every claim about what was built, in the PRD, notes, handover and book chapters, must match the diff. Fix a claim that does not. Report files changed outside the task's scope.

## 3. Scope

List the task and every sub-task: split PRDs, child scratchpad directories, and any handover and memory pointer each has. Close children first, and a parent only once all its children are closed.

## 4. Problems outside the scope (ask)

Collect only what is clearly decided as out of scope: what the PRD's Objective excludes, and anything already written down as deferred or undecided. A passing idea or a drift is not collected. A Plans item that needs its own task is split out: it moves into a child scratchpad task, and the parent PRD's Plans point at the child and no longer carry it. Never invent concerns.

For each one, ask the user a clarifying question first: whether it should be tracked at all. Never offer a PRD or a task before that is answered. Create nothing until the user answers. If there is nothing, skip this step and say nothing about it.

## 5. Books

Update the books to match what was built: the ones read at the start of the task, and any chapter that describes code the task changed. Open a chapter to edit it, not to learn from it. Then decide: a new chapter, an update to an existing one, or none (a recon that ended in "not worth pursuing" just archives).

- A chapter is a frozen snapshot of settled facts and decisions: no narrative, no dates, no changelog.
- Tags are one word each. A compound idea becomes several tags.
- A new book is a folder named `title_snake_case[date][tags]`, with the date written once.
- A stale fact in an existing chapter is corrected in place.

## 6. toc.md

Rewrite the book's line in place.

- The line is the folder name and nothing else, with no description.
- The bracketed tags equal the union of the tags on the book's chapters.
- An obsolete book may carry a one-sentence `warning:` naming what to read instead. An unfinished book carries a bare `[WIP]` at the end.

## 7. Task documents

Bring `prd.md`, notes, debug and recon files to their final state, with no stale plan or open question left in them. Append a last entry to `activities.md`.

## 8. Laws

A correction in the activity log that became a standing rule goes to `laws/<slug>.md` and one line in `laws/summary.md`. A rule never goes to memory.

## 9. Archive, in one action

- Move the task's scratchpad directory into `scratchpad/archive/`.
- If the task has a handover: set its Status to "Done, implemented <date>", move it into `handover/archive/`, drop its row from `handover/whiteboard.md` and renumber the rest in place, and delete its `type: project` memory file and `MEMORY.md` line. Memory writes need `lgrass sign i-write-memory`.

Archived copies are the permanent record and are never deleted.

## 10. Stale sweep

Search the remaining documents, READMEs and the toc for the task slug. Fix any reference to something now archived or renamed.

## 11. Report

Facts only, no closing commentary and no remaining-risks section. One line per action: the checks run, the claims corrected, the chapter and toc line, laws added, what was archived, references fixed, and the out-of-scope items raised with the user's answers.
