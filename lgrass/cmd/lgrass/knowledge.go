package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/faizalv/lemongrass/knowledge"
	"github.com/faizalv/lemongrass/project"
)

func cmdKnowledge(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: lgrass knowledge <write|edit|search|read|reindex|book|toc> ...")
		os.Exit(1)
	}
	switch args[0] {
	case "write":
		cmdKnowledgeWrite(args[1:])
	case "edit":
		cmdKnowledgeEdit(args[1:])
	case "search":
		cmdKnowledgeSearch(args[1:])
	case "read":
		cmdKnowledgeRead(args[1:])
	case "reindex":
		cmdKnowledgeReindex()
	case "book":
		cmdKnowledgeBook(args[1:])
	case "toc":
		cmdKnowledgeToc()
	default:
		fmt.Fprintf(os.Stderr, "unknown knowledge command: %s\n", args[0])
		os.Exit(1)
	}
}

func currentProject() project.Project {
	p, err := project.ResolveCwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return p
}

func cmdKnowledgeWrite(args []string) {
	var tags []string
	var bookID, title string
	var chapter int

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--tags":
			i++
			if i < len(args) {
				tags = splitCSV(args[i])
			}
		case "--book":
			i++
			if i < len(args) {
				bookID = args[i]
			}
		case "--chapter":
			i++
			if i < len(args) {
				chapter, _ = strconv.Atoi(args[i])
			}
		case "--title":
			i++
			if i < len(args) {
				title = args[i]
			}
		}
	}

	body, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading body from stdin: %v\n", err)
		os.Exit(1)
	}

	p := currentProject()
	entry, err := knowledge.Write(p.ID, knowledge.WriteOptions{
		Title:   title,
		Tags:    tags,
		BookID:  bookID,
		Chapter: chapter,
		Body:    string(body),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%s)\n", entry.ID, entry.Title)
}

func cmdKnowledgeEdit(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: lgrass knowledge edit <id> --lines A-B < replacement.md")
		os.Exit(1)
	}
	id := args[0]

	var lineRange string
	for i := 1; i < len(args); i++ {
		if args[i] == "--lines" {
			i++
			if i < len(args) {
				lineRange = args[i]
			}
		}
	}
	if lineRange == "" {
		fmt.Fprintln(os.Stderr, "usage: lgrass knowledge edit <id> --lines A-B < replacement.md")
		os.Exit(1)
	}
	start, end, err := parseLineRange(lineRange)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	replacement, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading replacement from stdin: %v\n", err)
		os.Exit(1)
	}

	p := currentProject()
	entry, err := knowledge.Edit(p.ID, id, start, end, string(replacement))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("edited %s (%s), lines %d-%d\n", entry.ID, entry.Title, start, end)
}

// A bare "A" is shorthand for "A-A"; bounds are inclusive and 1-indexed.
func parseLineRange(s string) (start, end int, err error) {
	parts := strings.SplitN(s, "-", 2)
	start, err = strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid line range %q", s)
	}
	end = start
	if len(parts) == 2 {
		end, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return 0, 0, fmt.Errorf("invalid line range %q", s)
		}
	}
	return start, end, nil
}

func cmdKnowledgeSearch(args []string) {
	query := strings.Join(args, " ")
	p := currentProject()

	store, err := knowledge.Open(knowledge.DBPath(), p.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	results, err := store.Search(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(results) == 0 {
		fmt.Println("no matching entries")
		return
	}
	for _, r := range results {
		if r.IsBook {
			line := fmt.Sprintf("%s\t%s\t[book, %d chapters]", r.ID, r.Title, r.ChapterCount)
			if len(r.Tags) > 0 {
				line += "\t[" + strings.Join(r.Tags, ", ") + "]"
			}
			fmt.Println(line)
			continue
		}
		line := fmt.Sprintf("%s\t%s", r.ID, r.Title)
		if len(r.Tags) > 0 {
			line += "\t[" + strings.Join(r.Tags, ", ") + "]"
		}
		if r.BookID != "" {
			line += fmt.Sprintf("\t(book %s, chapter %d)", r.BookID, r.Chapter)
		}
		fmt.Println(line)
	}
}

func cmdKnowledgeRead(args []string) {
	p := currentProject()

	if len(args) >= 2 && args[0] == "--book" {
		readBook(p, args[1])
		return
	}
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: lgrass knowledge read <id> | --book <id>")
		os.Exit(1)
	}
	entry, err := knowledge.Read(p.ID, args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	printEntry(entry)
}

func readBook(p project.Project, bookID string) {
	store, err := knowledge.Open(knowledge.DBPath(), p.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	chapters, err := store.Chapters(bookID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(chapters) == 0 {
		fmt.Fprintf(os.Stderr, "no entries in book %s\n", bookID)
		os.Exit(1)
	}
	for i, r := range chapters {
		entry, err := knowledge.Read(p.ID, r.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", r.ID, err)
			os.Exit(1)
		}
		if i > 0 {
			fmt.Println()
		}
		printEntry(entry)
	}
}

func cmdKnowledgeReindex() {
	p := currentProject()
	entries, books, err := knowledge.Reindex(p.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("reindexed %d entries, %d books\n", entries, books)
}

func cmdKnowledgeToc() {
	p := currentProject()

	store, err := knowledge.Open(knowledge.DBPath(), p.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	results, err := store.Search("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(knowledge.FormatTOC(results))
}

func cmdKnowledgeBook(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: lgrass knowledge book create --title \"...\" [--tags a,b] [--description \"...\"]")
		os.Exit(1)
	}
	switch args[0] {
	case "create":
		cmdKnowledgeBookCreate(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown knowledge book command: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdKnowledgeBookCreate(args []string) {
	var tags []string
	var title, description string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--title":
			i++
			if i < len(args) {
				title = args[i]
			}
		case "--tags":
			i++
			if i < len(args) {
				tags = splitCSV(args[i])
			}
		case "--description":
			i++
			if i < len(args) {
				description = args[i]
			}
		}
	}

	p := currentProject()
	book, err := knowledge.CreateBook(p.ID, knowledge.BookOptions{
		Title:       title,
		Tags:        tags,
		Description: description,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("created book %s (%s)\n", book.ID, book.Title)
}

func printEntry(e knowledge.Entry) {
	fmt.Printf("# %s\n", e.Title)
	if len(e.Tags) > 0 {
		fmt.Printf("tags: %s\n", strings.Join(e.Tags, ", "))
	}
	if e.BookID != "" {
		fmt.Printf("book: %s (chapter %d)\n", e.BookID, e.Chapter)
	}
	fmt.Println()
	fmt.Println(e.Body)
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
