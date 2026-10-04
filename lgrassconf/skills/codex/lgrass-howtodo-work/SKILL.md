---
name: lgrass-howtodo-work
description: Use at the start of any task, before reading source, planning or editing anything. How to work a task from reading the related books to closing it, including planning first, keeping the PRD current as work happens, and when to wait for the user.
---

## 1. Find the books

Read `biblio/books/toc.md` and open every book whose name or tags relate to the task, then the chapters whose names fit. Do this before reading source. Say which books you read. Go to the code only for what no book covers, and where code and a book disagree, the code wins and the chapter is corrected at closing.

## 2. Plan first

Show the plan in chat and write it to the task's scratchpad directory in the same turn: `biblio/scratchpad/<task-slug>/prd.md` with Problem, Expectation, Objective and Plans. Write every Plans item as `- [ ]`. A small task gets a short PRD, not none. For a task that already has a directory, read its `prd.md` and `activities.md` first and continue from the marks, since the marks show what is done.

## 3. What waits for the user

Wait for the user's go-ahead before changing code or running anything with side effects outside `biblio/`. Writing under `biblio/` never waits: the PRD, notes, activity log, books and toc are the model's own responsibility.

## 4. Keep the PRD current

In the same turn that a decision is made or a Plans item is finished:

- Flip the item to `- [x]`, only once it is done and verified.
- Write the decision into the PRD in place.
- Append a dated entry to `activities.md` for a decision, shipped work or a correction.

None of this needs approval. The marks are how anyone, including you tomorrow, knows what is done.

## 5. When approval is needed

Ask the user before changing the core idea: the Problem, the Expectation, the Objective, a settled design decision, or dropping a Plans item. Adding an item inside the same Objective, adding detail, marking done and logging do not need it.

## 6. Out-of-scope findings

A problem outside the Objective is not worked in this task. Offer the user a new scratchpad task for it. A Plans item big enough to be built and tracked on its own is offered as a split: its own task, with the parent's Plans pointing at it.

## 7. Pausing or delegating

Write a handover only when the task is paused or handed to someone else, never for a small task finished in one session. Its Status holds where to resume, what is next and what is blocked, and the PRD marks hold what is done. A handover comes with a whiteboard row and a memory pointer.

## 8. Done

When the work is done, run `lgrass-howtodo-taskclosing`.

Never run `git add`, `git commit` or any history-changing git command unless the user asks for it in that moment.
