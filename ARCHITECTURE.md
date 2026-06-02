# Lemongrass -- Architecture

## Containers

```
  HOST
  +--------------------------------------------------+
  |                                                  |
  |  browser / lemongrass CLI     ~/.lemongrass/     |
  |  localhost:9966               config.json        |
  |                               claude/   (creds)  |
  |                               projects/ (state)  |
  |                               postgres/ (data)   |
  |                               logs/              |
  |                               workspaces/        |
  +-------------------------+------------------------+
                            | :9966 (only exposed port)
                            |        bind mounts
  . . . . . DOCKER (lemongrass network) . . . . . . .
                            |
                    +-------v--------+
                    |   lg-server    |
                    |   :9966        +-----> lg-postgres
                    |                +-----> lg-embed
                    |   pty manager  |
                    |   session orch |
                    +-------+--------+
                            | docker exec + pty (stdin/stdout)
                    +-------v--------+
                    |   lg-runner    |
                    |                |
                    |   claude CLI   |
                    |   lg-hook      |
                    +----------------+
  . . . . . . . . . . . . . . . . . . . . . . . . . .

  lg-embed: local E5-base model, POST /embed for annotation vectors
  lg-postgres: projects, workspaces, tasks, semantic nodes
```

`lg-server` is the Go HTTP server. It manages sessions, spawns PTY processes into `lg-runner`, routes `#lg` calls to the right handlers, and serves the embedded Vue frontend.

`lg-runner` is where Claude Code actually runs. It has the Claude CLI and the `lg-hook` binary installed. It holds no state of its own.

`lg-postgres` stores everything that needs to persist: projects, workspaces, tasks, and the semantic map.

`lg-embed` runs a local `intfloat/e5-base` embedding model. Baked into the image at build time -- no network access at runtime. Called by `lg-server` after each annotation to generate the vector for semantic search.

All data lives at `~/.lemongrass/` on the host. Everything in there is bind-mounted into the containers, so the containers carry no state. Project folders get mounted at `/projects/<alias>` into both `lg-server` and `lg-runner`.

---

## How sessions work

The easier path would have been the SDK or `claude -p`. But Anthropic limits SDK usage on subscription plans, which gets in the way of building something more customised on top of Claude Code.

PTY worked, so that is what we use. Claude Code runs inside a Docker container (`lg-runner`). The Go server spawns it via `docker exec` wrapped in a pty allocated through `script(1)`. The pty makes Claude think it has a real terminal.

The second piece is hook interception. Claude Code fires a `PreToolUse` hook binary on every tool call. Lemongrass registers `lg-hook` as that binary. The hook reads the tool event from stdin as JSON, does its routing, and writes a JSON response to stdout. The response is either `allow + updatedInput` (rewrite the tool call) or `deny + reason` (block it). Claude Code reads the response before executing anything.

Three tool types are intercepted:

**Bash** routes through three tiers:
```
  Claude emits: Bash("#lg.recon.search 'user authentication'")
        |
        v
  lg-hook reads tool event from stdin
        |
        +-- #lg.  prefix --> POST /api/lg, wait for response (up to 10min)
        |                    allow + updatedInput: command = printf '<response>'
        |                    Claude runs the printf, sees response as tool result
        |
        +-- #lg!. prefix --> POST /api/lg (fire and forget)
        |                    allow + updatedInput: command = printf 'ok'
        |                    Claude does not wait for server
        |
        +-- permitted cmd -> sh -c <command> run locally in lg-runner
        |                    allow + updatedInput: command = printf '<output>'
        |                    (git log/diff/show/status/blame, cat, ls, find, grep, etc.)
        |
        +-- write redirect -> deny (file writes go through the Write tool)
        +-- destructive cmd -> deny with guidance to use #lg.echo
        +-- anything else  -> deny with permitted command list
```

**Write** is always allowed. The hook logs the file path and byte count to the write trail via a fire-and-forget POST to `lg-server`, then exits 0.

**Read** is gated in grooming sessions. PDF, image, markdown, plain text, and log files pass through. Everything else is denied with guidance to use `#lg.recon.read` instead. Execution sessions pass all reads through.

