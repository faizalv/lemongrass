# Lemongrass

Lemongrass runs coding agents in real terminals and gives them what a raw terminal does not: credentials they can use but never see, teammates they can message, a safety guard that holds with permission prompts off, and project rules that arrive at the start of every session.

It is an Electron app plus `lgrass`, a Go CLI that the agents call themselves. Claude Code and Codex are supported, and a pane can also open Cursor Agent or a plain shell. Lemongrass is licensed under the Mozilla Public License 2.0, see [LICENSE](LICENSE). Copyright belongs to The Lemongrass Authors, see [AUTHORS](AUTHORS).

![The Lemongrass app with a live workgroup and its thread, a web tab and the Vault](docs/images/overall-view-2.png)

## What it does

Each line links to its full description in [docs/features.md](docs/features.md).

- **[Agents use your databases and APIs without holding the credential.](docs/features.md#agents-use-your-databases-and-apis-without-holding-the-credential)** `lgrass db` and `lgrass rester` send queries, HTTP calls, browser sessions and Android app logins through a vault daemon, the only process that decrypts anything. The agent holds a short channel id that grants certain tables or methods and expires.
- **[Agents work as a team, across vendors.](docs/features.md#agents-work-as-a-team-across-vendors)** A leader declares a workgroup of Claude Code and Codex thinkers. The human approves what starts, each thinker posts its plan before it changes anything, and members talk to each other directly in a shared thread.
- **[Bypass mode with a guard that stays on.](docs/features.md#bypass-mode-with-a-guard-that-stays-on)** Claude Code tabs run without permission prompts, and a hook judges every call against a catalog of deny, ask and allow rules drawn at the project's edge. The rules are switchable per project behind the vault passphrase.
- **[Agents ask you instead of stalling.](docs/features.md#agents-ask-you-instead-of-stalling)** `lgrass ask` asks the human for a database channel, an HTTP channel or a command run in one dialog, and the answer goes back to the agent.
- **[A notification center.](docs/features.md#a-notification-center)** A bell in the header bar lists decisions waiting for the human, past answers, agent events and app events.
- **[Project rules arrive at the start of every session.](docs/features.md#project-rules-arrive-at-the-start-of-every-session)** Every Claude tab starts with an orientation and the project's laws in its system prompt, following the [bibliothek](https://github.com/faizalv/bibliothek) convention. Every change under `biblio/` is kept in a private history for 90 days.
- **[Run several Claude accounts.](docs/features.md#run-several-claude-accounts)** Each Claude tab and workgroup thinker picks the account it runs on.
- **[Share books and tasks with someone else's agent.](docs/features.md#share-books-and-tasks-with-someone-elses-agent)** The Node screen serves the books, tasks, laws and handovers you pick to other people's agents over MCP, read only, through a tunnel that closes on its own.
- **[Real terminals, controlled by structured events.](docs/features.md#real-terminals-controlled-by-structured-events)** Panes are `xterm.js` over `node-pty`, and control decisions come from each agent's hook events, never from parsing terminal text.

## Status

Linux is the primary platform. macOS (arm64 and x64) and Windows x64 are released, but the apps are not notarized or code signed. The guard is verified on Claude Code only, and Codex tabs run under Codex's own sandbox, which keeps them from `lgrass ask` and channels. What is built, in progress and planned, and every known limit, is under [Status and limits](docs/features.md#status-and-limits).

## Install

- **Fedora and other rpm distributions:** install the rpm from the [latest release](https://github.com/faizalv/lemongrass/releases).
- **macOS:** run `curl -fsSL https://raw.githubusercontent.com/faizalv/lemongrass/main/scripts/install-mac.sh | bash` in Terminal. It installs the dmg for your chip and opens the app with no warning. To install the dmg by hand, follow the steps under [macOS](docs/install.md#macos).
- **Windows 10 or 11 (x64):** download `lemongrass-<version>-setup.exe` from the [latest release](https://github.com/faizalv/lemongrass/releases) and run it. It is not code signed, so Windows asks once: click More info, then Run anyway.

The app updates itself from Settings, Updates. Verifying a download, release candidates and uninstalling are in [docs/install.md](docs/install.md).

## Build and develop

```
make build-linux   # the Go binaries and the Linux packages
make install       # builds and installs lgrass, lgrassd and lgrassconf
make dev           # builds the UI once and launches it
```

How the code is laid out, the `lgrass how` procedures every agent follows, the build targets for each platform and the dev loop are in [docs/development.md](docs/development.md).

## Contributing

Pull requests target `main` and need the `license-check` job to pass. Run `make license` to stamp the license header on new files. A first pull request from a fork waits for a maintainer's approval before its checks run.

Anyone whose change is merged is welcome to add their name and email to [AUTHORS](AUTHORS) in the same pull request.

The design notes behind Lemongrass are kept outside the repository. If you need background on a part of the code before changing it, email one of the people in [AUTHORS](AUTHORS).
