package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/faizalv/lemongrass/session"
)

const (
	defaultThreadListLimit = 20
	defaultListenTimeout   = 10 * time.Minute
	listenPollInterval     = 2 * time.Second
)

func cmdThread(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: lgrass thread <post|list|listen> ...")
		os.Exit(1)
	}
	switch args[0] {
	case "post":
		cmdThreadPost(args[1:])
	case "list":
		cmdThreadList(args[1:])
	case "listen":
		cmdThreadListen(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown thread command: %s\n", args[0])
		os.Exit(1)
	}
}

func cmdThreadPost(args []string) {
	var mention string
	var body string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--mention":
			i++
			if i < len(args) {
				mention = args[i]
			}
		default:
			if body == "" {
				body = args[i]
			}
		}
	}
	if body == "" {
		fmt.Fprintln(os.Stderr, `usage: lgrass thread post "<message>" [--mention <session-id>]`)
		os.Exit(1)
	}

	authorName := currentParticipantName()
	proj := currentProject()
	store, err := session.Open(session.DBPath(), proj.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	if _, err := store.PostThreadMessage(authorName, body, mention); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Best effort: push straight into every other live session's inbox
	// socket for immediate delivery. Never fatal. A session with no
	// captured socket, or a socket that's gone stale, still gets the
	// message via the hook-surfaced pull fallback (UnreadMentions) on
	// its next tool call, for a mention specifically.
	exclude := currentClaudeSessionExclusion(store)
	targets, err := store.LiveMessagingTargets(exclude)
	if err == nil && len(targets) > 0 {
		text := session.FormatThreadPush(authorName, body)
		delivered := session.DeliverAll(targets, text)
		fmt.Printf("posted. delivered live to %d/%d other session(s).\n", delivered, len(targets))
		return
	}
	fmt.Println("posted.")
}

func cmdThreadList(args []string) {
	limit := defaultThreadListLimit
	for i := 0; i < len(args); i++ {
		if args[i] == "--limit" {
			i++
			if i < len(args) {
				if n, err := strconv.Atoi(args[i]); err == nil {
					limit = n
				}
			}
		}
	}

	proj := currentProject()
	store, err := session.Open(session.DBPath(), proj.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	msgs, err := store.RecentThreadMessages(limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(session.FormatThreadList(msgs))
}

// cmdThreadListen blocks, polling for any new thread message in this
// project until one arrives or timeout elapses, then exits. Meant to
// be run as a background shell call so a session gets a live channel
// instead of waiting on its next tool-call hook to check in. Vendor
// neutral by construction: it only touches lgrass's own sqlite, so it
// works the same from any agent CLI that can background a shell command
// and learn when it produces output, not just Claude Code.
func cmdThreadListen(args []string) {
	timeout := defaultListenTimeout
	for i := 0; i < len(args); i++ {
		if args[i] == "--timeout" {
			i++
			if i < len(args) {
				if d, err := time.ParseDuration(args[i]); err == nil {
					timeout = d
				}
			}
		}
	}

	proj := currentProject()
	store, err := session.Open(session.DBPath(), proj.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	cursor, err := store.LatestThreadMessageID()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	deadline := time.Now().Add(timeout)
	for {
		msgs, err := store.NewThreadMessagesAfter(cursor)
		if err == nil && len(msgs) > 0 {
			fmt.Println(session.FormatThreadList(msgs))
			return
		}
		if time.Now().After(deadline) {
			fmt.Printf("lgrass: no new messages in %s. Relaunch `lgrass thread listen` to keep listening.\n", timeout)
			return
		}
		time.Sleep(listenPollInterval)
	}
}
