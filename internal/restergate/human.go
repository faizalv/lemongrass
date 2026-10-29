// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package restergate

import (
	"fmt"
	"io"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/faizalv/lemongrass/internal/vault"
)

// HumanCall is one request the human sends from the app's Rester Client, as one of a rester's
// users in one environment. It goes through no channel, so no scope applies: the human holds the
// passphrase and owns the credentials.
type HumanCall struct {
	Rester      string         `json:"rester"`
	Environment string         `json:"environment"`
	User        string         `json:"user"`
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	Body        []byte         `json:"body,omitempty"`
	ContentType string         `json:"content_type,omitempty"`
	Headers     []vault.Header `json:"headers,omitempty"`
}

// HumanResult is a human call's outcome. Body is the raw text, so the app shows it exactly as sent
// and keeps the order of JSON keys. A body that is not UTF-8 text is left out with Binary set.
// Size counts the bytes read, which stop at the inline limit with Truncated set.
type HumanResult struct {
	Status     int                 `json:"status"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body"`
	Binary     bool                `json:"binary"`
	Size       int                 `json:"size"`
	Truncated  bool                `json:"truncated"`
	DurationMs int64               `json:"duration_ms"`
	URL        string              `json:"url"`
}

func humanOwner(rester, environment string) tokenOwner {
	return tokenOwner{rester: rester, environment: environment}
}

// HumanRequest sends call with the token of its user, logging in on first use and caching the
// token until the vault daemon stops, a flush or an edit of the rester.
func (g *Gate) HumanRequest(rootSecret string, call HumanCall) (HumanResult, error) {
	start := time.Now()
	resp, target, err := g.humanExchange(rootSecret, call, false)
	if err != nil {
		return HumanResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLoginResponseBytes+1))
	if err != nil {
		return HumanResult{}, fmt.Errorf("restergate: reading response: %w", err)
	}
	result := HumanResult{
		Status:     resp.StatusCode,
		Headers:    map[string][]string(resp.Header),
		DurationMs: time.Since(start).Milliseconds(),
		URL:        target.URL,
	}
	if len(body) > maxLoginResponseBytes {
		body = body[:maxLoginResponseBytes]
		result.Truncated = true
	}
	result.Size = len(body)
	// A cut can split a multi-byte rune, so only the tail past the last full rune is forgiven.
	text := body
	if result.Truncated {
		for i := 0; i < utf8.UTFMax && len(text) > 0 && !utf8.Valid(text); i++ {
			text = text[:len(text)-1]
		}
	}
	if utf8.Valid(text) {
		result.Body = string(text)
	} else {
		result.Binary = true
	}
	return result, nil
}

// HumanDownload makes the same call as HumanRequest but hands back a 2xx body unread, so a body of
// any size can be streamed to a file.
func (g *Gate) HumanDownload(rootSecret string, call HumanCall) (Download, error) {
	resp, target, err := g.humanExchange(rootSecret, call, true)
	if err != nil {
		return Download{}, err
	}
	return downloadFrom(resp, target)
}

// FlushHumanTokens drops the human's cached tokens for one rester environment, for one user or,
// when user is empty, for every user, and returns how many were dropped.
func (g *Gate) FlushHumanTokens(rester, environment, user string) int {
	owner := humanOwner(rester, environment)
	g.mu.Lock()
	defer g.mu.Unlock()
	flushed := 0
	for key, t := range g.tokens {
		if key.owner != owner || (user != "" && vault.UserHandle(key.user) != vault.UserHandle(user)) {
			continue
		}
		vault.LockedFree(t.locked)
		delete(g.tokens, key)
		flushed++
	}
	return flushed
}

// forgetHumanTokens drops every human token of a rester, in every environment, after it is edited
// or deleted.
func (g *Gate) forgetHumanTokens(rester string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for key, t := range g.tokens {
		if key.owner.channel == "" && key.owner.rester == rester {
			vault.LockedFree(t.locked)
			delete(g.tokens, key)
		}
	}
}

func (g *Gate) humanExchange(rootSecret string, call HumanCall, stream bool) (*http.Response, requestTarget, error) {
	rester, err := g.vault.ResolveRester(rootSecret, call.Rester, call.Environment)
	if err != nil {
		return nil, requestTarget{}, err
	}
	user, err := resolveUser(rester, call.User)
	if err != nil {
		return nil, requestTarget{}, err
	}
	target, err := resolveRequestTarget(rester.BaseURL, call.Path)
	if err != nil {
		return nil, requestTarget{}, err
	}
	resp, err := g.send(humanOwner(call.Rester, call.Environment), rester, user, call.Method, target, call.Body, call.ContentType, call.Headers, stream)
	if err != nil {
		return nil, requestTarget{}, err
	}
	return resp, target, nil
}
