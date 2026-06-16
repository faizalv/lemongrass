# Lemongrass -- Architecture

## What It Is

Lemongrass is an intelligence layer that runs alongside Claude Code. It holds state the model cannot hold: a living symbol map, a knowledge store, session workbenches, and structured artifacts. These persist across sessions, models, and projects. The model itself is stateless; Lemongrass is not.

The core concept is PASM -- Progressive Annotated Semantic Map. Every annotation adds a vector-indexed description to a symbol. Every session that annotates leaves the map denser. Denser maps mean less raw reading in future sessions. The grooming and execution pipeline is one way to drive that accumulation, but not the only one.

Five layers make up the platform:

- **Semantic Map** -- parsed symbols with annotations, embeddings, and staleness tracking
- **Knowledge System** -- free-form architectural memory with vector search and labels
- **Workbench** -- session-scoped vector index for targeted analysis
- **Artifacts** -- structured outputs that can be exported, imported, and shared across projects
- **Rules** -- hook-level enforcement for annotation obligation and tool restrictions

---

## Containers

```
  HOST
  +--------------------------------------------------+
  |                                                  |
  |  browser / lemongrass CLI     ~/.lemongrass/     |
  |  localhost:9966                                  |
  |                               config.json        |
  |  Claude Code (host session)   claude/   (creds)  |
  |  lg-hook-host ............... lg.sock  (IPC)     |
  |  PreToolUse / PostCompact     projects/ (state)  |
  |  PostToolUse                  postgres/ (data)   |
  |                               logs/              |
  |                               workspaces/        |
  |                               grammars/ (lang)   |
  |                               device.json        |
  |                               lg-daemon.pid      |
  |                               lemongrass.sock    |
  +-------------------------+------------------------+
                            | :9966 (only exposed port)
                            |        bind mounts (including lg.sock)
  . . . . . DOCKER (lemongrass network) . . . . . . .
                            |
                    +-------v--------+
                    |   lg-server    |
                    |   :9966        +-----> lg-postgres
                    |                +-----> lg-embed
                    |   pty manager  +-----> lg-lang
                    |   session orch |
                    |   lg.sock      |
                    +-------+--------+
                            | docker exec + pty (stdin/stdout)
                    +-------v--------+
                    |   lg-runner    |
                    |                |
                    |   claude CLI   |
                    |   lg-hook      |
                    +----------------+
  . . . . . . . . . . . . . . . . . . . . . . . . . .

  lg-embed:    local E5-base model, POST /embed for annotation vectors
  lg-lang:     tree-sitter parser service, POST /parse for external language symbol extraction
  lg-postgres: projects, workspaces, tasks, semantic nodes, knowledge, artifacts
```

`lg-server` is the Go HTTP server. It manages sessions, spawns PTY processes into `lg-runner`, routes `#lg.*` calls to the right handlers, enforces the annotation obligation, and serves the embedded Vue frontend. It listens on a Unix socket at `~/.lemongrass/lg.sock` (bind-mounted from the host) for direct connections from `lg-hook-host`.

`lg-runner` is where Claude Code runs in PTY mode. It has the Claude CLI and `lg-hook` installed. It holds no state.

`lg-hook-host` is the host-side hook binary. `lemongrass up` installs it and registers it in `~/.claude/settings.json` as PreToolUse, PostToolUse, and PostCompact hooks. It intercepts Bash, Read, Write, Edit, and Skill calls in any Claude Code session running on the host and routes `#lg.*` commands to lg-server over the Unix socket.

`lg-postgres` stores everything persistent: projects, workspaces, tasks, the semantic map, knowledge entries, and artifacts.

`lg-embed` runs a local `intfloat/e5-base` embedding model baked into the image at build time. No network access at runtime. Called by lg-server after each annotation and knowledge save to generate embedding vectors.

`lg-lang` is a Go HTTP server that uses CGo to call the tree-sitter C library. It loads language grammar parsers (`.so` files) at startup via `dlopen`, walks project directories, and returns symbol nodes in the wire protocol format.

All data lives at `~/.lemongrass/` on the host. Everything in there is bind-mounted into the containers, so the containers carry no state. Project folders mount at `/projects/<alias>` into both `lg-server` and `lg-lang`.

---

## Filesystem Daemon

