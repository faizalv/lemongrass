# Lemongrass

A control plane for Claude Code.

The idea came from how I usually work with agentic coding. Drop a requirement, ask the model to plan for it, approve or reject the plan, then let it work. For small tasks this works fine. But I wondered how to push this into something more structured: you only deal with decisions, not with managing what the model reads or how it navigates the codebase.

Lemongrass lets you add a project, scans the codebase and builds a semantic map using no model, then gives you a workspace to drop requirements into. The grooming model reads the semantic map, produces tasks with implementation details, and waits for your approval. You approve, reject, or amend per task. Once all tasks are accepted, the executor model reads them and writes the code. You are only involved at the approval step.

A project holds the codebase, its semantic map, and its git branch. A workspace lives under a project. When you want to work on a new requirement, you create a workspace. Workspaces are logically separate from each other but share everything the project has.

---

## How it works

The easier path would have been the API or `claude -p`. But Anthropic limits SDK usage on subscription plans, which gets in the way of building something more customised on top of Claude Code.

PTY worked, so that is what we use. Claude Code runs inside a Docker container (`lg-runner`). The Go server spawns it via `docker exec` wrapped in a pty allocated through `script(1)`. The pty makes Claude think it has a real terminal. The server writes to stdin to inject prompts and reads stdout to capture output.

The second piece is hook interception. Claude Code fires shell scripts on tool use events. Lemongrass writes a `PreToolUse` hook into each workspace before starting a session. Any Bash call Claude makes goes through this hook first. Commands prefixed with `#lg.` are POSTed to the Go server and block until a response comes back. Commands prefixed with `#lg!.` fire async and return immediately. Everything else is evaluated locally inside `lg-runner` against a whitelist -- git read-only commands, cat, grep, etc. Anything not on the list is rejected with guidance.

```
  Claude emits: Bash("#lg.recon.search 'user authentication'")
        |
        v
  lg-hook reads $CLAUDE_TOOL_INPUT
        |
        +-- #lg.  prefix --> POST /api/lg  ---------> lg-server
        |                    block until response <-- handler result
        |                    print to stdout
        |                    Claude reads as tool result
        |
        +-- #lg!. prefix --> POST /api/lg (fire and forget)
        |                    print ok immediately
        |                    Claude does not wait
        |
        +-- permitted cmd -> sh -c <command>
        |                    print output
        |
        +-- anything else -> reject with guidance
```

---

## Architecture

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

All data lives at `~/.lemongrass/` on the host. Everything in there is bind-mounted into the containers, so the containers carry no state. Project folders get mounted at `/projects/<alias>` so both the server and the runner can access them.

---

## Session flow

```
  add project
        |
        v
  recon engine runs (no model involved)
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
        | nodes already annotated?
        |   yes --> #lg.recon.search "keyword"
        |           gets matching nodes with stored descriptions
        |           reasons from descriptions, no raw reading needed
        |
        |   no  --> #lg.recon.read <file:symbol:start-end>
        |           gets raw source at those lines
        |           #lg!.annotate <file:symbol:start-end>:"desc":return:[calls]
        |           fire and forget -- model keeps moving
        |           semantic map enriched as a side effect
        |
        | model writes task list with impl details
        |
        | #lg.tasks.checkpoint  -------> UI shows task list to user
        | (blocks)              <------- approve / reject + feedback per task
        |      |
        |      +-- any rejected --> model revises those tasks, resubmits
        |      |
        |      +-- all approved
        |                |
        |                v
        |          #lg!.handover
        |
        v
  workspace: awaiting_execution
        |
        v
  Execution session  [not yet built]
```

---

## Semantic map

The recon engine scans the codebase on project add and on a configurable sync interval. It builds one row per symbol in `lg_semantic_nodes` using language parsers -- no model involved at this stage.

Every symbol starts `unexplored`. The grooming model reads raw source via `#lg.recon.read`, understands it, and writes an annotation via `#lg!.annotate`. This transitions the node to `explored` and triggers embedding generation. If the source changes between scans the node goes `stale` -- the old description is kept as a hint but flagged until the model re-reads and re-annotates.

The semantic map compounds across sessions. Every grooming session enriches it. By the third or fourth session on related requirements the model can navigate most of the codebase entirely through search and `#lg.recon.related` without opening any raw files.

---

## #lg protocol

Commands the model uses to communicate with lemongrass inside a session. `#lg.` blocks. `#lg!.` fires and returns immediately.

| Command | Session | Blocking | Purpose |
|---|---|---|---|
| `#lg.recon.tree [path]` | grooming | yes | directory map with annotation coverage |
| `#lg.recon.search <query>` | grooming | yes | vector similarity search across annotated nodes |
| `#lg.recon.read <file:symbol:start-end>` | both | yes | raw source at those lines; [STALE] prefix on stale nodes |
| `#lg.recon.related <symbol>` | grooming | yes | callers and callees from the call graph |
| `#lg!.annotate <file:symbol:start-end>:"desc":return:[calls]` | both | no | store description, return type, calls; generate embedding |
| `#lg.tasks.checkpoint <json>` | grooming | yes | submit task list, block until user approves or rejects |
| `#lg!.handover` | grooming | no | end grooming, workspace moves to awaiting_execution |
| `#lg!.echo <message>` | both | no | send a message visible in the UI activity feed |

---

## Getting started

Requirements: Go, Node, Docker.

```shell
make lemongrass
lemongrass auth
lemongrass up
```

`auth` opens the Claude Code auth flow inside `lg-runner` and writes credentials to `~/.lemongrass/claude/`. The host machine does not need Claude Code installed.

`lemongrass up` generates a Docker Compose file from `~/.lemongrass/config.json` and starts all containers. The UI is at `http://localhost:9966`. Add a project from the UI and the Reconnaissance page shows the full symbol tree with coverage stats.

---

AGPL-3.0
