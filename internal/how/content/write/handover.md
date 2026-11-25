A handover is for one paused or delegated task, picked up cold by another session. It points at its task's `prd.md` and never at another scratchpad file. Its Status holds current state only: where to resume, what is next and what is blocked. What is done lives in the PRD's marks, the plan in the PRD, and the history in `activities.md`, so Status never restates them. No dates, no branch names or deploy state.

The file is `biblio/handover/<slug>.md`:

```markdown
# <Task>: handover

<one-line summary>. Design and phases: `biblio/scratchpad/<slug>/prd.md`.

## Status (authoritative, the Log below is history only)

<where to resume, what is next, what is blocked>

## Log (history only, not authoritative)

See `biblio/scratchpad/<slug>/activities.md`.
```

Add a pointer to the matching `biblio/books/` chapter on the summary line if one exists.

Its row in `biblio/handover/whiteboard.md` is `N. <slug> [tags] -> <slug>.md`, at its priority position with the rows below it renumbered. Tags follow the book tag rules, and `blocked` marks a handover stalled on a decision.
