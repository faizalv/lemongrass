// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

// Package threadsvc delivers ledger notifications, from threads and other sources: the ledger-driven delivery routines and the socket service the agent process hosts.
package threadsvc

import (
	"github.com/faizalv/lemongrass/internal/session"
	"github.com/faizalv/lemongrass/internal/workgroup"
)

type Deliverer struct {
	Store *session.Store
	// Asks the app to type the tab's nudge into its terminal. The app composes the line and marks the rows typed.
	Nudge func(tabID string) (typed bool, reason string, err error)
	// Set by the service only. Ready reports a tab with no prompt open, and Later asks for another look at a tab that is held.
	Ready func(tabID string) bool
	Later func(tabID string)
	// Records each decision; nil records nothing.
	Log func(tabID string, rowIDs []int64, outcome string)
}

func NewDeliverer(store *session.Store) *Deliverer {
	return &Deliverer{
		Store: store,
		Nudge: func(tabID string) (bool, string, error) {
			return workgroup.Nudge(workgroup.AppSocketPath(), tabID)
		},
		Log: appendDeliveryLog,
	}
}

// Types a coalesced nudge into a supported agent's terminal unless a prompt is open or the human is typing there. A failure leaves the rows owed for the service's next tick, so a tab whose process starts late still gets them.
func (d *Deliverer) DeliverTab(tabID string) (bool, error) {
	vendor, err := d.Store.VendorForTab(tabID)
	if err != nil || (vendor != "claude" && vendor != "codex") {
		return false, err
	}
	pending, err := d.Store.PendingForTab(tabID)
	if err != nil || pending.Empty() {
		return false, err
	}
	rows := pending.RowIDs()
	if d.Ready != nil && !d.Ready(tabID) {
		d.log(tabID, rows, "held, a prompt is open")
		d.later(tabID)
		return false, nil
	}

	typed, reason, err := d.Nudge(tabID)
	switch {
	case err != nil:
		d.log(tabID, rows, "not typed: "+err.Error())
	case typed:
		d.log(tabID, rows, "typed")
	case reason == "typing":
		d.log(tabID, rows, "held, the human is typing")
		d.later(tabID)
	default:
		d.log(tabID, rows, "not typed: "+reason)
	}
	return typed, nil
}

func (d *Deliverer) log(tabID string, rowIDs []int64, outcome string) {
	if d.Log != nil {
		d.Log(tabID, rowIDs, outcome)
	}
}

func (d *Deliverer) later(tabID string) {
	if d.Later != nil {
		d.Later(tabID)
	}
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
