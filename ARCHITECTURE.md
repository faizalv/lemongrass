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
  |                               grammars/ (lang)   |
  |                               lg-daemon.pid      |
  |                               lemongrass.sock    |
  +-------------------------+------------------------+
                            | :9966 (only exposed port)
                            |        bind mounts
  . . . . . DOCKER (lemongrass network) . . . . . . .
                            |
                    +-------v--------+
                    |   lg-server    |
                    |   :9966        +-----> lg-postgres
                    |                +-----> lg-embed
                    |   pty manager  +-----> lg-lang
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

  lg-embed:    local E5-base model, POST /embed for annotation vectors
  lg-lang:     tree-sitter parser service, POST /parse for external language symbol extraction
  lg-postgres: projects, workspaces, tasks, semantic nodes
```

`lg-server` is the Go HTTP server. It manages sessions, spawns PTY processes into `lg-runner`, routes `#lg` calls to the right handlers, and serves the embedded Vue frontend.

`lg-runner` is where Claude Code actually runs. It has the Claude CLI and the `lg-hook` binary installed. It holds no state of its own.

`lg-postgres` stores everything that needs to persist: projects, workspaces, tasks, and the semantic map.

`lg-embed` runs a local `intfloat/e5-base` embedding model. Baked into the image at build time -- no network access at runtime. Called by `lg-server` after each annotation to generate the vector for semantic search.

`lg-lang` is a Go HTTP server that uses CGo to call the tree-sitter C library. It loads language grammar parsers (`.so` files) at startup via `dlopen`, walks project directories, and returns symbol nodes in the wire protocol JSON format. See the Language parsers section below.

All data lives at `~/.lemongrass/` on the host. Everything in there is bind-mounted into the containers, so the containers carry no state. Project folders get mounted at `/projects/<alias>` into both `lg-server` and `lg-lang`.

---

## Filesystem daemon

The `lemongrass` CLI runs a background daemon process (`lg-daemon`) on the host for filesystem operations that cannot be performed from inside the `lg-server` container. It listens on a Unix socket at `~/.lemongrass/lemongrass.sock`.

The daemon handles two commands:

**BROWSE** -- walks the host filesystem from `/` using a parallel worker pool (concurrency controlled by `FsConcurrency` in config, default 8). Returns all directories as a newline-separated stream. Feeds the filesystem browser in the Add Project modal.

**VALIDATE `<path>`** -- checks whether a path looks like a project root or a container directory. Applies a three-tier check: hardcoded system path blacklist, sub-project count (3+ immediate subdirs with their own project markers), and large directory fallback (10+ children, no root marker). Returns `OK` or `WARN` followed by warning lines and `END`.

`lg-server` calls the daemon over the socket for both operations -- it cannot see the host filesystem directly.

---

## Language parsers

The Go parser (`go/ast`) runs in-process inside `lg-server`. It handles all `.go` files and is always active.

All other languages are handled by `lg-lang`. When a project is mapped, `lg-server` calls `POST http://lg-lang:3000/parse` with the project path and the active `.lgignore` patterns. `lg-lang` applies every loaded grammar to matching files and returns all symbol groups in a single response.

### Grammar loading

Grammars are compiled tree-sitter parsers -- shared libraries exposing a single C function (`tree_sitter_php()`, `tree_sitter_typescript()`, etc.). `lg-lang` loads them at startup via `dlopen`:

```
Grammar search order:
  1. ~/.lemongrass/grammars/<lang>.so   user-installed, takes precedence
  2. /app/grammars/<lang>.so            bundled in the Docker image
```

### Symbol extraction

Loading a grammar gives `lg-lang` the parser. Extracting symbols from a parsed file is a separate step driven by tree-sitter queries.

Each supported language ships an S-expression query file (`.scm`) embedded in the `lg-lang` binary at build time. The query file defines structural patterns that match specific node types in the concrete syntax tree and assigns named captures to the parts that matter:

