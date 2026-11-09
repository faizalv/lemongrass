---
layout: home

hero:
  name: Lemongrass
  text: Coding agents in real terminals
  tagline: Credentials they can use but never see, teammates they can message, a safety guard that holds with permission prompts off, and project rules that arrive at the start of every session.
  image:
    src: /logo.svg
    alt: Lemongrass
  actions:
    - theme: brand
      text: Install
      link: /install
    - theme: alt
      text: See what it does
      link: /features/connectors
    - theme: alt
      text: GitHub
      link: https://github.com/faizalv/lemongrass

features:
  - title: Databases and APIs without the credential
    details: <code>lgrass db</code> and <code>lgrass rester</code> send queries, HTTP calls, browser sessions and Android app logins through a vault daemon, the only process that decrypts anything. The agent holds a short channel id that grants certain tables or methods and expires.
    link: /features/connectors
  - title: Agents work as a team, across vendors
    details: A leader declares a workgroup of Claude Code and Codex thinkers. The human approves what starts, each thinker posts its plan before it changes anything, and members talk to each other directly in a shared thread.
    link: /features/workgroups
  - title: Bypass mode with a guard that stays on
    details: Claude Code tabs run without permission prompts, and a hook judges every call against a catalog of deny, ask and allow rules drawn at the project's edge. The rules are switchable per project behind the vault passphrase.
    link: /features/guard
  - title: Agents ask you instead of stalling
    details: <code>lgrass ask</code> asks the human for a database channel, an HTTP channel or a command run in one dialog, and the answer goes back to the agent.
    link: /features/ask
  - title: A notification center
    details: A bell in the header bar lists decisions waiting for the human, past answers, agent events and app events.
    link: /features/notifications
  - title: Project rules at the start of every session
    details: Every Claude tab starts with an orientation and the project's laws in its system prompt, following the bibliothek convention. Every change under <code>biblio/</code> is kept in a private history for 90 days.
    link: /features/project-rules
  - title: Several Claude accounts
    details: Each Claude tab and workgroup thinker picks the account it runs on.
    link: /features/accounts
  - title: Share books and tasks with someone else's agent
    details: The Node screen serves the books, tasks, laws and handovers you pick to other people's agents over MCP, read only, through a tunnel that closes on its own.
    link: /features/node
  - title: Real terminals, controlled by structured events
    details: Panes are <code>xterm.js</code> over <code>node-pty</code>, and control decisions come from each agent's hook events, never from parsing terminal text.
    link: /features/terminals
---

![The Lemongrass app with a live workgroup and its thread, the biblio editor, the Git history and the Vault](./images/overall-view-3-web.webp){.home-shot}

## Install

On Fedora and other rpm distributions, install the rpm from the [latest release](https://github.com/faizalv/lemongrass/releases).

On macOS, run this in Terminal. It installs the dmg for your chip and opens the app with no warning.

```sh
curl -fsSL https://raw.githubusercontent.com/faizalv/lemongrass/main/scripts/install-mac.sh | bash
```

On Windows 10 or 11 (x64), download `lemongrass-<version>-setup.exe` from the [latest release](https://github.com/faizalv/lemongrass/releases) and run it. It is not code signed, so Windows asks once: click More info, then Run anyway.

The app updates itself from Settings, Updates. The full steps, verifying a download and uninstalling are on the [install page](/install).

Lemongrass is licensed under the Mozilla Public License 2.0.
