package session

import "math/rand"

// Loading-screen-style reminders, surfaced periodically alongside the
// regular nudge -- only when the current project has actually adopted
// bibliothek (biblio/ exists), so they're never noise for one that hasn't.
var bibliothekTips = []string{
	"bibliothek tip: a handover is never written for finished work -- that's a book chapter.",
	"bibliothek tip: handover is temporary, permanence is a book entry.",
	"bibliothek tip: finishing a task includes writing its book chapter, same as running its tests.",
	"bibliothek tip: memory only ever points at biblio/handover/<task>.md or biblio/books/toc.md -- never a hardcoded book path.",
	"bibliothek tip: scratchpad is disposable working notes, not where a decided design lives.",
	"bibliothek tip: a standing behavioral rule goes to memory and biblio/laws/ together, not one without the other.",
	"bibliothek tip: check biblio/books/toc.md before researching something from scratch -- it might already be covered.",
	"bibliothek tip: a handover's Status section is the truth, its Log is history only -- don't read the Log for current state.",
	"bibliothek tip: once a handover's work is done, archive it -- biblio/handover/ should only ever hold what's still in flight.",
	"bibliothek tip: tags are one word each -- a compound idea is multiple tags, not one hyphenated tag.",
}

func RandomTip() string {
	return bibliothekTips[rand.Intn(len(bibliothekTips))]
}
