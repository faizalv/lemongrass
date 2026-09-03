package knowledge

import (
	"os"
	"testing"
)

const testProjectID = "testproj"

// setupLibrary isolates $HOME to a fresh temp dir (never the real
// ~/.lemongrass) and writes one book with two chapters plus one
// standalone entry, mirroring the fixture used to smoke-test this by
// hand.
func setupLibrary(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	if _, err := CreateBook(testProjectID, BookOptions{
		Title:       "Auth Redesign",
		Tags:        []string{"security", "auth"},
		Description: "How auth got redesigned",
	}); err != nil {
		t.Fatalf("CreateBook: %v", err)
	}

	if _, err := Write(testProjectID, WriteOptions{
		Title: "Standalone Note",
		Tags:  []string{"misc"},
		Body:  "just a plain entry, no book",
	}); err != nil {
		t.Fatalf("Write standalone: %v", err)
	}

	if _, err := Write(testProjectID, WriteOptions{
		Title:   "Chapter one",
		BookID:  "auth-redesign",
		Chapter: 1,
		Tags:    []string{"legacy"},
		Body:    "the old system used sessions",
	}); err != nil {
		t.Fatalf("Write chapter one: %v", err)
	}

	if _, err := Write(testProjectID, WriteOptions{
		Title:   "Chapter two",
		BookID:  "auth-redesign",
		Chapter: 2,
		Tags:    []string{"tokens"},
		Body:    "the new system uses tokens",
	}); err != nil {
		t.Fatalf("Write chapter two: %v", err)
	}
}

func TestCreateBookAndWriteChapters(t *testing.T) {
	setupLibrary(t)

	book, err := ReadBook(testProjectID, "auth-redesign")
	if err != nil {
		t.Fatalf("ReadBook: %v", err)
	}
	if book.Title != "Auth Redesign" {
		t.Errorf("book title = %q, want %q", book.Title, "Auth Redesign")
	}

	entry, err := Read(testProjectID, "chapter-one")
	if err != nil {
		t.Fatalf("Read chapter-one: %v", err)
	}
	if entry.BookID != "auth-redesign" || entry.Chapter != 1 {
		t.Errorf("chapter-one BookID/Chapter = %q/%d, want auth-redesign/1", entry.BookID, entry.Chapter)
	}

	standalone, err := Read(testProjectID, "standalone-note")
	if err != nil {
		t.Fatalf("Read standalone-note: %v", err)
	}
	if standalone.BookID != "" {
		t.Errorf("standalone-note BookID = %q, want empty", standalone.BookID)
	}
}

func TestWriteRejectsMissingBook(t *testing.T) {
	setupLibrary(t)

	_, err := Write(testProjectID, WriteOptions{
		Title:   "Ghost",
		BookID:  "no-such-book",
		Chapter: 1,
		Body:    "body",
	})
	if err == nil {
		t.Fatal("Write against a nonexistent book: expected an error, got nil")
	}
}

