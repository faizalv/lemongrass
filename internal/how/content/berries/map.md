# Building a berry's access map

A berry is the project's access map, `.berries/map.yaml`, committed with the code. Berries judges every replay against it, so the map is the spec: it says who may call each route, and nothing berries reports is better than the rules in it.

```yaml
roles: [customer, admin]
routes:
  - route: GET /invoices/{id}
    allow: [owner, admin]
    source: PRD invoices, "only the invoice's owner and admins see it"
    status: proposed
  - route: GET /me
    allow: [any]
    source: books/accounts, profile chapter
    status: proposed
```

- **Roles** are the tags the channel's users carry. `lgrass rester <short-id> users` lists them. Two pseudo-roles are built in: `owner` is the user a request was recorded as, and `any` is every user on the channel.
- **Routes** are a method and a path relative to the api's base URL. A `{name}` segment matches any one segment, and a literal segment beats a parameter, so `/invoices/export` and `/invoices/{id}` can both exist.

## The inventory: which routes exist

Take the routes from the project itself. When the framework can print its route table or the project has an OpenAPI spec, use that rather than reading handlers by hand. A route the project serves but the map leaves out is never tested. `lgrass berries map` lists the recorded requests no route covers.

## The rules: who may call each route

A rule states intent, so it comes from where intent is written: the task's PRD, the books, the business rules, or the human. Never derive a rule from what the code currently allows, because that writes a missing check into the map as allowed and berries then confirms the hole instead of finding it.

- Give every rule a `source` that names where its intent comes from, quoted when short.
- Write every rule you add or change as `status: proposed`. Only the human sets `approved`. List the proposed rules for them in chat with their sources.
- When the code allows something the stated intent forbids, tell the human. That disagreement is itself a finding, even before a run.
- When no source says who may call a route, ask the human instead of guessing.

## Users to test with

Each role needs at least one user on the channel to be tested at all, and two users to test one user of a role reading another's objects. `lgrass berries map <short-id>` names the roles that fall short. Ask for a channel with the users you need through `lgrass how ask request`, naming the roles.
