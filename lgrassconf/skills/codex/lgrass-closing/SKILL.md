---
name: lgrass-closing
description: How to close a finished task and its sub-tasks under the bibliothek convention. Use when work is done and needs its book chapter, archive, whiteboard and memory cleanup, or when the user says close, finish or archive a task. Not for "wrap it up", which means saving an undecided idea as a scratchpad note.
---

Invoking this skill is the go-ahead for every step below except the ones marked ask. Run the steps in order. Never run git commands.

## 1. Verify it is done

Compare the handover Status and the PRD Plans against the code. If a phase is not done, stop and tell the user what remains. Never close unfinished work.

## 2. Scope

List the task and every sub-task: split handovers, child scratchpad directories, and each one's memory pointer. Close children first. Close a parent only once all its children are closed. A scratchpad-only task (recon, debug, notes) has no handover, whiteboard row or memory pointer, so skip those steps for it.

## 3. Book

Decide: new chapter, update to an existing chapter, or none (a recon that ended in "not worth pursuing" just archives).

- A chapter is a frozen snapshot of settled facts and decisions. No narrative, no dates, no changelog, no "then X happened".
- Tags are one word each. A compound idea becomes several tags.
- A new book is a folder named `title_snake_case[date][tags]`, where the date is its creation date, written once.
- Correct a stale fact in an existing chapter in place.

## 4. toc.md

Rewrite the book's line in place, never append.

- The bracketed tags equal the union of the tags on the book's chapters.
- The description names scope, not chapters.
- An unfinished book carries a bare `[WIP]` at the end of the line and nothing else.

## 5. Laws

If the task's activity log holds corrections that became standing rules, each one goes to `laws/<slug>.md` and a one-line entry in `laws/summary.md`. A rule never goes to memory.

## 6. Archive, in one action

- Set the handover Status to the final state: "Done, implemented <date>".
- Move the handover file into `handover/archive/`.
- Move the task's scratchpad directory into `scratchpad/archive/`.

Archived copies are the permanent record. Never delete them.

## 7. Whiteboard

Drop the task's row from `handover/whiteboard.md` and renumber the remaining rows in place.

## 8. Memory

Delete the task's `type: project` memory file and its `MEMORY.md` line. One pointer per handover, so a parent and each child are each retired. Memory writes need `lgrass sign memory-feedback-law`.

## 9. Stale sweep

Search the remaining documents, READMEs and toc for the task slug. Fix any reference that points at something now archived or renamed.

## 10. Open issues (ask)

Collect what is still open from the PRD, the handover Status and the activity log: deferred work, known gaps, unanswered decisions.

For each issue, ask the user:

1. Create a scratchpad task for it? Use `notes.md` if it is not decided yet, and `prd.md` if it is.
2. If yes, activate it? Activating means a handover, a whiteboard row (added at the priority the user names) and a memory pointer. Declining leaves it as a scratchpad task only.

Create nothing until the user answers. Never open a handover for an issue the user has not asked to activate.

## 11. Report

One line per action taken: the chapter written or updated, the toc line, laws added, what was archived, the whiteboard rows dropped, the memory pointers deleted, the references fixed, and the open issues raised with the user's answers.
