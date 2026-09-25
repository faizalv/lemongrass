package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/faizalv/lemongrass/agent"
	"github.com/faizalv/lemongrass/vault"
)

const resterUsage = `usage:
  lgrass rester <short-id> info
  lgrass rester <short-id> users
  lgrass rester <short-id> flush [--user <name>]
  lgrass rester <short-id> <get|post|put|patch|delete|head|options> <path> [--user <name>] [body flags]

body flags, at most one kind per request:
  --body '<json>'                       an inline JSON body
  --body-file <path> [--content-type <mime>]
                                        the file's bytes as the body, JSON unless --content-type says otherwise
  --file <field>=<path>[;type=<mime>]   a file as one part of a multipart/form-data body, repeatable
  --form <key>=<value>                  a form field, repeatable; with --file it joins the multipart body,
                                        alone it makes an application/x-www-form-urlencoded body

info prints the channel's base URL, how long it stays valid, the methods it allows and its
users. users prints just the users with their tags. The request form proxies one HTTP call
through the channel's domain; the vault handles login and token injection. <path> is relative
to the domain's base URL, a full URL under it also works. --user may be left out when the
domain has a single user. Response is printed as JSON:
{"status": N, "headers": {...}, "body": ..., "url": "..."}.

Files are read from this machine as the current user and sent as they are, so a spreadsheet
arrives byte for byte. A part's type comes from the file extension unless ;type= is given. A
request body is limited to 25 MiB. A redirect to another host is not followed for a request
that sends a file, so the file is never resent elsewhere.

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

	bodyFile    string
	contentType string
	forms       []formField
	files       []filePart
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
		case "--body-file":
			cmd.bodyFile = value
		case "--content-type":
			cmd.contentType = value
		case "--form":
			field, err := parseFormField(value)
			if err != nil {
				return resterCommand{}, err
			}
			cmd.forms = append(cmd.forms, field)
		case "--file":
			part, err := parseFilePart(value)
			if err != nil {
				return resterCommand{}, err
			}
			cmd.files = append(cmd.files, part)
		default:
			return resterCommand{}, fmt.Errorf("unknown flag %s", name)
		}
	}

	if cmd.info || cmd.users {
		if len(positional) > 0 || cmd.user != "" || cmd.hasBodyFlags() {
			return resterCommand{}, fmt.Errorf("%s takes no other arguments", strings.ToLower(args[1]))
		}
		return cmd, nil
	}
	if cmd.flush {
		if len(positional) > 0 || cmd.hasBodyFlags() {
			return resterCommand{}, fmt.Errorf("flush takes only --user")
		}
		return cmd, nil
	}
	if len(positional) != 1 {
		return resterCommand{}, fmt.Errorf("%s needs exactly one path", strings.ToLower(cmd.method))
	}
	cmd.path = positional[0]
	if err := cmd.checkBodyFlags(); err != nil {
		return resterCommand{}, err
	}
	return cmd, nil
}

func (c resterCommand) hasBodyFlags() bool {
	return c.body != "" || c.bodyFile != "" || c.contentType != "" || len(c.forms) > 0 || len(c.files) > 0
}

// checkBodyFlags rejects a request that mixes body kinds or gives --content-type without a
// file to describe.
func (c resterCommand) checkBodyFlags() error {
	kinds := 0
	for _, used := range []bool{c.body != "", c.bodyFile != "", len(c.forms) > 0 || len(c.files) > 0} {
		if used {
			kinds++
		}
	}
	if kinds > 1 {
		return fmt.Errorf("use only one kind of body: --body, --body-file, or --form and --file")
	}
	if c.contentType != "" && c.bodyFile == "" {
		return fmt.Errorf("--content-type only goes with --body-file")
	}
	if c.contentType != "" {
		if err := vault.CheckContentType(c.contentType); err != nil {
			return fmt.Errorf("--content-type %q is not a valid media type", c.contentType)
		}
	}
	return nil
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
		var body []byte
		var contentType string
		body, contentType, err = buildResterBody(cmd)
		if err == nil {
			result, err = client.RequestHTTP(cmd.shortID, cmd.user, cmd.method, cmd.path, body, contentType)
		}
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
