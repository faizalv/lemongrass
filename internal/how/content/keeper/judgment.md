These need reading, so they apply whatever the findings above say:

1. **Memory pointers.** One `type: project` pointer per active handover. A pointer whose handover is archived or missing is deleted with its `MEMORY.md` line. Memory writes need `lgrass sign i-write-memory`.
2. **Git against PRD marks.** For each active PRD, compare its Plans marks with the code and with `git log` for the paths involved. An item the code shows as shipped gets marked `[x]`, and a mark the code does not support is cleared. Work that looks fully finished is reported as ready to close, not closed here.
3. **Books against code.** Check each chapter's claims against the code. A stale fact is corrected in place, with no changelog and no dated narrative. A book that is wholly obsolete gets a `warning:` on its toc line naming what to read instead.
