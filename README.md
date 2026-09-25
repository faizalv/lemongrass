# Lemongrass

An agent orchestrator: an Electron shell for running coding-agent CLIs (Claude Code today) in managed terminal panes, paired with `lgrass`, a Go CLI those agents invoke directly for project knowledge and session awareness.

## Layout

- `ui/` -- the Electron app. Terminal panes, project/layout management, window chrome, a `biblio/` browser/editor, and the Connector panel for database and HTTP access.
- `lgrassconf/` -- a tiny Go daemon (systemd user service) that owns Lemongrass agent configuration. It registers Claude Code and Codex hooks plus each agent's `lgrass-connector` and `lgrass-staleness` skills, kept as one embedded folder per vendor and skill.
- `lgrass/` -- the Go CLI:
  - Session/thread coordination between panes.
  - A `SessionStart` hook that delivers `biblio/laws/summary.md` and gates tool calls behind required skills. It never writes Claude config itself.
  - Identity/signing for inter-session messages.
  - `lgrass db` -- a local credential vault scoping agent access to a project's databases.
  - `lgrass rester` -- the same vault applied to HTTP APIs, handling login and token injection for a chosen user.

The three are deliberately siblings, not nested inside each other -- neither toolchain's file tree (`node_modules`/`tsconfig*` vs. `go.mod`/`go.sum`) sits inside the other's, and neither side's own scripts reach across that boundary. `lgrass` is bundled inside the Electron app at build time and self-installs onto `PATH` when the app launches, so the two stay in permanent version lockstep without a separate release pipeline.

## Bibliothek integration

[bibliothek](https://github.com/faizalv/bibliothek) is the knowledge layer Lemongrass builds on: a skill plus a `biblio/{scratchpad,laws,handover,books}/` folder convention, developed in its own repository and usable without Lemongrass. Lemongrass's own `biblio/` is an instance of it, and `lgrass` delivers a project's `biblio/laws/summary.md` at session start.

The Electron shell surfaces bibliothek's `biblio/` convention as a UI, not just files an agent reads on disk.

- A workspace/biblio toggle in the header switches the main view between terminal panes and a tree-and-reader split -- shown only when the project has a `biblio/` directory.
- The tree lists `books/`, `handover/`, `laws/`, `scratchpad/`, each with its own icon; `books/toc.md` gets a pinned "Table of Content" button.
- Only `scratchpad/` is editable, enforced server-side, through a Tiptap-based WYSIWYG editor that autosaves.

## Skills

`lgrassconf` installs two skills for each supported agent (Claude Code and Codex) and keeps them current.

- `lgrass-connector` -- teaches an agent to run a database query or an HTTP call through an existing `lgrass db` or `lgrass rester` channel, so the real credential never enters the session.
- `lgrass-staleness` -- teaches an agent to act on stale structure without being asked. When a document, book chapter, handover, PRD, `toc.md` line, README, or code comment contradicts the code or the bibliothek convention, the agent verifies the fact against its source and corrects the document in the same turn. Code wins over any document that describes it, dated records such as activity logs stay as written, and it asks only when the current state can't be determined or the fix is a design decision. It also checks structure: `toc.md` tags against the union of chapter tags, handovers pointing only at a `prd.md`, one whiteboard row per active handover, and finished handovers archived together with their scratchpad directory.

Bibliothek itself is installed separately from its own repository.

## lgrass db

A local credential vault for a project's databases.

- Passphrase-derived root key, envelope-encrypted per-connection credentials, short-lived "channels" scoping an agent to specific tables/operations -- the agent never sees a real connection string.
- Two processes: a vault that's the only thing that ever decrypts, and an agent holding only opaque channel keys.
- A Connections/Channels panel in the UI: create/test/delete connections, create/activate/rotate/revoke channels, behind a vault-wide passphrase unlock with auto-lock.
- In progress: a wire-protocol proxy so a whole app can point its own db driver at a local port instead of going through the CLI. Port allocation and the MySQL handshake listener are done; query execution and the Postgres listener aren't yet.

## lgrass rester

An HTTP counterpart to `lgrass db`, sharing the same vault, agent, and channel shape.

- A domain holds a base URL, a login endpoint, and per-user credentials or a pre-supplied token. The vault logs in, caches the token with its expiry, and injects it into each request. A stale token triggers one re-login and retry.
- A channel scopes an agent to a method allow-list plus an exclusion list of method and path pairs, and the agent calls `lgrass rester <short-id> <get|post|put|patch|delete|head|options> <path> [--user <name>]`, with a JSON body inline (`--body`) or from a file (`--body-file`), a raw file with `--content-type`, or `--file` and `--form` for a multipart upload such as a spreadsheet import. A path is resolved onto the domain's base URL and can never address another host. `lgrass rester <short-id> info` prints the base URL, expiry, allowed methods and users, and `lgrass rester <short-id> flush [--user <name>]` drops cached login tokens so the next request logs in again.
- Managed from the same Connector panel as database access, under a Database/HTTP switch.
- Planned: a shared audit log for connector calls, covering `lgrass db` writes as well.

## Build

```
make build-linux
```

Cross-compiles `lgrass` and `lgrassconf` for `linux/amd64` and `linux/arm64`, then builds and packages the Electron app for Linux. This is the one entrypoint above both toolchains -- `ui/`'s own `npm run build:linux` only builds the Electron half and expects `lgrass/dist/` to already exist, so use the root `make` target rather than running it directly.

```
make build-mac
```

Cross-compiles `lgrass` and `lgrassconf` for `darwin/amd64` and `darwin/arm64`, then packages a macOS `.app` / `.dmg` / `.zip` via electron-builder. On first launch the app copies the matching arch binaries into `~/.local/bin` and runs `lgrassconf install` (LaunchAgent). Notarization is off by default.

Windows packaging isn't wired up yet.

## Develop

```
cd ui && npm run dev
```

`make dev` from the repo root installs the `lgrassconf` keeper (systemd user unit on Linux, LaunchAgent on macOS) and builds `lgrass` into `~/.local/bin` first, so Claude Code and Codex configuration are kept correct before a session starts. Codex hook definitions still require review and trust through `/hooks`. `lgrass` isn't required for the Electron shell itself to run -- it's invoked by the agent CLI running inside a terminal pane, not by the app directly. New shell panes prompt for which CLI to spawn (Claude Code, Codex, Cursor Agent, or a plain shell). For work on `lgrass` alone:

```
cd lgrass && go build ./... && go test ./...
```
