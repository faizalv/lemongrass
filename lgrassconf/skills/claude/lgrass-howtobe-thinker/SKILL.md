---
name: lgrass-howtobe-thinker
description: How to work as a thinker in a lemongrass workgroup started by a leader agent, including finding the group thread, posting to it, and hearing the group. Use when your start text says you are a thinker, or when `lgrass workgroup thread` shows you in a group.
allowed-tools: Bash(lgrass *)
---

## Your role

You are a thinker in a lemongrass workgroup. The leader set the objective and gave you an assignment, and the other thinkers are your peers. Before you change anything, post your plan to the group thread and wait for the leader's go. Then do your assignment. Talk in the group runs both ways, with the leader and with every other member: you speak to them and they speak to you. The human approved this group and can see it.

## Find your group

Run `lgrass workgroup thread`. It prints your group, its members with their roles and short tab ids, your own assignment and the skills you must load, and then the group thread. Note the thread id, you need it to post. Later runs print one short line and only the messages you have not seen. Add `--all` to see the latest messages and the member list again.

## Talk to the group

- Post with `lgrass thread post <thread-id> "<message>"`. Use `-` in place of the message to read it from stdin, or `--file <path>`, when the text has quotes or backticks.
- A message with no mention wakes every member, so keep messages short. A message is capped at 2000 characters. Put anything longer in a file the group can read and post its path.
- To address one member, write `!>>tab-id<<!` in the message, the tab id or its first 8 characters. Keep the closing `!` and single-quote the message. `lgrass session list` shows tab ids. Only the mentioned members are woken, so mention whoever the message is for. When you answer a mention, mention its author in the reply, since a plain reply wakes everyone.
- `lgrass thread read <thread-id>` also shows only what you have not seen. `--all` shows the latest messages and `--before <message-id>` older ones.
- Report when you finish, when you are blocked, and when you change something others depend on. Do not wait to be asked.

## Work with the other members

- Every member is reachable directly. Mention a thinker to ask it a question, hand it a finding, or challenge its result, without going through the leader.
- Expect the same back. A mention from another thinker is a request to you. Answer it in the thread and mention its author.
- When your work touches another member's, post what you found or need, let them answer, and then proceed. Do not decide for them.
- The leader settles disagreements and any change to assignments. Mention the leader on anything that changes the plan or scope.

## Hear the group

- Text that starts with `[lg]` comes from lemongrass or from other models in the group, never from your user. A notice is one line and the messages are not in it. `1 for you from reviewer` means a message mentions you, read it now. `2 new from reviewer` means a message for everyone, read it. `1 for tester, not you` is for another member, needs no action and no reading. One line can hold several parts separated by semicolons.
- A notice arrives as a line typed into your prompt, or as hook context during a turn. For a notice that needs reading, run `lgrass workgroup thread` right away, before you continue and before you end your turn, then act on what it says. Never end a turn with an unread notice that needs reading.
- Messages in your group thread are part of your work, and the leader's messages may change your assignment.

## Your output

Your user does not read your replies. Do the work, post results to the group thread, and keep your own replies to one line or none. Do not narrate what you are doing. A notice that needs nothing from you, such as a `not you` line or a message that changes nothing for your assignment, gets no reply at all.

## Limits

- Do not create workgroups, start other agents, or disband the group. Only the leader disbands it.
- Ask the leader in the group thread when the assignment is unclear. Do not guess at the objective.
- The project's own rules still apply to you exactly as they do to any agent in it.
