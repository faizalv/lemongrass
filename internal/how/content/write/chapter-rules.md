A chapter records how the system works, never how it broke. A fix that restores intended behavior gets no chapter, and its root cause stays in the archived `debug.md`. Write or extend a chapter only when the task settled a new design, mechanism or decision a future reader needs. A recon that ended in "not worth pursuing" just archives.

When a chapter is written or changed:

- It is free-form prose, dense where that helps, and a frozen snapshot of settled facts: no narrative, no dates, no changelog. A stale fact is corrected in place.
- A book is a folder named `title_snake_case` and a chapter a file named `title_snake_case.md`, with no date or tags in either name.
- Tags live only on the book's toc line. Each is one token, lowercase or camelCase, so a compound idea is one camelCase tag. Use the terms the project already uses for the thing, such as a module or domain name, rather than a generic category word, because a tag exists to cut the toc down without opening a book.
- Its `biblio/books/toc.md` line is rewritten in place: `title_snake_case [tag,camelTag]`, plus an optional one-sentence `warning:` for an obsolete book.
- `toc.md`, the book and chapter names and the tags are the whole way to find a chapter. When something is hard to find, fix the grouping, the names or the tags; never add an index, a map or a log beside them.

Archived copies are the permanent record and are never deleted.
