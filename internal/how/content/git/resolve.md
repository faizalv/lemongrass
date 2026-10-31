# Resolving a conflict the human handed you

The human stopped in a merge, rebase or stash apply with conflicts and handed them to your tab. You resolve them in the working files and explain your choices. The human reviews every choice in the resolver and keeps the git operation: you never stage, commit, continue, skip or abort.

1. Run `lgrass git conflict show`. It prints the operation, the refs, every conflicted file with its conflict blocks, the blocks the human already resolved, the commits each side made to those files with their messages, and the conflict thread's id.
2. Read both sides' intent from those commits before you choose. Look past the marked blocks too: a clean region can still break, for example when one side renamed something the other side still uses.
3. Resolve in the working files and save them unstaged. Remove the markers of every block you resolve. Keep the blocks the human already resolved as they are. Never run `git add`, `git rm`, `git checkout --ours` or `--theirs`, or anything that continues, skips or aborts the operation.
4. Note every block you resolved and every line you changed outside the blocks: `lgrass git conflict note <path> <N|N-M> "<why>"`, with the lines as they stand in the working file now. Say what you kept and why, in a sentence or two the human can check against the code.
5. If you cannot decide a block on your own, leave its markers in place and ask in the conflict thread with `lgrass thread post <thread-id> "<question>"`. This thread is the one place you talk to the human about the conflict. A message there mentions no tab.
6. Run `lgrass git conflict submit` once your resolution is ready, even with blocks left for the human. The resolver opens on your result in your pane.
7. End your turn. The human accepts, overrides or comments in the resolver, and a reply arrives as a nudge for the conflict thread: read it with `lgrass thread read <thread-id>`, change what it asks, note the changes, and submit again. Submitting again replaces your earlier submission.
