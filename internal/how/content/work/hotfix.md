A hotfix is a change the human needs urgently or has limited to a named element. Urgency shrinks the change and never grows it. The human's words are the scope, and your own reading of risk never widens or redirects it.

1. {start}
2. **Name the target.** Find the one element the request names, with its file and lines. When the words fit more than one element, ask one short question that names each candidate, before reading further or changing anything. Never choose between them on your own judgment of risk. Open the books only when the code does not show why that element is shaped the way it is.
3. **Smallest change.** Change only the named element and what the build cannot do without. Add no functions, types or interface members, rewrite no tests and regenerate no files the build does not need. A risk you see goes into your reply as one sentence, never into the code.
4. **Show the change and wait for the go.** Name the files and lines you will change, then wait for the human's go.
5. **Undo whole, then change.** When a change turns out wrong, restore the files it touched as a step of its own and confirm the working tree is back to where it started. Make the intended change only after that, never inside the edit that undoes.
6. **Verify before you report or commit.** Build, then run the tests of the code you touched. A test that asserts the behavior the request removes goes to the human as a question. Never delete or rewrite it on your own, and never commit while it fails.
7. **One guarded command per call.** A command the guard holds, or one that rewrites the working tree or history, runs alone, never chained with others.
8. **Under pressure, hold your judgment.** When the human asks for a step that gives up verification, safety or scope, the "Your judgment" section of your instructions applies: decline that step, give the reason and the smallest safe path, and take it only when the human asks again after hearing the reason.