func TestChaptersInOrder(t *testing.T) {
	setupLibrary(t)

	store, err := Open(DBPath(), testProjectID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	chapters, err := store.Chapters("auth-redesign")
	if err != nil {
		t.Fatalf("Chapters: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("len(chapters) = %d, want 2", len(chapters))
	}
	if chapters[0].ID != "chapter-one" || chapters[1].ID != "chapter-two" {
		t.Errorf("chapter order = [%s, %s], want [chapter-one, chapter-two]", chapters[0].ID, chapters[1].ID)
	}
}

// findBookResult returns the collapsed book row for id, or nil if search
// results contain no such row.
func findBookResult(results []SearchResult, id string) *SearchResult {
	for i := range results {
		if results[i].IsBook && results[i].ID == id {
			return &results[i]
		}
	}
	return nil
}

func TestSearchCollapsesBookToOneLine(t *testing.T) {
	setupLibrary(t)

	store, err := Open(DBPath(), testProjectID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	t.Run("empty query lists everything, book collapsed", func(t *testing.T) {
		results, err := store.Search("")
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("len(results) = %d, want 2 (one standalone entry, one collapsed book)", len(results))
		}
		book := findBookResult(results, "auth-redesign")
		if book == nil {
			t.Fatal("no collapsed book row for auth-redesign")
		}
		if book.ChapterCount != 2 {
			t.Errorf("book.ChapterCount = %d, want 2", book.ChapterCount)
		}
	})

	t.Run("chapter text match still collapses to the book", func(t *testing.T) {
		results, err := store.Search("tokens")
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 1 || !results[0].IsBook || results[0].ID != "auth-redesign" {
			t.Fatalf("Search(tokens) = %+v, want a single collapsed auth-redesign row", results)
		}
	})

	t.Run("book found via its own description with no chapter match", func(t *testing.T) {
		results, err := store.Search("redesigned")
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 1 || !results[0].IsBook || results[0].ID != "auth-redesign" {
			t.Fatalf("Search(redesigned) = %+v, want a single collapsed auth-redesign row", results)
		}
	})

	t.Run("query matching nothing returns nothing", func(t *testing.T) {
		results, err := store.Search("nonexistentwordxyz")
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("Search(nonexistentwordxyz) = %+v, want no results", results)
		}
	})
}

func TestEditReplacesLineRange(t *testing.T) {
	setupLibrary(t)

	original, err := Read(testProjectID, "chapter-one")
	if err != nil {
		t.Fatalf("Read before edit: %v", err)
	}
	if original.Body != "the old system used sessions" {
		t.Fatalf("unexpected fixture body: %q", original.Body)
	}

	edited, err := Edit(testProjectID, "chapter-one", 1, 1, "the old system used cookies\nand also csrf tokens")
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	wantBody := "the old system used cookies\nand also csrf tokens"
	if edited.Body != wantBody {
		t.Errorf("edited.Body = %q, want %q", edited.Body, wantBody)
	}
	if edited.CreatedAt != original.CreatedAt {
		t.Errorf("CreatedAt changed: got %q, want %q", edited.CreatedAt, original.CreatedAt)
	}
	if edited.BookID != "auth-redesign" || edited.Chapter != 1 {
		t.Errorf("book linkage lost after edit: BookID=%q Chapter=%d", edited.BookID, edited.Chapter)
	}

	reread, err := Read(testProjectID, "chapter-one")
	if err != nil {
		t.Fatalf("Read after edit: %v", err)
	}
	if reread.Body != wantBody {
		t.Errorf("body on disk after edit = %q, want %q", reread.Body, wantBody)
	}
}

func TestEditOnMultiLineBody(t *testing.T) {
	setupLibrary(t)

	if _, err := Write(testProjectID, WriteOptions{
		Title: "Multi Line",
		Body:  "line one\nline two\nline three\nline four",
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Replace the middle two lines with three new ones -- the body should
	// grow by one line and the untouched head/tail lines survive.
	edited, err := Edit(testProjectID, "multi-line", 2, 3, "replacement a\nreplacement b\nreplacement c")
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	want := "line one\nreplacement a\nreplacement b\nreplacement c\nline four"
	if edited.Body != want {
		t.Errorf("edited.Body = %q, want %q", edited.Body, want)
	}
}

func TestEditRejectsOutOfRangeLines(t *testing.T) {
	setupLibrary(t)

	if _, err := Edit(testProjectID, "chapter-one", 1, 5, "replacement"); err == nil {
		t.Fatal("Edit past the end of a 1-line body: expected an error, got nil")
	}
	if _, err := Edit(testProjectID, "chapter-one", 0, 1, "replacement"); err == nil {
		t.Fatal("Edit with a 0 start line: expected an error, got nil")
	}
}

func TestEditRejectsEmptyResult(t *testing.T) {
	setupLibrary(t)

	if _, err := Edit(testProjectID, "chapter-one", 1, 1, ""); err == nil {
		t.Fatal("Edit that empties the whole body: expected an error, got nil")
	}
}

// TestOpenOnFreshHomeWithNoPriorWrites guards against a real bug: Open
// used to assume config.Dir() already existed, which held only because
// some prior Write/CreateBook call had run MkdirAll first. A project
// whose very first lgrass call is search/toc/reindex (nothing written
// yet) hit "unable to open database file".
func TestOpenOnFreshHomeWithNoPriorWrites(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store, err := Open(DBPath(), testProjectID)
	if err != nil {
		t.Fatalf("Open on a completely fresh $HOME: %v", err)
	}
	defer store.Close()

	if _, err := store.Search(""); err != nil {
		t.Errorf("Search on a fresh, empty index: %v", err)
	}
}

func TestReindexRebuildsBooksAndEntries(t *testing.T) {
	setupLibrary(t)

	if err := os.Remove(DBPath()); err != nil {
		t.Fatalf("removing index db: %v", err)
	}

	entries, books, err := Reindex(testProjectID)
	if err != nil {
		t.Fatalf("Reindex: %v", err)
	}
	if entries != 3 {
		t.Errorf("entries = %d, want 3", entries)
	}
	if books != 1 {
		t.Errorf("books = %d, want 1", books)
	}

	store, err := Open(DBPath(), testProjectID)
	if err != nil {
		t.Fatalf("Open after reindex: %v", err)
	}
	defer store.Close()

	results, err := store.Search("")
	if err != nil {
		t.Fatalf("Search after reindex: %v", err)
	}
	if book := findBookResult(results, "auth-redesign"); book == nil || book.ChapterCount != 2 {
		t.Fatalf("after reindex, auth-redesign book row = %+v, want IsBook with ChapterCount 2", book)
	}
}
