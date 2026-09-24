# Lemongrass

An agent orchestrator: an Electron shell for running coding-agent CLIs (Claude Code today) in managed terminal panes, paired with `lgrass`, a Go CLI those agents invoke directly for project knowledge and session awareness.

## Layout

- `ui/` -- the Electron app. Terminal panes, project/layout management, window chrome, a `biblio/` browser/editor, and the db vault's Connections/Channels panel.
- `lgrassconf/` -- a tiny Go daemon (systemd user service) that owns Lemongrass agent configuration. It registers Claude Code and Codex hooks plus each agent's `lgrass-connector` and `lgrass-staleness` skills, kept as one embedded folder per vendor and skill.
- `lgrass/` -- the Go CLI:
  - Session/thread coordination between panes.
  - A `SessionStart` hook that delivers `biblio/laws/summary.md` and gates tool calls behind required skills. It never writes Claude config itself.
  - Identity/signing for inter-session messages.
  - `lgrass db` -- a local credential vault scoping agent access to a project's databases.

The three are deliberately siblings, not nested inside each other -- neither toolchain's file tree (`node_modules`/`tsconfig*` vs. `go.mod`/`go.sum`) sits inside the other's, and neither side's own scripts reach across that boundary. `lgrass` is bundled inside the Electron app at build time and self-installs onto `PATH` when the app launches, so the two stay in permanent version lockstep without a separate release pipeline.

## Bibliothek integration

The Electron shell surfaces bibliothek's `biblio/` convention as a UI, not just files an agent reads on disk.

- A workspace/biblio toggle in the header switches the main view between terminal panes and a tree-and-reader split -- shown only when the project has a `biblio/` directory.
- The tree lists `books/`, `handover/`, `laws/`, `scratchpad/`, each with its own icon; `books/toc.md` gets a pinned "Table of Content" button.
- Only `scratchpad/` is editable, enforced server-side, through a Tiptap-based WYSIWYG editor that autosaves.

## lgrass db

A local credential vault for a project's databases.

- Passphrase-derived root key, envelope-encrypted per-connection credentials, short-lived "channels" scoping an agent to specific tables/operations -- the agent never sees a real connection string.
- Two processes: a vault that's the only thing that ever decrypts, and an agent holding only opaque channel keys.
- A Connections/Channels panel in the UI: create/test/delete connections, create/activate/rotate/revoke channels, behind a vault-wide passphrase unlock with auto-lock.
- In progress: a wire-protocol proxy so a whole app can point its own db driver at a local port instead of going through the CLI. Port allocation and the MySQL handshake listener are done; query execution and the Postgres listener aren't yet.

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
