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

// Blocks until this tab has a pending notification, meant to run backgrounded so its exit wakes an idle model, and relaunched on return. A Claude tab is notified directly, so it only records that it is ready unless --block is given.
func cmdListen(args []string) {
	timeout := defaultListenTimeout
	block := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--timeout":
			i++
			if i < len(args) {
				if d, err := time.ParseDuration(args[i]); err == nil {
					timeout = d
				}
			}
		case "--block":
			block = true
		}
	}
	if tabID == "" {
		fmt.Fprintf(os.Stderr, "lgrass listen: listening needs a lemongrass tab, %s is not set\n", tabIDEnv)
		os.Exit(1)
	}

	store := openStore()
	defer store.Close()

	if vendor, _ := store.VendorForTab(tabID); vendor == "claude" && !block {
		store.MarkReady(tabID, session.MarkListening)
		fmt.Println("lgrass: this tab is notified directly, so there is nothing to wait for. Marked ready. Use `lgrass listen --block` to wait for a notification anyway.")
		return
	}

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
	return consumePending(store, tab)
}

// The tab's pending nudge text with its rows marked sent; false when nothing is pending.
func consumePending(store *session.Store, tab string) (string, bool) {
	pending, err := store.PendingForTab(tab)
	if err != nil || len(pending) == 0 {
		return "", false
	}
	var rowIDs []int64
	for _, p := range pending {
		rowIDs = append(rowIDs, p.RowIDs...)
	}
	store.MarkNotificationsSent(rowIDs)
	store.MarkOtherSent(tab)
	return store.NotificationText(pending), true
}
