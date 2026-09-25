package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/faizalv/lemongrass/agent"
)

const resterUsage = `usage:
  lgrass rester <short-id> info
  lgrass rester <short-id> users
  lgrass rester <short-id> flush [--user <name>]
  lgrass rester <short-id> <get|post|put|patch|delete|head|options> <path> [--user <name>] [--body '<json>']

info prints the channel's base URL, how long it stays valid, the methods it allows and its
users. users prints just the users with their tags. The request form proxies one HTTP call
through the channel's domain; the vault handles login and token injection. <path> is relative
to the domain's base URL, a full URL under it also works. --user may be left out when the
domain has a single user. Response is printed as JSON:
{"status": N, "headers": {...}, "body": ..., "url": "..."}.

The vault keeps each user's login token until it expires and reuses it across requests. flush
drops the cached token of --user, or of every user when --user is left out, so the next request
logs in again and gets a new token. It prints {"flushed": N}, the number of tokens dropped. A
user whose token was pasted in by hand has no login, so it gets the same token back.
`

var resterVerbs = map[string]bool{
	"get": true, "post": true, "put": true, "patch": true, "delete": true, "head": true, "options": true,
}

type resterCommand struct {
	shortID string
	info    bool
	users   bool
	flush   bool
	method  string
	path    string
	user    string
	body    string
}

func parseResterArgs(args []string) (resterCommand, error) {
	if len(args) < 2 {
		return resterCommand{}, fmt.Errorf("expected a channel id and a command")
	}
	cmd := resterCommand{shortID: args[0]}

	action := strings.ToLower(args[1])
	switch {
	case action == "info":
		cmd.info = true
	case action == "users":
		cmd.users = true
	case action == "flush":
		cmd.flush = true
	case resterVerbs[action]:
		cmd.method = strings.ToUpper(action)
	default:
		return resterCommand{}, fmt.Errorf("unknown command %q", args[1])
	}

	var positional []string
	rest := args[2:]
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		if !strings.HasPrefix(arg, "--") {
			positional = append(positional, arg)
			continue
		}
		name, value, hasValue := strings.Cut(arg, "=")
		if !hasValue {
			i++
			if i >= len(rest) {
				return resterCommand{}, fmt.Errorf("%s needs a value", name)
			}
			value = rest[i]
		}
		switch name {
		case "--user":
			cmd.user = value
		case "--body":
			cmd.body = value
		default:
			return resterCommand{}, fmt.Errorf("unknown flag %s", name)
		}
	}

	if cmd.info || cmd.users {
		if len(positional) > 0 || cmd.user != "" || cmd.body != "" {
			return resterCommand{}, fmt.Errorf("%s takes no other arguments", strings.ToLower(args[1]))
		}
		return cmd, nil
	}
	if cmd.flush {
		if len(positional) > 0 || cmd.body != "" {
			return resterCommand{}, fmt.Errorf("flush takes only --user")
		}
		return cmd, nil
	}
	if len(positional) != 1 {
		return resterCommand{}, fmt.Errorf("%s needs exactly one path", strings.ToLower(cmd.method))
	}
	cmd.path = positional[0]
	return cmd, nil
}

func cmdRester(args []string) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(resterUsage)
		return
	}

	cmd, err := parseResterArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n\n%s", err, resterUsage)
		os.Exit(1)
	}

	client := &agent.Client{SocketPath: agentSocketPath()}
	var result any
	switch {
	case cmd.info:
		result, err = client.HTTPChannelInfo(cmd.shortID)
	case cmd.users:
		result, err = client.HTTPUsers(cmd.shortID)
	case cmd.flush:
		var flushed int
		flushed, err = client.FlushHTTPTokens(cmd.shortID, cmd.user)
		result = map[string]int{"flushed": flushed}
	default:
		result, err = client.RequestHTTP(cmd.shortID, cmd.user, cmd.method, cmd.path, []byte(cmd.body))
	}
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