The `lemongrass` CLI runs a background daemon process (`lg-daemon`) on the host for filesystem operations that cannot be performed from inside the `lg-server` container. It listens on a Unix socket at `~/.lemongrass/lemongrass.sock`.

**BROWSE** walks the host filesystem using a parallel worker pool and returns all directories as a stream. Feeds the filesystem browser in the Add Project modal.

**VALIDATE `<path>`** checks whether a path looks like a project root or a container directory. Applies a three-tier check: hardcoded system path blacklist, sub-project count (3+ immediate subdirs with their own project markers), and large directory fallback (10+ children, no root marker). Returns `OK` or `WARN`.

---

## Language Parsers

The Go parser (`go/ast`) runs in-process inside `lg-server`. It handles all `.go` files and is always active.

All other languages are handled by `lg-lang`. When a project is mapped, `lg-server` calls `POST http://lg-lang:3000/parse` with the project path and active `.lgignore` patterns. `lg-lang` applies every loaded grammar to matching files and returns all symbol groups in a single response.

Grammars are compiled tree-sitter parsers -- shared libraries exposing a single C function (`tree_sitter_php()`, `tree_sitter_typescript()`, etc.). `lg-lang` loads them at startup via `dlopen`:

```
Grammar search order:
  1. ~/.lemongrass/grammars/<lang>.so   user-installed, takes precedence
  2. /app/grammars/<lang>.so            bundled in the Docker image
```

Each supported language ships an S-expression query file (`.scm`) embedded in the `lg-lang` binary at build time. The query file defines structural patterns that match node types in the CST and assigns named captures to the parts that matter (`@symbol`, `@receiver`, `@params`, `@return_type`, `@node`). A per-language extraction function reads those captures and builds the `parsedNode` structs that become semantic map rows.

Every semantic node has a `kind` field (e.g. `vue-method`, `trait`, `blade`). The system uses `KindRole(kind)` to map kinds to cross-language roles (`method`, `func`, `type`, `component`, etc.) for internal logic like the annotation obligation threshold. The model always sees the raw `kind`.

`lemongrass setlang ts,php` writes the language list to `~/.lemongrass/config.json` and restarts `lg-lang`. `lemongrass rmlang php` removes it.

---

## The Semantic Map

Every symbol from every file in a registered project gets a row in `lg_semantic_nodes`. The recon engine builds this at project add time without any model and maintains it through file-sync on every edit.

Three statuses:

- `unexplored` -- parsed, no annotation yet
- `explored` -- annotated; has description, return type, dependencies, and an embedding vector
- `stale` -- source changed since last annotation; old description kept as a hint but flagged

The `explored` to `stale` transition is automatic. When a file is re-parsed, any symbol whose `content_hash` differs from the stored value is set to `stale`. The old annotation is preserved and surfaced as a `[STALE]` prefix in `#lg.recon.peruse` responses.

The embedding vector is generated alongside the annotation. Every `#lg.annotate` call triggers async embedding via lg-embed. `#lg.recon.search` queries those vectors.

`#lg.recon.tree` shows coverage at all depths. `#lg.recon.peek <dir>` shows direct-child symbols with status markers: `?` for unexplored, `*` for stale, nothing for explored.

---

## Knowledge System

A key-value store for things the semantic map cannot hold: non-obvious patterns, architectural decisions, cross-cutting constraints, anything a session needs to know that cannot be derived from code structure alone. Entries are not scoped to a single project and can flow between projects via the artifact system.

Every entry is vectorized on save. `#lg.knowledge.search` queries those vectors. A dedup check runs on every `knowledge.save` -- if an entry with cosine distance below 0.20 already exists, the response includes `[similar: key]` so the model can decide whether to merge or keep both.

Labels are first-class. Entries carry label lists. `#lg.knowledge.labels` does a vector search across label space, letting the model find entries by concept without knowing the exact key.

The handover mechanism passes selected knowledge entries to the execution session as a preamble. The grooming model calls `#lg!.handover key1,key2` to name which entries the executor needs. The server stores them as `handover_context` on the workspace. The execution session receives them as a block at the top of its system prompt.

---

## Workbench

`#lg.codebase.interim` loads files and symbols into a session-scoped vector index. `#lg.codebase.query` queries that index semantically. The pattern is: declare what is relevant, then query it multiple times rather than re-reading raw files on each question.