---

## Session flow

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
        | #lg.recon.tree
        | sees package map, annotation coverage per directory
        |
        | #lg.recon.peek <dir>
        | lists all symbols under a directory: kind, name, lines, status
        |
        | nodes already annotated?
        |   yes --> #lg.recon.search "keyword"
        |           gets matching nodes with stored descriptions
        |           reasons from descriptions, no raw reading needed
        |
        |   no  --> #lg.recon.read <path:symbol:kind>
        |           gets raw source at those lines
        |           #lg!.annotate <path:symbol:kind>:"desc":return:deps
        |           fire and forget -- model keeps moving
        |           semantic map enriched as a side effect
        |
        | model writes task list with impl details and reasoning
        |
        | #lg.tasks.checkpoint <json>  -----> UI shows task list
        | (blocks)                     <----- approve / reject per task
        |      |
        |      +-- any rejected --> model revises, resubmits
        |      |
        |      +-- all approved --> #lg!.handover
        |
        v
  workspace: awaiting_execution
        |
        v
  Execution session
        |
        | #lg.tasks.read
        | gets approved task list with title, reason, impl directives
        |
        | for each task:
        |   #lg.recon.peek / #lg.recon.read  -- navigate to relevant symbols
        |   Edit / Write tools               -- write the change directly
        |   #lg!.annotate                    -- re-annotate every touched symbol
        |
        | #lg!.done
        |
        v
  workspace: done
```

---

## Semantic map

The recon engine scans the codebase on project add and on a configurable sync interval. It builds one row per symbol in `lg_semantic_nodes` using language parsers -- no model involved at this stage.

Every symbol starts `unexplored`. The grooming model reads raw source via `#lg.recon.read`, understands it, and writes an annotation via `#lg!.annotate`. This transitions the node to `explored` and triggers embedding generation. If the source changes between scans the node goes `stale` -- the old description is kept as a hint but flagged until the model re-reads and re-annotates.

The executor model re-annotates every symbol it writes or modifies, so the map stays current after execution.

The semantic map compounds across sessions. Every grooming session enriches it. By the third or fourth session on related requirements the model can navigate most of the codebase entirely through search and `#lg.recon.related` without opening any raw files.

---

## #lg protocol

Commands the model uses to communicate with Lemongrass inside a session. `#lg.` blocks -- the hook waits for the server response before returning to Claude. `#lg!.` fires and returns immediately.

| Command | Session | Blocking | Purpose |
|---|---|---|---|
| `#lg.recon.tree [path]` | grooming | yes | directory map with annotation coverage |
| `#lg.recon.peek <dir>` | grooming | yes | all symbols under a directory: kind, name, lines, status |
| `#lg.recon.search <query>` | grooming | yes | vector similarity search; rejected below 80% coverage |
| `#lg.recon.read <path:symbol:kind>` | both | yes | raw source; `[STALE]` prefix on stale nodes |
| `#lg.recon.related <path:symbol:kind>` | grooming | yes | callers and callees from the call graph |
| `#lg!.annotate <path:symbol:kind>:"desc":return:deps` | both | no | store description, return type, deps; generate embedding |
| `#lg!.recon.drop <path>` | execution | no | remove all nodes for a path from the semantic map |
| `#lg.tasks.checkpoint <json>` | grooming | yes | submit task list; blocks until user approves or rejects |
| `#lg!.handover` | grooming | no | end grooming, workspace moves to awaiting_execution |
| `#lg.tasks.read` | execution | yes | get approved task list with title, reason, impl |
| `#lg!.done` | execution | no | end execution, workspace moves to done |
| `#lg.echo <message>` | both | no | send a status message visible in the UI activity feed |

---

## Workspace states

```
  idle --> grooming --> awaiting_execution --> executing --> done
```

A project can have multiple workspaces but only one can be in `executing` at a time. The execution lock blocks a second executor from starting on the same project. Grooming is not affected by it. A crashed executor can be force-stopped from the UI, which resets the workspace to `awaiting_execution` and releases the lock.
