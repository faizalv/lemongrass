# Adding to the toolbox

1. **Check first.** Run `lgrass toolbox list`. When an item already does this job, improve it with `lgrass toolbox update` instead of adding another.
2. **Only reusable work.** Add a file only when work beyond this task will run it again; a file only this task needs stays in its task directory. Never put a credential, a password or a connection string in an item.
3. **Write it where you work.** Write the file in your task directory and run it once, then add it: `lgrass toolbox add <slug> --about "<one sentence on what it is for>" --tags <tag,tag> <file>...`. The slug names what the item does, and tags follow the book tag rules.
4. **When the toolbox is full,** the add is refused. Remove an item no longer used, or merge it into one that covers the same job, then add.
