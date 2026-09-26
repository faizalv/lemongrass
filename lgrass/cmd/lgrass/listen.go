package main

import (
	"fmt"
	"os"
	"time"

	"github.com/faizalv/lemongrass/session"
	"github.com/faizalv/lemongrass/threadsvc"
)

const (
	defaultListenTimeout = 10 * time.Minute
	listenWaitSeconds    = 10
	listenPollInterval   = 2 * time.Second
)

// Blocks until this tab has a pending notification, meant to run backgrounded so its exit wakes an idle model, and relaunched on return.
func cmdListen(args []string) {
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
	if tabID == "" {
		fmt.Fprintf(os.Stderr, "lgrass listen: listening needs a lemongrass tab, %s is not set\n", tabIDEnv)
		os.Exit(1)
	}

	store := openStore()
	defer store.Close()

	deadline := time.Now().Add(timeout)
	for {
		if text, ok := listenOnce(store, tabID); ok {
			fmt.Println(text)
			return
		}
		if time.Now().After(deadline) {
			fmt.Printf("lgrass: no new messages in %s. Relaunch `lgrass listen` to keep listening.\n", timeout)
			return
		}
		if _, err := threadsvc.Wait(threadsvc.SocketPath(), tabID, listenWaitSeconds); err != nil {
			time.Sleep(listenPollInterval)
		}
	}
}

// One pass of the listener loop: writes the heartbeat, then consumes the tab's pending notifications.
func listenOnce(store *session.Store, tab string) (string, bool) {
	store.Heartbeat(tab)
	pending, err := store.PendingForTab(tab)
	if err != nil || len(pending) == 0 {
		return "", false
	}
	var rowIDs []int64
	for _, p := range pending {
		rowIDs = append(rowIDs, p.RowIDs...)
	}
	store.MarkNotificationsSent(rowIDs)
	return store.NotificationText(pending), true
}
