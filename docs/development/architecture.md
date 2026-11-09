# How it fits together

- `ui/` is the Electron app: terminal panes, project and layout management, the bibliothek tree and reader, and the Vault tab.
- `cmd/lgrass` is the Go CLI models and users call: session and thread coordination, `lgrass db`, `lgrass rester`, `lgrass ask`, and `lgrass workgroup`.
- `cmd/lgrassd` is the binary only the app and the agents' hooks call, installed off `PATH`: the hooks and their gates, the vault, agent and node daemons, tab records and account management.
- `cmd/lgrassconf` is a small Go daemon, a systemd user service on Linux, a LaunchAgent on macOS and a Task Scheduler logon task on Windows. It registers the Claude Code and Codex hooks, installs the Codex skills and keeps every Claude account's hooks and shared folders in place.
- `internal/` holds the Go packages the three binaries share, such as the vault, the guard, the hook engine, the session store and the account manager. It is importable only from inside this repository.

The Go code is one module at the repo root, and `ui/` is the second toolchain beside it. Neither side's scripts reach across that boundary except through the `dist/` binaries. The binaries are bundled inside the Electron app at build time, and `lgrass` installs itself onto `PATH` when the app launches, so the two stay on the same version without a separate release pipeline.
