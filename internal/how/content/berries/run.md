# Running berries

Berries never writes a request of its own. It replays the calls you made with `lgrass rester` in this project, called seeds, as their own user and as every other user on the channel, and judges each answer against `.berries/map.yaml`. Only GET, HEAD and OPTIONS seeds are replayed; writes are recorded but skipped.

## Record seeds

Every successful `lgrass rester` call in a project with a map is recorded as a seed, so trying your work through the channel as you already do is the whole step. Call each route you changed as a user the map allows. For a route allowed to `owner`, call it as the user who owns the object, such as a customer reading their own invoice. `lgrass berries seeds` lists what is recorded.

## Run

`lgrass berries run <short-id>` needs an active api channel on the rester and environment the seeds were recorded on. Read its result by count first:

- **unmapped**: requests no route covers. Add the routes with `lgrass how berries map`.
- **control_failed**: a seed no longer succeeds as its own user, often because the object is gone. Record the call again.
- **over_restricted**: a user the map allows was refused. Either the rule or the code is wrong; tell the human which, with the route.
- **open_proof_gap** and **confirmed**: stains. Triage each with `lgrass how berries stain`.
- **ruled_out**: the app refused a user the map denies, and the result names how. Nothing to do.

A stain fixed in an earlier run that appears again is `regressed`, which means a later change removed the check.

## When to run

After you add or change a route or the code that guards it, and before closing a task that did. Run it again after a fix to see that nothing else regressed.
