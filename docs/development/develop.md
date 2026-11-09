# Develop

```
make install   # builds and installs lgrass, lgrassd and lgrassconf
make dev       # builds the UI once and launches it
```

`make install` builds `lgrass` into `~/.local/bin` and `lgrassd` into `~/.lemongrass/bin`, installs the `lgrassconf` keeper (systemd user unit on Linux, LaunchAgent on macOS) and moves the bibliothek pin to its latest commit, so Claude Code and Codex configuration are correct before a session starts. All three binaries are built first and only then moved into place, so a failed install leaves the previous set untouched and says so.

`make dev` installs nothing. It builds the UI with no hot reload into a snapshot under `ui/.snapshot/` and launches Electron from that snapshot, so editing the source never reaches a running instance; run `make dev` again to pick changes up. If the UI build fails it launches the newest earlier snapshot with a warning, and it uses the binaries `make install` last installed. Codex hook definitions still require review and trust through `/hooks`. `lgrass` is not required for the Electron shell itself to run. The agent CLI running inside a terminal pane invokes it, not the app directly. New shell panes prompt for which CLI to spawn (Claude Code, Codex, Cursor Agent, or a plain shell). `cd ui && npm run dev` is still the hot-reloading way to work on the UI alone. For work on the Go code alone, run these from the repo root:

```
go build -a ./... && go test ./...
```
