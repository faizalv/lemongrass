---
name: lgrass-staleness
description: How to act upon stale structure against the codebase or bibliothek. Use when a document, book chapter, handover, PRD, toc line, README, or code comment contradicts the code or the bibliothek convention.
---

## Act without asking

Verify the current fact against its source, then update the stale document in the same turn. Ask only when the current state cannot be determined or the fix is a design decision. Report each fix as one line in the reply.

## Source of truth

- Code wins over any document that describes it.
- The bibliothek skill wins over how a project lays out its `biblio/`.
- A PRD wins over a handover for design, and a handover's Status wins for progress.

## What stays as written

Dated records of what happened, meaning activity logs and archived tasks.

## Structure checks against bibliothek

- A `toc.md` line's bracketed tags equal the union of the tags on its book's chapters.
- A stale fact in a chapter is corrected in place, with no changelog and no dated narrative.
- A handover points at a `prd.md` only, and its Status names phases as the PRD's Plans names them.
- The whiteboard has one row per active handover and none for an archived one.
- A finished handover is archived together with its scratchpad directory, and its memory pointer is retired.
- A rename propagates to file names, tags, the toc, and every reference.
