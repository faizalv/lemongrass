package vault

import (
	"fmt"
	"time"
)

// tokenCacheKey identifies one cached token: a channel plus which of the channel's domain
// users it belongs to.
type tokenCacheKey struct {
	channel ChannelID
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

func (s *Service) cachedTokenFor(id ChannelID, user string) (cachedToken, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tokens[tokenCacheKey{id, user}]
	return t, ok
}

func (s *Service) setCachedToken(id ChannelID, user, token string, expiresAt time.Time) error {
	locked, err := lockKey([]byte(token))
	if err != nil {
		return err
	}
	key := tokenCacheKey{id, user}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tokens == nil {
		s.tokens = make(map[tokenCacheKey]cachedToken)
	}
	if old, ok := s.tokens[key]; ok {
		lockedFree(old.locked)
	}
	s.tokens[key] = cachedToken{locked: locked, expiresAt: expiresAt}
	return nil
}

// markTokenUsed records that a call using this cached token just came back 2xx, arming the
// reactive re-login-and-retry signal for its next use.
func (s *Service) markTokenUsed(id ChannelID, user string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := tokenCacheKey{id, user}
	if t, ok := s.tokens[key]; ok {
		t.everUsed = true
		s.tokens[key] = t
	}
}

func (s *Service) evictCachedToken(id ChannelID, user string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := tokenCacheKey{id, user}
	if t, ok := s.tokens[key]; ok {
		lockedFree(t.locked)
		delete(s.tokens, key)
	}
}

// forgetTokens evicts every cached token for a channel, on revocation or TTL expiry.
func (s *Service) forgetTokens(id ChannelID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, t := range s.tokens {
		if key.channel == id {
			lockedFree(t.locked)
			delete(s.tokens, key)
		}
	}
}

func findDomainUser(domain Domain, userName string) (DomainUser, error) {
	for _, u := range domain.Users {
		if u.Name == userName {
			return u, nil
		}
	}
	return DomainUser{}, fmt.Errorf("vault: domain has no user %q", userName)
}

// obtainToken returns user's current token: a cached one if it isn't within
// tokenRefreshMargin of its computed expiry, otherwise a fresh one -- a login user re-logs in
// via domain.LoginEndpoint, a bring-your-own-token user's stored token is reused directly
// since there's no login call to refresh from.
func (s *Service) obtainToken(id ChannelID, domain Domain, userName string) (string, bool, error) {
	if cached, ok := s.cachedTokenFor(id, userName); ok && time.Until(cached.expiresAt) > tokenRefreshMargin {
		return string(cached.locked), cached.everUsed, nil
	}
	return s.refreshToken(id, domain, userName)
}

// refreshToken always obtains a fresh token, bypassing any cached one -- used both by
// obtainToken on a cache miss/near-expiry and by the reactive re-login-and-retry path.
func (s *Service) refreshToken(id ChannelID, domain Domain, userName string) (string, bool, error) {
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

	if err := s.setCachedToken(id, userName, token, expiresAt); err != nil {
		return "", false, err
	}
	return token, false, nil
}
