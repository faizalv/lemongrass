# Lemongrass

Lemongrass is a layer of abstraction built on top of Claude Code. The idea was to build an infrastructure for Claude so it can perform fully autonomous work without losing coherence and keep the code quality consistent after several runs. It runs on your Claude subscription, not an API key. We run Claude Code inside a container via PTY, it basically sees Lemongrass as a proper terminal it talks to.
We built an environment where the codebase is semantically mapped with a tree-sitter-powered engine and a model could store its knowledge when it worked on a feature as a vector that could be searched up by the next model facing the same issue. An environment built for compounding effectiveness the more you worked on a project.

We have a term for the future we bet on, it's not vibe coders but conductors. As in an orchestra, we used to play the violin ourselves, or the cello or the guitar. We know each instrument, as in we know JavaScript, Go, Rust and many more stacks out there. A conductor could play one of those, they'll write the music sheet everyone uses, they know a drum must enter exactly here or there, but they don't touch the instrument themselves because they are conducting an orchestra. Just like software engineers with agentic coding.

In Lemongrass, we separated 2 phases of development into grooming and execution. Grooming is when a model learns the project, weighing options, and then submits for tasks. These tasks are half-cooked implementation details that you can scan for logic failure or if it sounds you could accept it.
Once tasks are approved, the grooming model will do a handover along with knowledge (if any) to the next phase. We could stop here, execute the tasks tomorrow or that very time, no problem the tasks are there, the knowledge is preserved. Once you're ready, an execution phase happens where a model takes the tasks and reads the knowledge, and then quickly implements those in a task block.
What is a task block? Basically a marker, a model could mark start and done a task, and when the done signal is received Lemongrass will send it to you to review. Find a problem in implementation? reject it and give a comment, when the model marks the next task as done they'll get a notification that you rejected the previous task so they can stop the next tasks and amend the one you reject instead.

The fun part is the knowledge system, we decided to ditch markdown format for these and went full into vectorized basic English way. Why? have you ever thought about a problem and realised that you don't fetch unnecessary knowledge about how to make a sourdough? That's the idea, knowledge should be searchable not just retrievable blindly. And there's also PASM, Progressive annotated semantic map you can read here [PASM.md](PASM.md)

But again, this all is just hypothetical theory. Electricity was discovered because of a frog's legs twitching, the future is always started dumb not perfect. Have a nice day!

---

## The pipeline

```
  add project
        |
        v
  recon engine scans codebase
  builds semantic map, no model involved
  (Go via go/ast; PHP, TypeScript, Python, Vue via tree-sitter)
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
        model reads approved tasks and handover knowledge
                |
                v
        for each task: mark start, implement, mark done
        you review each completed task
                |
                +-- rejected --> model gets note at next task done
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