Inputs to interim are flexible: `F:relpath` for a full file, `S:path:symbol:kind` for a specific symbol body, `R:glob` for a file pattern. Mixed inputs work in one call.

Workbench contents are scoped to the session ID. Multiple sessions on the same project have separate workbenches with no bleed between them. Calling `#lg.codebase.interim` again resets the workbench for that session before loading new content.

After 3 consecutive `#lg.codebase.search` calls without loading a workbench, the server appends a one-time nudge to the response.

---

## Artifacts

Artifacts are exportable units of intelligence produced from any layer of the platform: annotations from the semantic map, entries from the knowledge store, or structured outputs from grooming and execution sessions. Anything Lemongrass holds can become an artifact.

They can be exported from one context and imported into any other -- same project, different project, different team. This is the primary mechanism for moving accumulated knowledge across project boundaries without re-deriving it. A data model annotated in one codebase, an architectural decision captured in knowledge, a task breakdown from a grooming session -- all are candidates.

Federation extends this to shared artifact repositories accessible to any registered project.

---

## Actor Modes

Two modes. Both use the same lg-server, semantic map, knowledge store, workbench, and `#lg.*` protocol. From the server's perspective they are identical -- same session handling, same obligation enforcement, same command routing. The difference is how Claude Code runs and how the hook connects to the server.

### PTY Mode

Claude Code runs inside `lg-runner` (Docker) via `docker exec` wrapped in a PTY allocated through `script(1)`. `lg-hook` is registered as the PreToolUse binary inside the container.

This is the mode for structured workspace sessions: grooming, execution, amendment. The workspace pipeline, task checkpoints, and execution lock all belong here.

**Bash** routing:

```
  Claude emits: Bash("#lg.recon.search user authentication")
        |
        v
  lg-hook reads tool event from stdin
        |
        +-- #lg.  prefix --> POST /api/lg, wait for response (up to 10 min)
        |                    allow + updatedInput: command = printf '<response>'
        |                    Claude runs the printf, sees response as tool result
        |
        +-- #lg!. prefix --> POST /api/lg (fire and forget)
        |                    allow + updatedInput: command = printf 'ok'
        |
        +-- permitted cmd -> sh -c <command> run locally in lg-runner
        |                    (git log/diff/show/status/blame, pwd, wc, echo)
        |                    output delivered back via updatedInput
        |
        +-- file reader    -> deny (cat, head, tail; use #lg.system.read)
        +-- write redirect -> deny (file writes go through the Write tool)
        +-- destructive cmd -> deny with guidance to use #lg.echo
        +-- ls/find/grep   -> deny (use codebase.ls, codebase.fl, codebase.search)
        +-- anything else  -> deny with permitted command list
```

**Write** is allowed, locked, and traced. The hook acquires a session-scoped file lock before the write and releases it after. The file path and byte count are logged to the write trail, which triggers obligation accumulation for any symbols the model had previously read from that file.

**Read** is gated. Images pass through. Documents (PDF, DOCX, XLSX) are intercepted and converted via markitdown, delivering a markdown version. Files over 10 KB require a line range. Files with lines over 2,000 characters are rejected with guidance to use `#lg.system.read`.

### Headless Mode

Claude Code runs natively on the host -- no Docker, no PTY, no container spawning. `lg-hook-host` is registered as PreToolUse, PostToolUse, and PostCompact hooks in `~/.claude/settings.json` by `lemongrass up`. It connects to lg-server over the Unix socket at `~/.lemongrass/lg.sock`.

This is the mode for direct codebase work: a developer running Claude Code in their editor or terminal against a registered project. No workspace is required. The model calls `#lg.*` freely in any order and works against the full platform -- semantic map, knowledge, workbench, obligation, artifacts -- without any pipeline or approval step.

**Bash** routing follows the same rules. `#lg.*` and `#lg!.*` go to lg-server. Inside the project, ls, find, and grep are blocked with the same cooldown system. Permitted commands (git read operations, pwd, echo, wc) are allowed through -- the host Claude Code session executes them natively rather than via `sh -c` inside a container.

**Write and Read** gating is identical to PTY mode. The obligation system, warning cooldowns, and skill-reload enforcement all run through the same hook logic, just compiled with `isHost = true` and connecting over the socket instead of HTTP.

PostCompact fires at the start of each new context window. The hook posts a compact notification to lg-server and sets a skill-reload flag in the session directory. The next tool call blocks until the model reloads the skill.

