---
name: lemongrass
description: Use lgrass db and lgrass rester, this project's credential-vault gatekeepers, to run a database query or an HTTP call through an existing channel without ever touching the real credential. Use when this project has lemongrass/lgrass available and a task needs to query a database or call an HTTP API through a channel. Thread/session coordination is intentionally left out of this skill until the thread system rework lands.
---

## Register this project, if needed

If the commands below report an error about the project instead of running, this directory isn't a registered lemongrass project yet. Run `lgrass init` once, then continue.

## Querying a database: `lgrass db`

    lgrass db <short-id> --tables <t1,t2|*> --sql "<statement>"

Runs a read-only statement (SELECT/SHOW/DESCRIBE/EXPLAIN) against the database a channel grants
access to, through the running agent and vault daemons -- no credential ever touches this
session. `--tables` is a declared statement of intent, checked against what the statement
actually references; it isn't the security boundary, the channel's own scope is. Use `*` for a
statement with no specific table (`SHOW TABLES` and the like).

## Calling an HTTP API: `lgrass rester`

    lgrass rester <short-id> --user <name> --method <METHOD> --path <path> [--body '<json>']

Proxies one HTTP call through the channel's domain, authenticated as `user`; the vault handles
login and token injection, so no bearer token ever touches this session or its shell history.
Response is printed as JSON: `{"status": N, "headers": {...}, "body": ...}`.

## Not covered here right now

Thread/session coordination (`lgrass session`, `lgrass thread`) is deliberately left out of this
skill until the thread system rework finishes.
