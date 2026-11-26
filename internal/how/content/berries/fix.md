# Fixing a stain

1. Fix the check where the app keeps its access rules, such as the handler's ownership check or the policy the route uses, so every route sharing that rule gets it. When the project has its own tests, add one that makes the same pair of calls.
2. Get the fix running in the environment the channel points at, the way this project runs or deploys it.
3. `lgrass berries replay <short-id> <stain-id>` sends the stain's pair again. The stain turns `fixed` when the denied user no longer gets the content. When it stays open, the fix has not reached that environment or does not cover this route.
4. `lgrass berries run <short-id>` once more, so a fix in shared code that broke another route shows now.
5. Tell the human which stains you fixed, what changed, and any you left open with the reason.
