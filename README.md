# Lemongrass

A control plane for Claude Code. You point it at a codebase, describe what you want built, and it runs a structured session pipeline to plan and write the code. Strictly personal use.

---

## The problem

Claude Code CLI is great but it is designed for interactive use. You sit in front of a terminal, you type, Claude responds. If you want to drive it programmatically, you either pay for the API (different product, different behavior) or you figure out a way to automate the CLI itself.

Lemongrass wraps the Claude Code CLI in a pty, injects input, reads output, and uses Claude Code's own hook system to intercept tool calls. The result is something that behaves like an API but runs on your existing Claude subscription.

The other thing it solves is context bloat. Letting a model loose on a whole codebase is expensive and usually counterproductive. Lemongrass builds a semantic map of your codebase -- descriptions of every symbol, searchable by meaning -- so the grooming model reasons from that instead of reading raw files. It only touches the code it actually needs.

---

## How it works

The core mechanism is two things working together.

**PTY control.** Claude Code runs inside a Docker container (`lg-runner`). The Go server spawns it via `docker exec` wrapped in a pty allocated through `script(1)`. This makes Claude think it has a real terminal. The server writes to stdin and reads from stdout. No API call involved.

**Hook interception.** Claude Code has a hook system that fires shell scripts on tool use events. Lemongrass configures a `PreToolUse` hook that intercepts every Bash call Claude makes. If the command starts with `#lg.`, the hook POSTs it to the Go server and blocks until it gets a response. If it starts with `#lg!.`, it fires async and returns `ok` immediately. Everything else passes through `rtk` for output compression before going back to Claude.

This is a clean RPC-like protocol where Claude is the client and lemongrass is the server. Claude just thinks it is running bash commands.

```
  Claude emits: Bash("#lg.recon.search 'user authentication'")
        |
        v
  lg-hook reads $CLAUDE_TOOL_INPUT
        |
        +-- #lg.  prefix --> POST /api/lg  -----------------------> lg-server
        |                    block until response <---------------- handler result
        |                    print to stdout
        |                    Claude reads as tool result
        |
        +-- #lg!. prefix --> POST /api/lg async (fire and forget)
        |                    print {"status":"ok"} immediately
        |                    Claude does not wait
        |
        +-- anything else -> rtk exec <command>
                             print compressed output
                             Claude reads as tool result
```

---

## The semantic map

When you add a project, lemongrass immediately runs a structural analysis pass on the codebase. Every meaningful symbol -- Go functions, methods, types, interfaces, Vue components, Pinia stores, TypeScript exports -- gets a node in the `semantic_nodes` table. All nodes start as `unexplored`.

The analysis is deterministic and fast. It uses the AST of each language, not a model. For Go that is `go/parser`. For Vue it reads `<script setup>` blocks and extracts props and emits. For TypeScript it finds exported declarations. The whole thing runs in milliseconds.

```
  add project
        |
        v
  mapping engine runs all language parsers
  framework parsers first (Vue, Nuxt, Next, Laravel...)
  language parsers on whatever is left (TypeScript, Go...)
        |
        v
  semantic_nodes table populated
  Go: 42 nodes · Vue: 11 nodes · TypeScript: 8 nodes · 0 explored
        |
        v
  Reconnaissance page shows the full symbol map
```

Nodes become `explored` when a model reads the raw code and calls `#lg!.annotate` with a natural language description. That description gets embedded via a local embedding model (`lg-embed`) and stored alongside the node. From that point on, the grooming model can find the symbol by searching for its meaning rather than its name.

The annotation format is compact and consistent. `#lg.recon.read` returns raw code in this shape, and `#lg!.annotate` accepts a description in the same shape:

```
/modules/auth/handler/auth.go:LoginSSO:123-145:"validates SSO token, exchanges for internal JWT":*entity.User
```

---

## Session flow

```
  add project
        |
        v
  mapping engine runs (AST parse, no model)
  all symbols inserted as unexplored
        |
        v
  open workspace --> Grooming session
        |
        |  model calls #lg.recon.tree
        |  sees package map and annotation coverage per package
        |
        |  nodes already annotated?
        |    yes --> #lg.recon.search "keyword"
        |            gets matching nodes with descriptions
        |            reasons from descriptions, no raw code needed
        |
        |    no  --> #lg.recon.read <file:symbol:start-end>
        |            gets raw code in annotate format
        |            #lg!.annotate <file:symbol:start-end>:"desc":return
        |            fire and forget -- model already has the understanding
        |            continues building the semantic map as a side effect
        |
        |  model writes task list with impl details
        |  each task references specific symbols by file:symbol:line
        |
        |  #lg.tasks.checkpoint  ---------> UI shows task list to user
        |  (blocks)              <--------- approve / reject + feedback
        |       |
        |       +-- rejected --> model revises, resubmits (loop)
        |       |
        |       +-- approved
        |                 |
        |                 v
        |           #lg!.handover "summary"
        |
        v
  Executor session (new pty, scoped prompt)
        |
        |  reads approved tasks
        |  #lg.recon.read on each referenced symbol --> gets current code
        |  generates patch
        |  #lg!.patch <file:symbol:start-end>:"<new code>"
        |  Worker applies patch to file asynchronously
        |  #lg!.annotate any new symbols it wrote
        |
        v
  #lg!.done
```

