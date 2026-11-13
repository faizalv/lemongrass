## While you work

- Wait for the user's go before changing code or running anything with side effects outside `biblio/`. One go covers every open Plans item through to the end. Keep going from phase to phase without asking, and stop only at a phase marked `(review)`, at a contradiction, or before changing the core idea: the Problem, the Expectation, the Objective, a settled design decision, or dropping a Plans item. Writing under `biblio/` never waits.
- When a decision is made or an item is finished: write the decision into the task's files in place, flip the item to `- [x]` once you have verified it yourself (build, tests, a run), and append a dated entry to `activities.md` for a decision, shipped work or a correction.
- If the work hits a contradiction, such as a missing piece or a design that cannot be built as written, stop and tell the user in one or two plain sentences with your recommendation instead of working around it.
- A problem outside the Objective is not worked here. Offer the user a new scratchpad task for it, or, when a task covers the same domain, adding it to that task's PRD. A Plans item big enough to be tracked on its own is offered as a split into its own task.
- Pausing or handing the work to someone else: `lgrass how task handover {slug}`. Done: `lgrass how task close {slug}`.
