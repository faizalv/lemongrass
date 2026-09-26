// Package threadsvc delivers thread notifications: the ledger-driven delivery routines and the socket service the agent process hosts.
package threadsvc

import (
	"time"

	"github.com/faizalv/lemongrass/session"
)

const (
	vendorClaude = "claude"

	// Five attempts within roughly ten minutes with the backoff below, then the row stays pending for a hook or listener.
	MaxAttempts = 5

	baseBackoff = 30 * time.Second
)

// The wait before attempt n+1 once n attempts have been made.
func Backoff(attempts int) time.Duration {
	if attempts < 1 {
		return 0
	}
	return baseBackoff << (attempts - 1)
}

type Deliverer struct {
	Store *session.Store
	Push  func(session.MessagingTarget, string) error
}

func NewDeliverer(store *session.Store) *Deliverer {
	return &Deliverer{Store: store, Push: session.Deliver}
}

// Pushes one coalesced nudge to a Claude tab's inbox socket. Other vendors are left pending for their listener or a hook, and it reports whether a push reached the tab.
func (d *Deliverer) DeliverTab(tabID string) (bool, error) {
	vendor, err := d.Store.VendorForTab(tabID)
	if err != nil || vendor != vendorClaude {
		return false, err
	}
	pending, err := d.Store.PendingForTab(tabID)
	if err != nil || len(pending) == 0 {
		return false, err
	}
	var rowIDs []int64
	for _, p := range pending {
		rowIDs = append(rowIDs, p.RowIDs...)
	}

	target, ok, err := d.Store.MessagingTargetForTab(tabID)
	if err != nil {
		return false, err
	}
	if !ok || d.Push(target, session.FormatNotification(pending)) != nil {
		return false, d.Store.RecordNotificationAttempt(rowIDs)
	}
	return true, d.Store.MarkNotificationsSent(rowIDs)
}

// The direct path a posting command takes when the service is unreachable.
func (d *Deliverer) DeliverPending() {
	tabs, err := d.Store.PendingTabs()
	if err != nil {
		return
	}
	for _, tab := range tabs {
		d.DeliverTab(tab)
	}
}
