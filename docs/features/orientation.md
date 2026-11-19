# Every agent starts with an orientation

Every Claude Code, Codex and Antigravity tab Lemongrass opens starts with an orientation and the project's laws. Claude tabs get it appended to their system prompt, Codex tabs inside the model instructions file Lemongrass writes for them, and Antigravity tabs as a system message from its `SessionStart` hook. Claude and Codex tabs of one project share the same text, so they read it from the prompt cache. Antigravity has no way to add to its system prompt, so its copy rides in the conversation instead.

The orientation is onboarding, not procedure. It covers where the tab is and what reaches it, every `lgrass` command and when to use which, what the guard blocks, and how knowledge splits: the code and its comments are committed and owned by the team, while books are each person's own. It also says what a good comment is. A resumed session gets the current laws again from its hook. Sessions in the same project know about each other, and the hook warns when two tabs touch the same file.

## Procedures: `lgrass how`

The steps an agent follows for a kind of work live in `lgrass how`, embedded in the `lgrass` binary, not in the orientation. The orientation only says which action fits which situation, so changing a procedure never rewrites the cached prompt. `lgrass how <domain> <action>` prints the steps for one action at the moment it is taken. Some actions are manuals with fixed text, and others read the project and print only the branch that fits it. `lgrass how` lists the domains.

| Action | What it gives an agent |
|---|---|
| `work task`, `work research`, `work debug`, `work hotfix`, `work discussion` | How to work each kind of task: read the related books, plan first, keep the PRD or the working file current, wait for the user before code. |
| `task handover` | Pause or hand over a task: the handover file, its whiteboard row and memory pointer, or a refusal when the work is finished or has no PRD. |
| `task close` | Close a finished task: verify it is done, compare git with the stated facts, update the books and `toc.md`, archive, and sweep stale references. |
| `book write` | Where a book chapter goes, and how to write it. |
| `laws write` | Write a standing law the user approved, and its summary line. |
| `ask request` | How to ask the human with `lgrass ask`: the YAML for each type, the reason, and what each answer means. |
| `keeper sweep` | Keep `biblio/` and the code in agreement: the structural drift it finds itself, then the checks that need reading. |
| `leader create`, `leader addmember`, `thinker start`, `collaborator join` | Workgroup roles. |

No Lemongrass skill holds these procedures. The orientation is the only place a tab learns which action to run.

## The `biblio` folder

The convention behind this is [bibliothek](https://github.com/faizalv/bibliothek): a skill plus a `biblio/{scratchpad,laws,handover,books}/` folder layout, developed in its own repository and usable without Lemongrass. Lemongrass pins it as a git submodule and puts its text into every tab's orientation, so tabs never load it as a skill. The app shows that folder as a tree and reader beside the terminal panes. The scratchpad is editable in a WYSIWYG editor that autosaves, enforced on the server side. The other folders are read-only. While the app runs, every change under `biblio/` is committed within seconds to a private git history in `~/.lemongrass/history/`, never the project's own repository, and kept for 90 days. Agents read it with `lgrass biblio log` and `lgrass biblio show` to recover a file a bad edit damaged. The history needs git: the rpm and deb packages depend on it, and elsewhere the history stays off with a message saying git is missing.
