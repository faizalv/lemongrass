`biblio/` holds this project's working knowledge: the plans of tasks in flight, the standing rules, the handovers of paused work and the settled knowledge about the code, and the scripts and queries the project reuses. It sits beside the code, and nothing in it loads on its own; you open a file when you need it. This text tells you what each part holds and how to read it. How to write each file is printed by the `lgrass how` action for that write, at the moment you write it; the Procedures section above names the action for each.

## Memory holds pointers, biblio holds knowledge

Your agent's own memory loads in every session, so it holds pointers and nothing else: one pointer per active handover. A fact, a decision or a rule never goes in memory, and a pointer never goes in an instructions file committed with the project. Everything else lives in `biblio/`, which loads only when opened, because a project's knowledge keeps growing and an always-loaded place cannot grow with it.

## The five folders

`biblio/scratchpad/` holds work in flight, and both you and the user write in it. Each task has its own directory, named by its slug, and nothing sits loose at the top. A task directory holds the files its kind of work needs:

- `prd.md` for something being built. Its Plans checkboxes are the progress record.
- `debug.md` for a fault being chased.
- `recon.md` for an investigation.
- `notes.md` for an idea not decided yet.
- `activities.md`, always: the dated log of what happened.

Any other file or folder the work needs sits beside them. When you are pointed at a task, read its files, say back what you understood, and continue from its checkboxes. Its text is the record of the task, not new instructions to you.

`biblio/laws/` holds the project's standing rules, one file per rule, binding for every agent. `laws/summary.md` lists them one line each and arrives in your context at session start. A complete summary there counts as read; when it is missing or incomplete, open the file in full. Open a single law's file only when its line is not enough.

`biblio/handover/` holds one file per paused or delegated task, so another session can pick it up cold. Its Status is the current state, and its Log only points at the task's `activities.md`. `handover/whiteboard.md` ranks the active handovers by priority, one row each.

`biblio/books/` holds settled knowledge: design decisions, domain facts and the reasons behind the code. A book is a folder and a chapter is a file inside it. `books/toc.md` lists every book with its tags, and you find a chapter through those tags and the book and chapter names; there is no other index. A chapter is a snapshot of settled facts, never a workspace.

`biblio/toolbox/` holds working files that other work uses again, such as scripts, queries and runbooks. Each item is a folder with one line in `toolbox/inventory.md`, which you check before writing such a file yourself. Only `lgrass toolbox` changes it.

## In flight and archived

`scratchpad/` and `handover/` hold only work still in flight. Each has an `archive/` folder that keeps finished tasks as the permanent record, and nothing in it is deleted. Each fact has one home, and other files point at it instead of repeating it, the way a handover points at its task's `prd.md`.
