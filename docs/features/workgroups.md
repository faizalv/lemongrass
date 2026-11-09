# Agents work as a team, across vendors

A leader agent declares a workgroup of one or more thinkers, each running Claude Code or Codex, with `lgrass workgroup create`, and can add thinkers to the live group with `lgrass workgroup add`.

- **The human approves what starts.** A proposal raises an OS notification and waits in an approval modal that shows every thinker's complete starting text before anything spawns. The human picks the account each Claude thinker runs on. Approval issues a one-time token, and that token is the only path that opens the members' tabs.
- **Plan first.** A thinker posts its plan to the group thread and waits for the leader's go before it changes anything. A gate denies every tool except `lgrass` commands until the thinker has run the `lgrass` commands and loaded the skills its config lists, starting with `lgrass how thinker start` for a Claude thinker.
- **Real conversation.** Members share a thread with mentions, unread-only reads, and notices delivered through each agent's own hooks. Any member can address any other directly.
- **Persistent.** Membership survives a tab closing and an app restart. The sidebar lists each live group, and a workgroup tab shows its members, whether their tabs are online and the thread. A closed member reopens with its saved session and its own account. A disbanded group's thread stays readable.
