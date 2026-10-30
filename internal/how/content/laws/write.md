# Writing a law

Write a law only after the user said an explicit yes to it in chat. A question, a reword request or silence is not a yes: revise and offer again. After a no, never offer it again.

1. Write `biblio/laws/<slug>.md`, the slug in snake_case:

   ```markdown
   # <Law title>

   <the rule, stated as an instruction>

   **Why:** <rationale>
   **How to apply:** <concrete guidance>
   ```

   Keep it terse and timeless, a handful of lines: no dates, no history of when it recurred, no session narrative.

2. In the same turn, add one line for it to `biblio/laws/summary.md` in the same form as the lines already there: the rule from the file's opening line, then the path to the file.
3. Never write a law to memory. If a `feedback` memory entry holds the same rule, delete that file and its `MEMORY.md` line. Memory writes need `lgrass sign i-write-memory`.

To change an existing law, edit its file in place and rewrite its summary line in place.
