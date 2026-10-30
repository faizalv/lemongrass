Debugging is chasing a fault to its root cause.

1. {debug}
2. Keep `debug.md` current, rewritten in place: the symptom, what is ruled out and how, and the current best theory. Never retry what is already ruled out. Test a theory against evidence before acting on it, and when a known-working case exists, compare against it concretely instead of guessing one theory at a time.
3. Once found, write the root cause into `debug.md`, then propose the fix to the user and wait for the go before changing code.
4. A fix that restores intended behavior gets no book chapter; its root cause stays in the archived `debug.md`. Close with `lgrass how task close {slug}`.
