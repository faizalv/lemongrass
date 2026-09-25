package restergate

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/faizalv/lemongrass/vault"
)

// tokenCacheKey identifies one cached token: a channel plus which of the channel's domain
// users it belongs to.
type tokenCacheKey struct {
	channel vault.ChannelID
	user    string
}

// cachedToken holds a token obtained (or pasted, for a bring-your-own-token user) for one
// user, locked in memory the same way a channel's derived key is, never persisted to disk
// between vault restarts.
type cachedToken struct {
	locked    []byte
	expiresAt time.Time
	// everUsed gates the reactive re-login-and-retry signal: only a token that has already
	// succeeded once is treated as "went stale", not a token that was simply always wrong.
	everUsed bool
}

// tokenRefreshMargin is how far ahead of a cached token's computed expiry obtainToken
// proactively re-logs in, so a request doesn't race an about-to-expire token.
const tokenRefreshMargin = 30 * time.Second

func (g *Gate) cachedTokenFor(id vault.ChannelID, user string) (cachedToken, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	t, ok := g.tokens[tokenCacheKey{id, user}]
	return t, ok
}

func (g *Gate) setCachedToken(id vault.ChannelID, user, token string, expiresAt time.Time) error {
	locked, err := vault.LockKey([]byte(token))
	if err != nil {
		return err
	}
	key := tokenCacheKey{id, user}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.tokens == nil {
		g.tokens = make(map[tokenCacheKey]cachedToken)
	}
	if old, ok := g.tokens[key]; ok {
		vault.LockedFree(old.locked)
	}
	g.tokens[key] = cachedToken{locked: locked, expiresAt: expiresAt}
	return nil
}

// markTokenUsed records that a call using this cached token just came back 2xx, arming the
// reactive re-login-and-retry signal for its next use.
func (g *Gate) markTokenUsed(id vault.ChannelID, user string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	key := tokenCacheKey{id, user}
	if t, ok := g.tokens[key]; ok {
		t.everUsed = true
		g.tokens[key] = t
	}
}

func (g *Gate) evictCachedToken(id vault.ChannelID, user string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	key := tokenCacheKey{id, user}
	if t, ok := g.tokens[key]; ok {
		vault.LockedFree(t.locked)
		delete(g.tokens, key)
	}
}

// forgetTokens evicts every cached token for a channel, on revocation or TTL expiry.
func (g *Gate) forgetTokens(id vault.ChannelID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for key, t := range g.tokens {
		if key.channel == id {
			vault.LockedFree(t.locked)
			delete(g.tokens, key)
		}
	}
}

// resolveUser returns the user a call runs as: the named one, or the domain's only user when
// name is empty. Otherwise the error lists the users a caller can choose from.
func resolveUser(domain vault.Domain, name string) (string, error) {
	if name == "" && len(domain.Users) == 1 {
		return domain.Users[0].Name, nil
	}
	if _, err := findDomainUser(domain, name); err == nil {
		return name, nil
	}
	if len(domain.Users) == 0 {
		return "", errors.New("restergate: this domain has no users")
	}
	names := make([]string, len(domain.Users))
	for i, u := range domain.Users {
		names[i] = u.Name
		if len(u.Tags) > 0 {
			names[i] += " (" + strings.Join(u.Tags, ", ") + ")"
		}
	}
	if name == "" {
		return "", fmt.Errorf("restergate: this domain has several users, pass --user with one of: %s", strings.Join(names, ", "))
	}
	return "", fmt.Errorf("restergate: domain has no user %q, available users: %s", name, strings.Join(names, ", "))
}

func findDomainUser(domain vault.Domain, userName string) (vault.DomainUser, error) {
	for _, u := range domain.Users {
		if u.Name == userName {
			return u, nil
		}
	}
	return vault.DomainUser{}, fmt.Errorf("restergate: domain has no user %q", userName)
}

// obtainToken returns user's current token: a cached one if it isn't within
// tokenRefreshMargin of its computed expiry, otherwise a fresh one -- a login user re-logs in
// via domain.LoginEndpoint, a bring-your-own-token user's stored token is reused directly
// since there's no login call to refresh from.
func (g *Gate) obtainToken(id vault.ChannelID, domain vault.Domain, userName string) (string, bool, error) {
	if cached, ok := g.cachedTokenFor(id, userName); ok && time.Until(cached.expiresAt) > tokenRefreshMargin {
		return string(cached.locked), cached.everUsed, nil
	}
	return g.refreshToken(id, domain, userName)
}

// refreshToken always obtains a fresh token, bypassing any cached one -- used both by
// obtainToken on a cache miss/near-expiry and by the reactive re-login-and-retry path.
func (g *Gate) refreshToken(id vault.ChannelID, domain vault.Domain, userName string) (string, bool, error) {
	user, err := findDomainUser(domain, userName)
	if err != nil {
		return "", false, err
	}

	var token string
	var expiresAt time.Time
	if user.IsBYOT() {
		expiresAt, err = byotExpiry(domain, user)
		if err != nil {
			return "", false, err
		}
		token = user.Token
	} else {
		token, expiresAt, err = loginHTTP(domain, user)
		if err != nil {
			return "", false, err
		}
	}

	if err := g.setCachedToken(id, userName, token, expiresAt); err != nil {
		return "", false, err
	}
	return token, false, nil
}
