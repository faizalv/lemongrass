# Triaging a stain

`lgrass berries stain <stain-id>` prints the stain, the rule it was judged by with that rule's source, and the pair that proves it: the control, the response the seed's own user got, and the replay, the response the other user got.

## Authorization stains

- **confirmed**: a user the map denies got the same content as the owner. That is an access-control hole unless the rule itself is wrong. Fix it with `lgrass how berries fix`.
- **open_proof_gap**: a user the map denies got a 2xx with other content. Decide whose content it is from the two previews. When it is the other user's own data, such as a route that answers with the caller's records whatever id it is given, the route is not the hole the rule assumed; propose a corrected rule to the human. When it holds any of the owner's data, treat it as confirmed.

When you believe a rule is wrong, never edit it so the stain goes away. Propose the change as `status: proposed` with its source and tell the human why. A rule changes only with the human's approval.

## Passive stains

A `passive.` stain is about the response itself: a missing header, a cookie without its flags, a stack trace in a body. The detail names what is missing. Fix it where the app sets its headers or handles errors, then confirm with `lgrass how berries fix`.

## Accepting

Accepting a stain as a known risk is the human's call, made in the Berries pane. Never accept one yourself, and never leave one unreported because you judge it minor: tell the human and let them decide.
