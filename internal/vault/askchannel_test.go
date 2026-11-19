// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package vault

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type askChannels struct {
	svc                                *Service
	dbAsk, dbHuman, httpAsk, httpHuman ChannelID
}

// newAskChannels makes one ask-made and one human-made channel of each kind, all with ttl.
func newAskChannels(t *testing.T, ttl time.Duration) askChannels {
	t.Helper()
	svc := putMultiEnv(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}
	a := askChannels{svc: svc}
	db := func(name string) ChannelID {
		c, err := svc.CreateChannel(testRootSecret, name, "app-backend", fullScope(), ttl)
		if err != nil {
			t.Fatalf("CreateChannel: %v", err)
		}
		return c.ID
	}
	http := func(name string) ChannelID {
		c, err := svc.CreateHTTPChannel(testRootSecret, name, "shop", "staging", TypeAPI, HTTPScope{Methods: []string{"GET"}}, nil, ttl)
		if err != nil {
			t.Fatalf("CreateHTTPChannel: %v", err)
		}
		return c.ID
	}
	a.dbAsk, a.dbHuman, a.httpAsk, a.httpHuman = db("ask"), db("human"), http("ask"), http("human")
	if err := svc.MarkFromAsk(a.dbAsk); err != nil {
		t.Fatalf("MarkFromAsk: %v", err)
	}
	if err := svc.MarkHTTPFromAsk(a.httpAsk); err != nil {
		t.Fatalf("MarkHTTPFromAsk: %v", err)
	}
	return a
}

func (a askChannels) wantGone(t *testing.T, gone bool) {
	t.Helper()
	_, dbErr := a.svc.ChannelScope(a.dbAsk)
	_, httpErr := a.svc.HTTPChannelScope(a.httpAsk)
	if gone != errors.Is(dbErr, ErrNotFound) || gone != errors.Is(httpErr, ErrNotFound) {
		t.Errorf("ask channels gone = (%v, %v), want both %v", dbErr, httpErr, gone)
	}
	if _, err := a.svc.ChannelScope(a.dbHuman); err != nil {
		t.Errorf("human db channel: %v, want it kept", err)
	}
	if _, err := a.svc.HTTPChannelScope(a.httpHuman); err != nil {
		t.Errorf("human HTTP channel: %v, want it kept", err)
	}
}

func TestSweepAskChannelsDeletesOnlyExpiredAskChannels(t *testing.T) {
	live := newAskChannels(t, time.Hour)
	if err := live.svc.SweepAskChannels(time.Now()); err != nil {
		t.Fatalf("SweepAskChannels: %v", err)
	}
	live.wantGone(t, false)

	expired := newAskChannels(t, -time.Hour)
	if err := expired.svc.SweepAskChannels(time.Now()); err != nil {
		t.Fatalf("SweepAskChannels: %v", err)
	}
	expired.wantGone(t, true)
}

func TestSweepAskChannelsSkipsAChannelActivatedSinceExpiry(t *testing.T) {
	a := newAskChannels(t, -time.Hour)
	if _, err := a.svc.Activate(testRootSecret, a.dbAsk, time.Hour); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if err := a.svc.SweepAskChannels(time.Now()); err != nil {
		t.Fatalf("SweepAskChannels: %v", err)
	}
	if _, err := a.svc.ChannelScope(a.dbAsk); err != nil {
		t.Errorf("reactivated ask channel: %v, want it kept", err)
	}
}

func TestOpenDeletesAnExpiredAskChannel(t *testing.T) {
	a := newAskChannels(t, -time.Hour)
	if _, _, err := a.svc.OpenChannel(a.dbAsk); !errors.Is(err, ErrChannelExpired) {
		t.Errorf("OpenChannel = %v, want ErrChannelExpired", err)
	}
	if _, _, err := a.svc.OpenHTTPChannel(a.httpAsk); !errors.Is(err, ErrChannelExpired) {
		t.Errorf("OpenHTTPChannel = %v, want ErrChannelExpired", err)
	}
	a.wantGone(t, true)
}

func TestExpireKeylessChannelsDeletesAskChannels(t *testing.T) {
	a := newAskChannels(t, time.Hour)
	a.svc.mu.Lock()
	clear(a.svc.activeKeys)
	a.svc.mu.Unlock()
	if err := a.svc.ExpireKeylessChannels(time.Now()); err != nil {
		t.Fatalf("ExpireKeylessChannels: %v", err)
	}
	a.wantGone(t, true)
}

func TestChannelsStoredBeforeFromAskReadAsHumanMade(t *testing.T) {
	var c Channel
	if err := json.Unmarshal([]byte(`{"ID":"x","Name":"n"}`), &c); err != nil || c.FromAsk {
		t.Errorf("db channel: FromAsk = %v, err = %v, want false", c.FromAsk, err)
	}
	var h HTTPChannel
	if err := json.Unmarshal([]byte(`{"ID":"x","Name":"n","Domain":"shop"}`), &h); err != nil || h.FromAsk {
		t.Errorf("HTTP channel: FromAsk = %v, err = %v, want false", h.FromAsk, err)
	}
}
