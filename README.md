# Lemongrass

A control plane for Claude Code.

The idea came from how I usually work with agentic coding. Drop a requirement, ask the model to plan for it, approve or reject the plan, then let it work. For small tasks this works fine. But I wondered how to push this into something more structured: you only deal with decisions, not with managing what the model reads or how it navigates the codebase.

Lemongrass lets you add a project, scans the codebase and builds a semantic map using no model, then gives you a workspace to drop requirements into. The grooming model reads the semantic map, produces tasks with implementation details, and waits for your approval. You approve, reject, or amend per task. Once all tasks are accepted, the executor model reads them and writes the code. You are only involved at the approval step.

A project holds the codebase, its semantic map, and its git branch. A workspace lives under a project. When you want to work on a new requirement, you create a workspace. Workspaces are logically separate from each other but share everything the project has.

The thinking behind the semantic map is in [PASM.md](PASM.md).

---

## The pipeline

```
  add project
        |
        v
  recon engine scans codebase
  builds semantic map, no model involved
        |
        v
  create workspace, drop in requirements
        |
        v
  Grooming session
  model reads the map, annotates symbols, produces task list
        |
        v
  you review tasks -- approve or reject per task, add feedback
        |
        +-- any rejected --> model revises, resubmits
        |
        +-- all approved
                |
                v
        Execution session
        model reads approved tasks, writes the code
                |
                v
        done
```

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

For how the internals work, see [ARCHITECTURE.md](ARCHITECTURE.md).

---

AGPL-3.0
