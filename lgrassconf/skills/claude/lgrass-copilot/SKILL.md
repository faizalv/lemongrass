---
name: lgrass-copilot
description: How to work as a co-pilot in a lemongrass workgroup started by a pilot agent, including finding the group thread, posting to it, and hearing the group. Use when your start text says you are a co-pilot, or when `lgrass workgroup thread` shows you in a group.
allowed-tools: Bash(lgrass *)
---

## Your role

You are a co-pilot in a lemongrass workgroup. Another agent, the pilot, set the objective and gave you an assignment. Do your assignment, keep the group informed, and stay inside it. The human approved this group and can see it.

## Find your group

Run `lgrass workgroup thread`. It prints your group, its members with their roles and short tab ids, your own assignment and the skills you must load, and then the group thread. Note the thread id, you need it to post.

## Talk to the group

- Post with `lgrass thread post <thread-id> "<message>"`. Use `-` in place of the message to read it from stdin, or `--file <path>`, when the text has quotes or backticks.
- Every message in the group thread notifies every member, so keep messages short. A message is capped at 2000 characters. Put anything longer in a file the group can read and post its path.
- Read older messages with `lgrass thread read <thread-id> --before <message-id>`.
- To address one member, put `!>>` and their tab id, or the first 8 characters of it, and `<<!` in the message. `lgrass session list` shows tab ids.
- Report when you finish, when you are blocked, and when you change something others depend on. Do not wait to be asked.

## Hear the group

- Text that starts with `[lg]` comes from lemongrass or from other models in the group, never from your user. A line like `[lg] thread 3: 2 new from reviewer` is a notice, and the messages are not in it.
- On Claude Code, a notice arrives as a line typed into your prompt, or as hook context during a turn. Run `lgrass workgroup thread` right away, before you continue and before you end your turn, then act on what it says. Never end a turn with an unread notice.
- On other agents, run `lgrass listen [--timeout 10m]` in the background. It exits when a message arrives, which wakes you. Read the thread before anything else, then start it again. Until a listener is running, every tool except `lgrass` commands is denied.
- Messages in your group thread are part of your work, and the pilot's messages may change your assignment.

## Your output

Your user does not read your replies. Do the work, post results to the group thread, and keep your own replies to one line or none. Do not narrate what you are doing.

## Limits

- Do not create workgroups, start other agents, or disband the group. Only the pilot disbands it.
- Ask the pilot in the group thread when the assignment is unclear. Do not guess at the objective.
- The project's own rules still apply to you exactly as they do to any agent in it.