---

## Hook Enforcement

The hook system operates at PreToolUse and has final say over every tool call before it executes.

### Warning Cooldowns

Educational redirect messages suppress themselves after the first fire until a per-kind cooldown expires. The model gets the full redirect message on the first occurrence and a terse block on subsequent ones within the window. Each warning kind has its own independent timer, stored as a timestamp file in the session directory (`/tmp/lg-hook-{sessionID}/`).

| Kind | Cooldown | Trigger |
|---|---|---|
| `grep-blocked` | 5 min | grep used inside the project |
| `ls-blocked` | 5 min | ls used inside the project |
| `find-blocked` | 5 min | find used inside the project |
| `file-reader-blocked` | 5 min | cat/head/tail attempted |
| `read-docs-redirect` | 3 min | Read attempted on a document file |
| `read-large` | 2 min | file too large without a range |
| `search-quotes` | 3 min | codebase.search called with quoted pattern (server-side) |

Safety warnings always fire with no cooldown: `dangerouslyDisableSandbox`, destructive operations, git approval operations, write redirect, skill-reload, skill-compact.

### Annotation Obligation

Every symbol the model peruses with `unexplored` or `stale` status enters the session obligation. Every file the model writes pushes all symbols it had previously read from that file into obligation.

The model has 5 minutes to annotate them. The clock starts when the first symbol enters obligation and resets when obligation reaches zero. Within the first 2m30s the model works freely. After that, every `#lg.*` response carries a countdown suffix. After 5 minutes, all `#lg.*` calls block except `#lg.annotate` and `#lg.obligation`.

`#lg.obligation` shows the current symbol list and time remaining. Annotating a symbol removes it. A model working entirely in well-annotated code carries no obligation. One that reads unexplored symbols or edits files it has previously read accumulates it.

---

## Annotation Commitment

Before the grooming model can submit a task checkpoint it must declare annotation scope via `#lg.commitment <path>`. Each commitment sets a coverage requirement: at least 30% of method and func nodes under that prefix must have been read via `#lg.recon.peruse` and annotated via `#lg.annotate` in the same session.

The read-first rule is strict: annotations not preceded by a peruse call for the exact `path:symbol:kind` triple score zero toward the threshold.

A commitment to `.` (root) requires that 70% of the project is already explored before it is accepted.

`#lg.commitment.status` reports progress per commitment. The server rejects a checkpoint with shortfalls and returns a per-commitment breakdown.

---

## Device Awareness

`lemongrass up` detects the host hardware on startup and writes a device profile to `~/.lemongrass/device.json`. This includes memory (MB), CPU cores, and a tier classification:

- `high` -- 16 GB RAM or more
- `mid` -- 8 GB RAM or more
- `low` -- less than 8 GB RAM
- `unknown` -- could not detect

`#lg.project.stat` surfaces this alongside annotation coverage and recommends tooling based on the combination.

---

## Session Flow

The grooming and execution flow is the structured pipeline for UI-driven work. Headless sessions skip this flow entirely and call `#lg.*` directly in any order.

```
  add project
        |
        v
  recon engine runs (no model)
  all symbols inserted as unexplored nodes
        |
        v
  create workspace, add requirements
        |
        v
  Grooming session
        |
        | #lg.project.stat      -- coverage, device tier, tooling advice
        | #lg.recon.tree [path] -- full coverage map; n/m explored; n stale per dir
        | #lg.recon.peek <dir>  -- direct-child symbols with status markers
        | #lg.recon.search      -- vector search across annotated nodes
        | #lg.recon.peruse      -- raw source; counts toward obligation
        | #lg.annotate          -- write annotation; clears obligation entry
        | #lg.knowledge.*       -- save and retrieve architectural insights
        | #lg.commitment        -- declare annotation scope before checkpoint
        |
        | #lg.tasks.checkpoint  -----> UI shows task list
        | (blocks)              <----- approve / reject per task
        |      |
        |      +-- any rejected --> feedback sent to model --> amending session
        |      |
        |      +-- all approved --> #lg!.handover key1,key2
        |
        v
  workspace: awaiting_execution
        |
        v
  Execution session
        |
        | #lg.tasks.read           -- get approved task list
        | for each task:
        |   #lg.tasks.start <n>    -- mark in_progress
        |   navigate, read, edit
        |   #lg.annotate           -- re-annotate every touched symbol
        |   #lg.tasks.finish <n>   -- mark done; capture diff
        |
        | #lg!.done
        |
        v
  workspace: done
```

