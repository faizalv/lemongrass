# Procedures: `lgrass how`

The procedures an agent follows live in `lgrass how`, embedded in the `lgrass` binary. `lgrass how <domain> <action>` prints the steps for one action at the moment it is taken. Some actions are manuals with fixed text, and others read the project and print only the branch that fits it. `lgrass how` lists the domains.

| Action | What it gives an agent |
|---|---|
| `work task`, `work research`, `work debug`, `work discussion` | How to work each kind of task: read the related books, plan first, keep the PRD or the working file current, wait for the user before code. |
| `task handover` | Pause or hand over a task: the handover file, its whiteboard row and memory pointer, or a refusal when the work is finished or has no PRD. |
| `task close` | Close a finished task: verify it is done, compare git with the stated facts, update the books and `toc.md`, archive, and sweep stale references. |
| `book write` | Where a book chapter goes, and how to write it. |
| `laws write` | Write a standing law the user approved, and its summary line. |
| `ask request` | How to ask the human with `lgrass ask`: the YAML for each type, the reason, and what each answer means. |
| `keeper sweep` | Keep `biblio/` and the code in agreement: the structural drift it finds itself, then the checks that need reading. |
| `leader create`, `leader addmember`, `thinker start`, `collaborator join` | Workgroup roles. |

Claude tabs learn these from their orientation, and no lemongrass skill is installed for Claude. `lgrassconf` still installs the older skills for Codex, which has no orientation yet. Bibliothek is not among them: Claude tabs get it in their system prompt, and a Codex tab or a Claude session outside Lemongrass uses it only if it is installed separately from its own repository.
