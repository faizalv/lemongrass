# Development

How the code is laid out, how agents get their procedures, and how to build and run Lemongrass from source.

## How it fits together

- `ui/` is the Electron app: terminal panes, project and layout management, the bibliothek tree and reader, and the Vault tab.
- `cmd/lgrass` is the Go CLI models and users call: session and thread coordination, `lgrass db`, `lgrass rester`, `lgrass ask`, and `lgrass workgroup`.
- `cmd/lgrassd` is the binary only the app and the agents' hooks call, installed off `PATH`: the hooks and their gates, the vault, agent and node daemons, tab records and account management.
- `cmd/lgrassconf` is a small Go daemon, a systemd user service on Linux, a LaunchAgent on macOS and a Task Scheduler logon task on Windows. It registers the Claude Code and Codex hooks, installs the Codex skills and keeps every Claude account's hooks and shared folders in place.
- `internal/` holds the Go packages the three binaries share, such as the vault, the guard, the hook engine, the session store and the account manager. It is importable only from inside this repository.

The Go code is one module at the repo root, and `ui/` is the second toolchain beside it. Neither side's scripts reach across that boundary except through the `dist/` binaries. The binaries are bundled inside the Electron app at build time, and `lgrass` installs itself onto `PATH` when the app launches, so the two stay on the same version without a separate release pipeline.

## Procedures: `lgrass how`

The procedures an agent follows live in `lgrass how`, embedded in the `lgrass` binary. `lgrass how <domain> <action>` prints the steps for one action at the moment it is taken. Some actions are manuals with fixed text, and others read the project and print only the branch that fits it. `lgrass how` lists the domains.

| Action | What it gives an agent |
|---|---|
| `work task`, `work research`, `work debug`, `work discussion` | How to work each kind of task: read the related books, plan first, keep the PRD or the working file current, wait for the user before code. |
| `task handover` | Pause or hand over a task: the handover file, its whiteboard row and memory pointer, or a refusal when the work is finished or has no PRD. |
| `task close` | Close a finished task: verify it is done, compare git with the stated facts, update the books and `toc.md`, archive, and sweep stale references. |
| `book write` | Where a book chapter goes, and how to write it. |
| `laws write` | Write a standing law the user approved, and its summary line. |
| `ask request` | How to ask the human with `lgrass ask`: the YAML for each type, the reason, and what each answer means. |
| `keeper sweep` | Keep `biblio/` and the code in agreement: the structural drift it finds itself, then the checks that need reading. |
| `leader create`, `leader addmember`, `thinker start`, `collaborator join` | Workgroup roles. |

Claude tabs learn these from their orientation, and no lemongrass skill is installed for Claude. `lgrassconf` still installs the older skills for Codex, which has no orientation yet. Bibliothek is not among them: Claude tabs get it in their system prompt, and a Codex tab or a Claude session outside Lemongrass uses it only if it is installed separately from its own repository.

## Build

```
make build-linux
```

Cross-compiles `lgrass`, `lgrassd` and `lgrassconf` for `linux/amd64` into `dist/`, then builds and packages the Electron app for Linux. This is the one entrypoint above both toolchains. `ui/`'s own `npm run build:linux` builds only the Electron half and expects `dist/` to exist already, so use the root `make` target. `make verify` compiles every Go package without writing a binary into the source tree. Every Go target first checks out the bibliothek submodule, and `make bibliothek-update` moves its pin to bibliothek's latest commit. It builds AppImage, snap, deb and rpm, so it needs `dpkg` and `snapcraft` on the host.

```
make build-rpm
```

Same cross-compile, then builds only the rpm (needs `rpmbuild`), for Fedora and other rpm distributions. The package installs the app under `/opt/Lemongrass` and registers no service. On each launch the app copies `lgrass` and `lgrassconf` into `~/.local/bin` and `lgrassd` into `~/.lemongrass/bin`, then runs `lgrassconf install`, which writes the keeper's systemd user unit. Removing the package prints a reminder to run `lgrassconf uninstall`, described under [Uninstall](install.md#uninstall).

```
make build-mac
```

Cross-compiles `lgrass`, `lgrassd` and `lgrassconf` for `darwin/amd64` and `darwin/arm64`, then packages a macOS `.app`, `.dmg` and `.zip` via electron-builder. On first launch the app copies the matching arch binaries into `~/.local/bin` and runs `lgrassconf install` (LaunchAgent). The app is signed ad hoc and not notarized, since both need an Apple Developer ID.

```
make build-windows
```

Cross-compiles the three binaries for `windows/amd64` as `.exe`, then builds the per-user NSIS `setup.exe` on Linux. electron-builder runs the installer once under the system wine to extract the uninstaller, so the host needs wine with 32-bit support. On first launch the app copies `lgrass` and `lgrassconf` into `%USERPROFILE%\.local\bin`, adds that folder to the user's `Path`, and runs `lgrassconf install`, which registers the keeper's logon task. The `setup.exe` is unsigned.

## Develop

```
make install   # builds and installs lgrass, lgrassd and lgrassconf
make dev       # builds the UI once and launches it
```

`make install` builds `lgrass` into `~/.local/bin` and `lgrassd` into `~/.lemongrass/bin`, installs the `lgrassconf` keeper (systemd user unit on Linux, LaunchAgent on macOS) and moves the bibliothek pin to its latest commit, so Claude Code and Codex configuration are correct before a session starts. All three binaries are built first and only then moved into place, so a failed install leaves the previous set untouched and says so.

`make dev` installs nothing. It builds the UI with no hot reload into a snapshot under `ui/.snapshot/` and launches Electron from that snapshot, so editing the source never reaches a running instance; run `make dev` again to pick changes up. If the UI build fails it launches the newest earlier snapshot with a warning, and it uses the binaries `make install` last installed. Codex hook definitions still require review and trust through `/hooks`. `lgrass` is not required for the Electron shell itself to run. The agent CLI running inside a terminal pane invokes it, not the app directly. New shell panes prompt for which CLI to spawn (Claude Code, Codex, Cursor Agent, or a plain shell). `cd ui && npm run dev` is still the hot-reloading way to work on the UI alone. For work on the Go code alone, run these from the repo root:

```
go build -a ./... && go test ./...
```
