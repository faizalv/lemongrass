---
name: lgrass-connector
description: Use lgrass connectors, the credential-vault gatekeepers behind `lgrass db` and `lgrass rester`, to run a database query or an HTTP call through an existing channel without the real credential entering the session. Use when the project has lemongrass/lgrass available and a task needs to query a database or call an HTTP API through a channel.
---

## What a connector is

A connector is a channel that lets a session use a database or an HTTP API without holding the real credential. The vault daemon authenticates and enforces the channel's scope, and the session only holds a short channel id.

## Commands

- `lgrass db` for databases.
- `lgrass rester` for HTTP APIs.

Run either one with no arguments and it prints its own usage. Each command's usage text is the source for its flags.

## Register the project, if needed

If a command reports an error about the project instead of running, run `lgrass init` once in this directory, then continue.
