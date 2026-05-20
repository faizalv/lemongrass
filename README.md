# Lemongrass

A control plane for Claude Code.

The idea came from how I usually work with agentic coding. Drop a requirement, ask the model to plan for it, approve or reject the plan, then let it work. For small tasks this works fine. But I wondered how to push this into something more structured: you only deal with decisions, not with managing what the model reads or how it navigates the codebase.

Lemongrass lets you add a project, scans the codebase and builds a semantic map using no model, then gives you a workspace to drop requirements into. The grooming model reads the semantic map, produces tasks with implementation details, and waits for your approval. You approve, reject, or amend. Once all tasks are accepted, the executor model reads them and writes the code. You are only involved at the approval step.

Lemongrass has two levels of scope. A project holds the codebase, its semantic map, and its git branch. A workspace lives under a project. When you want to work on a new requirement, you create a workspace. Workspaces are logically separate from each other but share everything the project has.

---

## How it works

The easier path would have been the API or `claude -p`. But Anthropic limits SDK usage on subscription plans, which gets in the way of building something more customized on top of Claude Code.

PTY worked, so that is what we use. Claude Code runs inside a Docker container (`lg-runner`). The Go server spawns it via `docker exec` wrapped in a pty allocated through `script(1)`. The pty makes Claude think it has a real terminal. The server writes to stdin to inject prompts and reads stdout to capture output. It is a bit flaky compared to a proper API, but it works.

The second piece is hook interception. Claude Code fires shell scripts on tool use events. Lemongrass writes a `PreToolUse` hook into each workspace before starting a session. Any Bash call Claude makes goes through this hook first. Commands prefixed with `#lg.` are POSTed to the Go server and block until a response comes back. Commands prefixed with `#lg!.` fire async and return ok immediately. Everything else passes through `rtk` for output compression before going back to Claude.

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

## Session flow

```
  add project
        |
        v
  semantic analysis
  all symbols inserted as unexplored
        |
        v
  create a workspace --> Grooming session
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
        |            fire and forget, model already has the understanding
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

---

## #lg protocol

These are the commands the model uses to talk to lemongrass, not CLI commands for the user. Nine total across two session types. `recon.read` and `annotate` are available in both.

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

`lemongrass up` generates a Docker Compose file from `~/.lemongrass/config.json` and starts all containers. The UI is at `http://localhost:9966`. Add a project from the UI and the Reconnaissance page shows the full symbol tree.

---

MIT