```scheme
; example: PHP top-level function
(program
  (function_definition
    name: (name) @symbol
    parameters: (formal_parameters) @params
    return_type: (_)? @return_type) @node)
```

When `lg-lang` processes a file it runs all patterns in the query against the CST in a single pass. Each match returns a set of captures keyed by name -- `@node` (the full declaration span), `@symbol` (the identifier), `@receiver` (method receiver if present), `@params`, `@return_type`, and so on. A per-language extraction function reads those captures and builds the `parsedNode` structs that become semantic map rows.

The query files are compiled at `lg-lang` startup alongside the grammar binary. If the query references a node type or field name that does not exist in the compiled grammar, `ts_query_new` returns a compile error and the grammar is not loaded -- the error log will show the byte offset and error type (NodeType, Field, or Structure) to pinpoint the bad pattern.

### setlang / rmlang

`lemongrass setlang ts,php` writes the language list to `~/.lemongrass/config.json` and restarts the `lg-lang` container. `lg-lang` reads `LG_LANGUAGES` from its environment at startup and loads only the listed grammars. A project with no configured languages gets only the Go and config parsers.

`lemongrass rmlang php` removes the language from config and restarts `lg-lang`.

### Development vs production

In the current development phase, grammar `.so` files are compiled from source and bundled inside the Docker image at `/app/grammars/`. `setlang` activates them by config -- no download needed.

In the production phase (planned), `setlang` will download pre-compiled grammar binaries for the current platform from GitHub Releases into `~/.lemongrass/grammars/`, giving independent grammar version management without rebuilding the image.

### KindRole

Every semantic node has a `kind` field (e.g. `vue-method`, `trait`, `blade`). The system uses `KindRole(kind)` to map kinds to cross-language roles (`method`, `func`, `type`, `component`, etc.) for internal logic like the annotation quota. The model always sees the raw `kind` -- roles are never exposed to it.

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
        | #lg.recon.tree [path]
        | no arg = root directories only; pass a path to drill one level deeper
        | run iteratively until reaching the target directory
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
        |      +-- any rejected --> feedback sent to model --> amending session
        |      |                    model revises tasks, resubmits checkpoint
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

The Go parser runs in-process. External languages (TypeScript, Vue, Python, PHP, and others configured via `setlang`) are parsed by `lg-lang` over HTTP. All parsers produce the same `ParseResult` format; the engine merges all groups and upserts them together.

Every symbol starts `unexplored`. The grooming model reads raw source via `#lg.recon.read`, understands it, and writes an annotation via `#lg!.annotate`. This transitions the node to `explored` and triggers embedding generation. If the source changes between scans the node goes `stale` -- the old description is kept as a hint but flagged until the model re-reads and re-annotates.

The executor model re-annotates every symbol it writes or modifies, so the map stays current after execution.

The semantic map compounds across sessions. Every grooming session enriches it. By the third or fourth session on related requirements the model can navigate most of the codebase entirely through search and `#lg.recon.related` without opening any raw files.

---

## #lg protocol

Commands the model uses to communicate with Lemongrass inside a session. `#lg.` blocks -- the hook waits for the server response before returning to Claude. `#lg!.` fires and returns immediately.

| Command | Session | Blocking | Purpose |
|---|---|---|---|
| `#lg.recon.tree [path]` | grooming | yes | one level deep from root or given path; drill iteratively then peek |
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
                  ^              |
                  |    rejected  |
                  +-- amending --+
```

`amending` is entered when a checkpoint is rejected and an amendment session is started. The model revises the rejected tasks and resubmits a new checkpoint. On approval the workspace returns to `awaiting_execution`.

A project can have multiple workspaces but only one can be in `executing` at a time. The execution lock blocks a second executor from starting on the same project. Grooming and amendment are not affected by it. A crashed executor can be force-stopped from the UI, which resets the workspace to `awaiting_execution` and releases the lock.
