package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/faizalv/lemongrass/session"
	"github.com/faizalv/lemongrass/threadsvc"
)

const (
	defaultThreadListLimit = 20
	defaultThreadReadLimit = 10
)

type threadArgs struct {
	positional []string
	file       string
	before     int64
	limit      int
}

func parseThreadArgs(args []string, defaultLimit int) threadArgs {
	parsed := threadArgs{limit: defaultLimit}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file":
			i++
			if i < len(args) {
				parsed.file = args[i]
			}
		case "--before":
			i++
			if i < len(args) {
				parsed.before, _ = strconv.ParseInt(args[i], 10, 64)
			}
		case "--limit":
			i++
			if i < len(args) {
				if n, err := strconv.Atoi(args[i]); err == nil && n > 0 {
					parsed.limit = n
				}
			}
		default:
			parsed.positional = append(parsed.positional, args[i])
		}
	}
	return parsed
}

func cmdThread(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: lgrass thread <create|post|read|list> ...")
		os.Exit(1)
	}
	switch args[0] {
	case "create":
		cmdThreadCreate(args[1:])
	case "post":
		cmdThreadPost(args[1:])
	case "read":
		cmdThreadRead(args[1:])
	case "list":
		cmdThreadList(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown thread command: %s\n", args[0])
		os.Exit(1)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func requireTab() {
	if tabID == "" {
		fmt.Fprintf(os.Stderr, "lgrass thread: posting needs a lemongrass tab, %s is not set\n", tabIDEnv)
		os.Exit(1)
	}
}

// Wakes the thread service, or delivers directly and prunes the ledger itself when the service is unreachable.
func notifyTargets(store *session.Store) {
	if threadsvc.Wake(threadsvc.SocketPath()) == nil {
		return
	}
	threadsvc.NewDeliverer(store).DeliverPending()
	store.PruneNotifications(time.Now())
}

func openStore() *session.Store {
	store, err := session.Open(session.DBPath(), currentProject().ID)
	if err != nil {
		fail(err)
	}
	return store
}

// Content is the positional argument, "-" for stdin, or --file, so long or quote-heavy text never has to pass through a shell argument.
func threadContent(parsed threadArgs, positionalIndex int) (string, error) {
	var text string
	switch {
	case parsed.file != "":
		data, err := os.ReadFile(parsed.file)
		if err != nil {
			return "", err
		}
		text = string(data)
	case len(parsed.positional) > positionalIndex && parsed.positional[positionalIndex] == "-":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		text = string(data)
	case len(parsed.positional) > positionalIndex:
		text = parsed.positional[positionalIndex]
	}
	return strings.TrimSpace(text), nil
}

func cmdThreadCreate(args []string) {
	parsed := parseThreadArgs(args, defaultThreadReadLimit)
	if len(parsed.positional) < 1 {
		fmt.Fprintln(os.Stderr, `usage: lgrass thread create "<title>" "<content>" (content may be - for stdin or --file <path>)`)
		os.Exit(1)
	}
	requireTab()
	content, err := threadContent(parsed, 1)
	if err != nil {
		fail(err)
	}

	store := openStore()
	defer store.Close()
	id, err := store.CreateThread(tabID, parsed.positional[0], content)
	if err != nil {
		fail(err)
	}
	notifyTargets(store)
	fmt.Printf("thread %d created.\n", id)
}

func cmdThreadPost(args []string) {
	parsed := parseThreadArgs(args, defaultThreadReadLimit)
	if len(parsed.positional) < 1 {
		fmt.Fprintln(os.Stderr, `usage: lgrass thread post <thread-id> "<content>" (content may be - for stdin or --file <path>)`)
		os.Exit(1)
	}
	threadID, err := strconv.ParseInt(parsed.positional[0], 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lgrass thread: %q is not a thread id\n", parsed.positional[0])
		os.Exit(1)
	}
	requireTab()
	content, err := threadContent(parsed, 1)
	if err != nil {
		fail(err)
	}

	store := openStore()
	defer store.Close()
	msg, err := store.PostMessage(tabID, threadID, content)
	if err != nil {
		fail(err)
	}
	notifyTargets(store)
	fmt.Printf("posted as message %d.\n", msg.ID)
}

func cmdThreadRead(args []string) {
	parsed := parseThreadArgs(args, defaultThreadReadLimit)
	if len(parsed.positional) < 1 {
		fmt.Fprintln(os.Stderr, "usage: lgrass thread read <thread-id> [--before <message-id>] [--limit N]")
		os.Exit(1)
	}
	threadID, err := strconv.ParseInt(parsed.positional[0], 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lgrass thread: %q is not a thread id\n", parsed.positional[0])
		os.Exit(1)
	}

	store := openStore()
	defer store.Close()
	printThread(store, threadID, parsed, "")
}

// The shared read path: prints the page, then settles this tab's notifications for the messages shown.
func printThread(store *session.Store, threadID int64, parsed threadArgs, header string) {
	thread, err := store.ThreadByID(threadID)
	if err != nil {
		fail(err)
	}
	msgs, more, err := store.ReadThread(threadID, parsed.before, parsed.limit)
	if err != nil {
		fail(err)
	}
	if tabID != "" {
		ids := make([]int64, len(msgs))
		for i, m := range msgs {
			ids[i] = m.ID
		}
		store.MarkMessagesRead(tabID, ids)
	}
	if header != "" {
		fmt.Println(header)
	}
	fmt.Println(session.FormatThreadRead(thread, msgs, more, threadLabels(store, thread, msgs)))
}

func cmdThreadList(args []string) {
	parsed := parseThreadArgs(args, defaultThreadListLimit)

	store := openStore()
	defer store.Close()
	threads, err := store.ListThreads(parsed.limit)
	if err != nil {
		fail(err)
	}
	fmt.Println(session.FormatThreadList(threads, store.Labels(threadCreators(threads))))
}

func threadCreators(threads []session.Thread) []string {
	ids := make([]string, len(threads))
	for i, t := range threads {
		ids[i] = t.CreatedBy
	}
	return ids
}

func threadLabels(store *session.Store, thread session.Thread, msgs []session.Message) map[string]string {
	ids := []string{thread.CreatedBy}
	for _, m := range msgs {
		ids = append(ids, m.TabID)
		ids = append(ids, m.Mentions...)
	}
	return store.Labels(ids)
}
