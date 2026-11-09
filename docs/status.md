# Status and limits

Built:

- The vault, `lgrass db` reads and writes, `lgrass rester` for APIs, web apps and Android app login, and the Vault tab.
- SSH tunnels for database connections, checked live against a real bastion.
- Workgroups with Claude Code and Codex members, the thread layer, the approval modal and the workgroup pane. Codex tabs receive nudges and thread notices through the hooks `lgrassconf` registers for Codex. Its limits are under Limits.
- Several Claude accounts, picked per tab and per workgroup thinker.
- The guard for Claude Code: the data catalog with deny, ask and allow defaults, interpreter and script-file checks, unit tested and checked live in a tab. The sealed-policy round trip through the Safety pane has not been run live.
- `lgrass ask` with the ask dialog, the Decisions section and the grant view for database and HTTP channels, checked live for command, database and HTTP asks.
- The notification center behind the header bell.
- The bibliothek browser and editor, and the Git panel.
- The biblio history: the watcher in the agent daemon, 90-day pruning, and `lgrass biblio log` and `show`.

In progress:

- The node: a read-only MCP endpoint over an ngrok tunnel, built and unit tested. Its transport and the Node screen have been checked by hand, and Claude Code has read listings, text and images through a live tunnel. Codex as a client, a live revoke and a live expiry have not been run yet. Search and an `lgrass node` client for Lemongrass users are planned.
- A wire-protocol proxy so a whole app can point its own database driver at a local port. Port allocation and the MySQL handshake listener are done. Query execution and the Postgres listener are not.

Planned:

- An advisory reviewer on a command ask, a second opinion from a small model run outside Lemongrass.
- An audit of `lgrass rester` calls and of `lgrass db` reads.
- An investigator role for cheap, read-only workgroup members.

Limits:

- Linux is the primary platform. Releases carry macOS dmgs for arm64 and x64, but the app is not notarized, so a browser-downloaded dmg needs the steps under [macOS](install.md#macos). An ad hoc signed app is a new app to macOS after every update, so folder access it was granted is asked again. Windows x64 ships as an unsigned `setup.exe`, so SmartScreen warns on the first run of each download, see [Windows](install.md#windows). Windows on arm64 runs it under emulation.
- An HTTP channel's scope is declared and checked against the request. The database scope is checked against what the parsed statement touches, which is a stronger guarantee.
- Codex tabs start with no bypass flag and run under Codex's own sandbox and approvals. A write by `lgrass` to its database, such as a thread post, asks for sandbox approval, so a Codex thinker cannot post unattended yet. Codex tabs also get no orientation in their system prompt, only the laws summary from the `SessionStart` hook, and Codex asks for review of the hook definitions through `/hooks`.
- The guard does not see what a build tool fetches as a side effect (`go build`, `cargo build`, npm lifecycle scripts). Builds are left unlimited on purpose.
- On MySQL, a statement that calls a function is refused when the database user cannot read `performance_schema.user_defined_functions`, because the function check fails closed. Function-free reads still work.
- The guard is verified on Claude Code only. A Codex tab cannot reach `lgrass ask` or a channel, since Codex's sandbox refuses the agent socket. Its hook also runs on Codex events, but no rule has been checked against what Codex sends. A Codex tab gets the default and global policy but not the project's, and the deletion exemption for the launch project is off for it. Cursor tabs have no hooks, so no guard.
- A web channel's scope is stored and validated but not enforced in the browser, and a browser shared by a workgroup is not built.
- The guard reads interpreter code as a tripwire against accident, not against deliberate evasion. Inline code (`python -c`, `node -e`, `php -r`, `perl -e`, `ruby -e`, `deno eval`), code piped or heredoc'd into an interpreter, and script files up to 256 KiB run by an interpreter or by shebang are checked for secret paths, destructive calls on paths that are not literals inside the project, and literal subprocess commands. A call built at runtime, `eval` inside the language, encoded payloads, imported modules and subprocess commands built from variables pass. A destructive shell command whose target comes from a variable is denied.
- The node's `~/.lemongrass/node.json` holds token hashes and the ngrok authtoken at mode 0600. The guard keeps agents' tools away from it, with the interpreter caveat above. A token is shown once and only its hash is stored.
