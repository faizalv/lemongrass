# Lemongrass

An agent orchestrator: an Electron shell for running coding-agent CLIs (Claude Code today) in managed terminal panes, paired with `lgrass`, a Go CLI those agents invoke directly for project knowledge and session awareness.

## Layout

- `ui/` -- the Electron app. Terminal panes, project/layout management, window chrome.
- `lgrass/` -- the Go CLI. Knowledge (write/search/read, grouped into books with chapters), and hooks that give an agent session awareness of other sessions in the same project.

The two are deliberately siblings, not nested inside each other -- neither toolchain's file tree (`node_modules`/`tsconfig*` vs. `go.mod`/`go.sum`) sits inside the other's, and neither side's own scripts reach across that boundary. `lgrass` is bundled inside the Electron app at build time and self-installs onto `PATH` when the app launches, so the two stay in permanent version lockstep without a separate release pipeline.

## Build

```
make build-linux
```

Cross-compiles `lgrass` for `linux/amd64` and `linux/arm64`, then builds and packages the Electron app for Linux. This is the one entrypoint above both toolchains -- `ui/`'s own `npm run build:linux` only builds the Electron half and expects `lgrass/dist/` to already exist, so use the root `make` target rather than running it directly.

mac and Windows packaging aren't wired up yet.

## Develop

```
cd ui && npm run dev
```

`lgrass` isn't required for the Electron shell itself to run -- it's invoked by the agent CLI running inside a terminal pane, not by the app directly. For work on `lgrass` alone:

```
cd lgrass && go build ./... && go test ./...
```
