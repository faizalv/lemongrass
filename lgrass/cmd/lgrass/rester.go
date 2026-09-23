package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/faizalv/lemongrass/agent"
)

const resterUsage = `usage: lgrass rester <short-id> --user <name> --method <METHOD> --path <path> [--body '<json>']

Proxies one HTTP call through the channel's domain, authenticated as user; the vault handles
login and token injection. Response is printed as JSON:
{"status": N, "headers": {...}, "body": ...}.
`

func cmdRester(args []string) {
	if len(args) == 0 {
		fmt.Print(resterUsage)
		return
	}

	shortID := args[0]
	var user, method, path, body string
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--user":
			i++
			if i < len(args) {
				user = args[i]
			}
		case "--method":
			i++
			if i < len(args) {
				method = args[i]
			}
		case "--path":
			i++
			if i < len(args) {
				path = args[i]
			}
		case "--body":
			i++
			if i < len(args) {
				body = args[i]
			}
		}
	}
	if user == "" || method == "" || path == "" {
		fmt.Fprint(os.Stderr, resterUsage)
		os.Exit(1)
	}

	client := &agent.Client{SocketPath: agentSocketPath()}
	result, err := client.RequestHTTP(shortID, user, method, path, []byte(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	out, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}
