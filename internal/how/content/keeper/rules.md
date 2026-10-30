# Keeper sweep

Keep `biblio/` and the code in agreement. Running this is the go-ahead for every fix below: verify the current fact against its source, then fix the stale document in the same turn. Ask only when the current state cannot be determined or the fix is a design decision.

Git is read-only here: `log`, `status`, `diff` and `show` only.

Source of truth: code wins over any document that describes it, a book chapter included. The bibliothek convention wins over how a project lays out its `biblio/`. A PRD wins over a handover for design, and a handover's Status wins for progress. Dated records of what happened, activity logs and archived tasks, stay as written.
