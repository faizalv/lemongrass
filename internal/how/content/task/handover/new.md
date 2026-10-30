1. **Marks.** Make sure every finished Plans item in `biblio/scratchpad/{slug}/prd.md` is `[x]`, and append a dated entry to `activities.md`.
2. **File.** Write `biblio/handover/{slug}.md`:

   ```markdown
   # <Task>: handover

   <one-line summary>. Design and phases: `biblio/scratchpad/{slug}/prd.md`.

   ## Status (authoritative, the Log below is history only)

   <where to resume, what is next, what is blocked>

   ## Log (history only, not authoritative)

   See `biblio/scratchpad/{slug}/activities.md`.
   ```

   Add a pointer to the matching `biblio/books/` chapter on the summary line if one exists.
3. **Whiteboard.** Add `N. {slug} [tags] -> {slug}.md` to `biblio/handover/whiteboard.md` at its priority position and renumber the rows below it. Tags follow the book tag rules, and `blocked` marks a handover stalled on a decision.
4. **Memory pointer.** Write a `type: project` memory file whose body is one or two sentences pointing at `biblio/handover/{slug}.md`, never the content, and add its line to `MEMORY.md`. Memory writes need `lgrass sign i-write-memory`.
