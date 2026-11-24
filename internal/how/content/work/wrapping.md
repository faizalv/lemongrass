Wrap file created: `{path}`

# Wrapping up a session

The human is pausing, and the next session starts cold with nothing but `biblio/`. Work this session raised that no file holds yet is lost unless you persist it now.

1. **List what is not yet persisted.** Collect only work this session raised that no file holds yet: decisions, corrections, ideas, proposed features, findings and open questions, including those about another part of the project. Skip anything that is not about the work. Keep each idea at the scope the human gave it, never only the part that fit the task at hand.
2. **Give each its home.** A task's decisions go in its `prd.md` Design decisions, and its unanswered questions in Open decisions, in the human's terms. Something for another task goes in that task's PRD. An idea with no task gets `biblio/scratchpad/<new-slug>/notes.md` with an `activities.md`. `activities.md` is history and is never the only home.
3. **Hand over open tasks.** For every task with work left, run `lgrass how task handover <slug>`. Its Status names each question waiting for the human, and quotes any wording that waits for approval.
4. **Fill the wrap file.** One line per item from step 1: what it is, then where it now lives. Pointers only; the content stays in its home.
5. **Check it cold.** Read the wrap file, each Status and each home as a session that never saw this conversation. Anything you would still need from the conversation goes in now.
6. **Report.** Say where the wrap file is, in one line.
