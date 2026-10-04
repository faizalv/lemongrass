---
name: lgrass-closing
description: How to close a finished task and its sub-tasks under the bibliothek convention. Use when work is done and needs its book chapter, archive, whiteboard and memory cleanup, or when the user says close, finish or archive a task. Not for "wrap it up", which means saving an undecided idea as a scratchpad note.
---

Invoking this skill is the go-ahead for every step below except the ones marked ask. Run the steps in order.

Closing means every in-scope objective is finished with full confidence. Once step 1 passes, the closing carries no hedges: no "what's not covered", "not re-run yet", "not verified", "should work" or caveat section. Anything outside the PRD's Objective is out of scope and is not mentioned.

The other side of that rule: if at any step you find the task is not actually closable, stop and refuse to close. Say it plainly, as "I can't close this because <the specific in-scope objective and the concrete evidence>". A blocker on an in-scope objective is never a caveat, and refusing is always better than closing with doubt. Do not archive, retire memory or edit the whiteboard once you have refused.

## 1. Verify it is done

Compare the handover Status and the PRD Plans against the code, and check each in-scope objective in the PRD's Objective one by one. Every one must be confirmed finished with 100% confidence.

- A doubt you can settle yourself (re-run the tests, rebuild, read the code) gets settled now, before anything else. It never travels into the report.
- If a phase is not done, or a doubt cannot be settled, refuse to close: name the unconfirmed objective and the evidence, then stop. Never close unfinished work, and never close with a caveat.
- The same holds for a stale or contradictory handover Status, a failing build or test, or code that does not match what the PRD says was built. Each one is a reason to refuse, not to note.

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
- The line is the folder name and nothing else, with no description.
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

Collect only what is already written down as deferred, out of scope or undecided in the PRD, the handover Status and the activity log. Never invent concerns, unrelated issues or speculation. If nothing is recorded, skip this step and say nothing about it.

For each issue, ask the user:

1. Create a scratchpad task for it? Use `notes.md` if it is not decided yet, and `prd.md` if it is.
2. If yes, activate it? Activating means a handover, a whiteboard row (added at the priority the user names) and a memory pointer. Declining leaves it as a scratchpad task only.

Create nothing until the user answers. Never open a handover for an issue the user has not asked to activate.

## 11. Report

Facts only, no closing commentary and no remaining-risks section. One line per action taken: the chapter written or updated, the toc line, laws added, what was archived, the whiteboard rows dropped, the memory pointers deleted, the references fixed, and the open issues raised with the user's answers.
