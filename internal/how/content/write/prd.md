Lemongrass's tools read a PRD by its headings, so a PRD uses exactly the headings below, in this order, and no others: no extra section, no renamed heading, no other heading level.

```markdown
# <Task title>

## Problem

<what is wrong or missing>

## Expectation

<what is true once it is solved>

## Objective

<what the work sets out to do, and what is out of scope>

## Plans

<how the work is done: a workgroup, subagents or solo, and why>

### Design decisions

- <a settled decision>

### Open decisions

- <a question still waiting for an answer>

### Phase 1: <the deliverable>

- [ ] <a step>

### Phase 2: <the deliverable> (review)

- [ ] <a step>
- [ ] User approval
```

- Problem, Expectation and Objective hold prose or bullets, with no headings of their own.
- `### Open decisions` exists once the task has had an open question. When one is settled, write the decision into Design decisions and cross the open line out by wrapping it in `~~`; never delete it. An open decision is never a checkbox, because every checkbox counts as a step.
- Every phase is a `### Phase N: <deliverable>` heading, a one-phase task included, numbered from 1 in build order. Its steps are `- [ ]` items, flipped to `- [x]` once done and verified, and a step may hold indented `- [ ]` sub-steps. Nothing but steps goes under a phase heading.
- A phase is an agreed, testable deliverable and never crosses a runtime boundary. Split into phases only when the work spans multiple domains or distant features; smaller actions within one domain are steps of one phase. Each part that can be verified on its own gets its own step, and the step says what verifies it.
- End a phase heading with ` (review)` only when the user asked to look at it before the next phase starts, or when a later phase depends on how it turns out.
- The last checkbox in the PRD is `- [ ] User approval`, under the last phase. Only the user's explicit acceptance ticks it, never your own check.
- Write Plans as the simplest reading of the Objective. Every step must be needed to meet an Expectation. If a similar function or an earlier feature suggests adding more, tell the user in one or two sentences with your recommendation and add it only after a yes, because whoever resumes the PRD reads it as the spec.
- The line under `## Plans` says how the work is done: a workgroup for a multi-phase, high-value task, subagents for mostly read-only research or a refactor that needs only writes, solo for a task spanning few files or domains.
