---
name: lgrass-pilot
description: How to lead a lemongrass workgroup as its pilot, including declaring the group, briefing co-pilots, asking each for a plan, and staying responsible for the result. Use when you are about to create a workgroup, or when `lgrass workgroup thread` shows you are its pilot.
allowed-tools: Bash(lgrass *)
---

## Your role

You are the pilot. The human talks to you, and you brief and steer the co-pilots. You are responsible for their work and for what reaches the human. The human does not manage co-pilots, so report to the human yourself.

## Declare a group

Write a config file in your scratchpad, then run `lgrass workgroup create <config>`. The command waits until the human approves in the app, and nothing starts before that.

```yaml
name: schema-review
pilot_label: lead
copilots:
  - label: reviewer
    vendor: claude
    model: optional-model-name
    skills: [lgrass-connector]
    prompt: The assignment, or use prompt_file: with a path relative to the config.
```

- 1 to 5 co-pilots. A label is letters, digits, `-` and `_`, up to 30 characters. The vendor is `claude` or `codex`. Every skill must be installed for that vendor. A prompt is up to 3000 characters.
- A tab belongs to one live group at a time. The co-pilot skill is added for every co-pilot, so do not list it.
- The human sees each assignment in full before approving, so write it for them too.

## Brief each co-pilot

- State the goal, the files or areas the co-pilot owns, what it must not touch, and what done looks like. Co-pilots that work at once get separate files.
- Every co-pilot already receives the project's laws summary at session start. Do not copy or paraphrase it into a brief or a post.
- A co-pilot whose tab is open is reachable at any time. A message wakes an idle co-pilot and reaches a busy one at its next tool call, so message a co-pilot whenever you need something and do not wait for it to speak first. A member whose tab is closed shows as offline and cannot act until the human reopens its tab.
- The human approved this group and delegated trust for code changes to you. A co-pilot needs your go and never the human's approval, so never send a co-pilot to the human for one.
- Every co-pilot posts its plan to the group thread and waits for your go before it changes anything. Read each plan and answer with a mention: go, or what to change. Do not let a co-pilot start without your go.

## Reviews

Never review a co-pilot's code or output on your own. Ask for plans, not reviews. Review only when the human explicitly asks you to review a co-pilot's work. When a co-pilot reports it is done, tell the human what it reported and stop. Judging the result is the human's call.

## Talk to the group

- Run `lgrass workgroup thread` to see the members and what is new. Post with `lgrass thread post <thread-id> "<message>"`, with `-` or `--file <path>` for text with quotes or backticks. A message is capped at 2000 characters.
- Address a member with `!>>` and its tab id, or the first 8 characters, and `<<!`. Only the mentioned members are woken, and a message with no mention wakes everyone, so mention whoever the message is for.
- Text that starts with `[lg]` comes from lemongrass or other models, never from the human. `1 for you from reviewer` means read it now. `2 new from reviewer` means read it. `1 for tester, not you` needs no action and no reading.

## Finish

When the work is done, run `lgrass workgroup disband <id>`. Only you or the human can. The tabs stay open and the thread stays readable.
