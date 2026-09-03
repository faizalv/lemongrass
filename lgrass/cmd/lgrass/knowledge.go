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
		fmt.Fprintln(os.Stderr, "usage: lgrass knowledge <write|search|read|reindex> ...")
		os.Exit(1)
	}
	switch args[0] {
	case "write":
		cmdKnowledgeWrite(args[1:])
	case "search":
		cmdKnowledgeSearch(args[1:])
	case "read":
		cmdKnowledgeRead(args[1:])
	case "reindex":
		cmdKnowledgeReindex()
	default:
		fmt.Fprintf(os.Stderr, "unknown knowledge command: %s\n", args[0])
		os.Exit(1)
	}
}

// currentProject resolves the current working directory to its registered
// project.
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
	var series, title string
	var part, partTotal int

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--tags":
			i++
			if i < len(args) {
				tags = splitCSV(args[i])
			}
		case "--series":
			i++
			if i < len(args) {
				series = args[i]
			}
		case "--part":
			i++
			if i < len(args) {
				part, partTotal = parsePart(args[i])
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
		Title:     title,
		Tags:      tags,
		Series:    series,
		Part:      part,
		PartTotal: partTotal,
		Body:      string(body),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%s)\n", entry.ID, entry.Title)
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
		line := fmt.Sprintf("%s\t%s", r.ID, r.Title)
		if len(r.Tags) > 0 {
			line += "\t[" + strings.Join(r.Tags, ", ") + "]"
		}
		if r.Series != "" {
			line += fmt.Sprintf("\t(series %s, part %d/%d)", r.Series, r.Part, r.PartTotal)
		}
		fmt.Println(line)
	}
}

func cmdKnowledgeRead(args []string) {
	p := currentProject()

	if len(args) >= 2 && args[0] == "--series" {
		readSeries(p, args[1])
		return
	}
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: lgrass knowledge read <id> | --series <id>")
		os.Exit(1)
	}
	entry, err := knowledge.Read(p.ID, args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	printEntry(entry)
}

func readSeries(p project.Project, seriesID string) {
	store, err := knowledge.Open(knowledge.DBPath(), p.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	results, err := store.Series(seriesID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(results) == 0 {
		fmt.Fprintf(os.Stderr, "no entries in series %s\n", seriesID)
		os.Exit(1)
	}
	for i, r := range results {
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
	count, err := knowledge.Reindex(p.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("reindexed %d entries\n", count)
}

func printEntry(e knowledge.Entry) {
	fmt.Printf("# %s\n", e.Title)
	if len(e.Tags) > 0 {
		fmt.Printf("tags: %s\n", strings.Join(e.Tags, ", "))
	}
	if e.Series != "" {
		fmt.Printf("series: %s (part %d/%d)\n", e.Series, e.Part, e.PartTotal)
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

func parsePart(s string) (int, int) {
	pieces := strings.SplitN(s, "/", 2)
	part, _ := strconv.Atoi(strings.TrimSpace(pieces[0]))
	total := 0
	if len(pieces) == 2 {
		total, _ = strconv.Atoi(strings.TrimSpace(pieces[1]))
	}
	return part, total
}