**Grooming** reasons entirely from descriptions, not raw code. It only reads raw files when it hits an unexplored node -- and when it does, it annotates it so the next session doesn't have to. No code is generated during grooming. The user approves a plan, not a diff.

**Executor** takes the approved task list and generates actual code patches. It reads the raw code at exactly the symbols grooming referenced, produces replacements, and dispatches them to the Worker.

**Worker** is not a model. It is a Go mechanism inside lg-server that receives patch jobs from the executor and writes them to the filesystem with per-file locking. One file at a time, no thinking, no retry.

---

## Architecture

```
  HOST
  +---------------------------------------------------------+
  |                                                         |
  |  browser / lemongrass CLI       ~/.lemongrass/          |
  |  localhost:9966                 config.json             |
  |                                 claude/      (creds)    |
  |                                 projects/    (state)    |
  |                                 postgres/    (data)     |
  |                                 redis/       (data)     |
  |                                 logs/                   |
  +----------------------------+----------------------------+
                               | :9966 (only exposed port)
                               |         bind mounts
  . . . . . . DOCKER (lemongrass network) . . . . . . . . .
                               |
                       +-------v--------+
                       |   lg-server    |
                       |   :9966        +---------> lg-postgres
                       |                +---------> lg-redis
                       |   pty manager  +---------> lg-embed
                       |   session orch |
                       |   worker       |
                       +-------+--------+
                               | docker exec + pty (stdin/stdout)
                       +-------v--------+
                       |   lg-runner    |
                       |                |
                       |  claude CLI    |
                       |  lg-hook       |
                       +----------------+
  . . . . . . . . . . . . . . . . . . . . . . . . . . . . .

  lg-embed: local E5-base model, generates embeddings for annotations
  lg-postgres: stores semantic_nodes, sessions, tasks, workspaces
  lg-redis: cache
```

`~/.lemongrass/` is bind-mounted into all containers. No Docker volumes. Wipe Docker completely, run `lemongrass up`, everything is back.

Project folders are bind-mounted at `/projects/<alias>` inside the containers so both lg-server and lg-runner can reach the code.

---

## Go server module layout

The server follows a strict module pattern. Every domain is a module implementing a two-method interface (`LoadMe` for wiring dependencies, `StartHTTPRouter` for registering routes). Modules do not import each other directly. Cross-module calls go through a client package or the event bus.

```
modules/
  pty/      PTY session management, the core mechanism
  lg/       #lg command routing and dispatch
  recon/    semantic mapping engine -- multi-language, multi-framework
  fs/       project management and filesystem browsing
```

The `recon` module is worth explaining. It uses `go/parser` + `go/ast` for Go, and equivalent AST parsers for other languages. Parsers are registered behind a `lang.Parser` interface with a `Priority()` method. Framework parsers (Vue, Nuxt, Next, Laravel) run first and claim the files they understand. Language parsers (TypeScript, PHP) run on whatever is left. Adding a new language means implementing the interface and registering it -- nothing else changes.

---

## Commands

Nine commands across two session types. `recon.read` and `annotate` are shared.

| Command | Session | Blocking | Purpose |
|---|---|---|---|
| `#lg.recon.tree [path]` | grooming | yes | package map with annotation coverage |
| `#lg.recon.search <query>` | grooming | yes | vector search across annotated nodes |
| `#lg.recon.read <file:symbol:start-end>` | both | yes | raw code in annotate format |
| `#lg!.annotate <file:symbol:start-end>:"desc":return` | both | no | store description, generate embedding |
| `#lg.tasks.checkpoint <json>` | grooming | yes | submit tasks, block until approved |
| `#lg!.handover "summary"` | grooming | no | end grooming, queue executor |
| `#lg.tasks.read` | executor | yes | get approved tasks |
| `#lg!.patch <file:symbol:start-end>:"<code>"` | executor | no | send patch to worker |
| `#lg!.done "summary"` | executor | no | close executor session |

---

## Getting started

Requirements: Go, Node, Docker.

```shell
make lemongrass
lemongrass auth
lemongrass up
```

`auth` opens the Claude Code auth flow inside `lg-runner` and writes credentials to `~/.lemongrass/claude/`. The host machine does not need Claude Code installed.

`lemongrass up` generates a Docker Compose file from `~/.lemongrass/config.json` and starts all containers. The UI is at `http://localhost:9966`. Add a project from the UI -- lemongrass maps it immediately and the Reconnaissance page shows the full symbol tree.

---

MIT