---

## #lg Protocol

`#lg.` blocks until the server responds. `#lg!.` fires and returns immediately. "both" means PTY sessions and headless sessions alike.

| Command | Session | Blocking | Purpose |
|---|---|---|---|
| `#lg.obligation` | both | yes | annotation debt: unexplored/stale symbols touched; time remaining; blocks after 5 min |
| `#lg.project.stat` | both | yes | annotation coverage, device tier, tooling advice |
| `#lg.recon.tree [path]` | both | yes | full coverage map; explored/total/stale per directory |
| `#lg.recon.peek <dir\|file>` | both | yes | direct-child symbols with status markers; subdirectory counts |
| `#lg.recon.search <query>` | both | yes | vector + full-text search across annotated nodes; top 10 deduplicated |
| `#lg.recon.peruse <path:symbol:kind>` | both | yes | raw source from semantic map; `[STALE]` prefix on stale nodes; pipe-separate for multiple |
| `#lg.recon.related <path:symbol:kind>` | both | yes | callers and callees from the call graph |
| `#lg.commitment <path>` | grooming | yes | declare annotation scope; 30% threshold; root requires 70% project coverage |
| `#lg.commitment.status` | grooming | yes | per-commitment progress; call before checkpoint |
| `#lg.knowledge.save <key>:<content> [labels]` | both | yes | upsert a knowledge entry; `[similar: ...]` dedup signal on response |
| `#lg.knowledge.read <key>` | both | yes | retrieve a knowledge entry by key |
| `#lg.knowledge.search <query>[:<label>]` | both | yes | vector search across knowledge; top 5 results |
| `#lg.knowledge.labels [query]` | both | yes | list all labels or vector search for relevant ones |
| `#lg.knowledge.delete <key>` | both | yes | remove a knowledge entry |
| `#lg.annotate <path:symbol:kind>:"desc":return:deps` | both | yes | store annotation; generate embedding; clears obligation entry |
| `#lg!.recon.drop <path>` | execution | no | remove all nodes for a path from the semantic map |
| `#lg.tasks.checkpoint <json>` | grooming | yes | submit task list; blocks until user approves or rejects |
| `#lg!.handover [key1,key2,...]` | grooming | no | end grooming; optional key list stored as execution preamble |
| `#lg.tasks.read` | execution | yes | get approved task list with title, reason, impl, task_id |
| `#lg.tasks.start <task_id>` | execution | yes | mark task in_progress; response includes pending rejection note if any |
| `#lg.tasks.finish <task_id>:<notes>` | execution | yes | mark task done; capture per-task diff |
| `#lg!.done` | execution | no | end execution; workspace moves to done |
| `#lg.codebase.interim <inputs>` | both | yes | load files/symbols into session workbench: `S:path:sym:kind`, `F:path`, `R:glob` |
| `#lg.codebase.query <question>` | both | yes | semantic search across everything loaded into the workbench |
| `#lg.codebase.search <pattern> [path/]` | both | yes | grep replacement; supports regex; 2-line context; no quotes around pattern |
| `#lg.codebase.ls [path]` | both | yes | directory listing with sizes and child counts |
| `#lg.codebase.fl <pattern> [path]` | both | yes | find files by name or glob; grouped by directory |
| `#lg.system.read <path>` | both | yes | inspect docs/markdown; warns if >150 lines or >10k chars |
| `#lg.system.read.confirm <path> [N-M]` | both | yes | deliver docs/markdown unconditionally; optional 1-indexed line range |
| `#lg.echo <message>` | both | no | send a status message to the UI activity feed |

---

## Workspace States

```
  idle --> grooming --> awaiting_execution --> executing --> done
                  ^              |
                  |    rejected  |
                  +-- amending --+
```

`amending` is entered when a checkpoint is rejected and an amendment session is started. The model revises the rejected tasks and resubmits a new checkpoint. On approval the workspace returns to `awaiting_execution`.

A project can have multiple workspaces but only one can be in `executing` at a time. The execution lock blocks a second executor from starting on the same project. Grooming and amendment are not affected by it. A crashed executor can be force-stopped from the UI, which resets the workspace to `awaiting_execution` and releases the lock.
