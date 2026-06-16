package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

func cmdMap(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: lemongrass map <prune> [args...]\n")
		os.Exit(1)
	}
	switch args[0] {
	case "prune":
		cmdMapPrune(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown map subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdMapPrune(args []string) {
	orphanDays := 30
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--orphan-days" && i+1 < len(args):
			if n, err := strconv.Atoi(args[i+1]); err == nil && n > 0 {
				orphanDays = n
			}
			i++
		}
	}

	cfg := resolveProject()

	q := url.Values{}
	q.Set("orphan_days", strconv.Itoa(orphanDays))

	fmt.Print("Pruning semantic map... ")
	resp, err := http.Post(
		cfg.ServerURL+fmt.Sprintf("/api/recon/projects/%d/prune?%s", cfg.ProjectID, q.Encode()),
		"application/json", nil,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nerror: server unreachable: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "\nerror: server returned %d: %s\n", resp.StatusCode, body)
		os.Exit(1)
	}

	var result struct {
		Superseded int `json:"superseded"`
		Orphans    int `json:"orphans"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "\nerror: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("done\n%d superseded node(s) removed, %d orphan(s) removed (cutoff: %d days)\n",
		result.Superseded, result.Orphans, orphanDays)
}
