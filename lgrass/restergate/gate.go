// Package restergate makes HTTP calls through a channel: it checks scope, logs users in, caches their tokens and sends the request. The domain config comes from the vault and only tokens are cached here.
package restergate

import (
	"sync"
	"time"

	"github.com/faizalv/lemongrass/vault"
)

// RequestTimeout bounds one proxied call end to end at every hop: the caller's socket, both
// daemons' connections and the upstream request. Login and admin operations keep their own deadline.
var RequestTimeout = 120 * time.Second

type Gate struct {
	vault *vault.Service

	mu     sync.Mutex
	tokens map[tokenCacheKey]cachedToken
}

// New returns a Gate reading domain config from svc and dropping a channel's cached tokens whenever the vault invalidates that channel.
func New(svc *vault.Service) *Gate {
	g := &Gate{vault: svc, tokens: make(map[tokenCacheKey]cachedToken)}
	svc.OnInvalidate(g.forgetTokens)
	return g
}
