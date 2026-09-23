package vault

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPResult is a proxied call's outcome. Body is left as whatever shape the upstream sent --
// parsed JSON when the content type says so, a string otherwise -- since an HTTP body has no
// columns/rows to normalize into the way a db QueryResult does.
type HTTPResult struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
	Body    any                 `json:"body"`
}

// RequestHTTP proxies one call through channel id as user: checks expiry and scope, obtains
// (or reuses) that user's token, and on a non-400 4xx from an already-successfully-used token,
// transparently re-logs in and retries exactly once before giving up.
func (s *Service) RequestHTTP(id ChannelID, user, method, requestPath string, body []byte) (HTTPResult, error) {
	c, err := s.loadHTTPMeta(id)
	if err != nil {
		return HTTPResult{}, err
	}
	if c.Expired(time.Now()) {
		s.forgetKey(id)
		s.forgetTokens(id)
		return HTTPResult{}, fmt.Errorf("vault: channel %s has expired. Ask the channel owner to reactivate it or share a fresh channel id.", id)
	}

	s.mu.Lock()
	channelKey, ok := s.activeKeys[id]
	s.mu.Unlock()
	if !ok {
		return HTTPResult{}, fmt.Errorf("vault: channel %s is not active", id)
	}

	if !c.Scope.Allow(method, requestPath) {
		return HTTPResult{}, fmt.Errorf("vault: channel %s does not grant %s %s", id, method, requestPath)
	}

	wrapped, err := s.channels.Get(string(id), channelKey)
	if err != nil {
		return HTTPResult{}, err
	}
	defer zero(wrapped)

	var domain Domain
	if err := json.Unmarshal(wrapped, &domain); err != nil {
		return HTTPResult{}, fmt.Errorf("vault: decoding domain for channel %s: %w", id, err)
	}

	token, everUsed, err := s.obtainToken(id, domain, user)
	if err != nil {
		return HTTPResult{}, err
	}

	result, err := doHTTPRequest(domain, token, method, requestPath, body)
	if err != nil {
		return HTTPResult{}, err
	}

	if everUsed && isStaleTokenSignal(result.Status) {
		result, err = s.retryWithFreshToken(id, domain, user, method, requestPath, body)
		if err != nil {
			return HTTPResult{}, err
		}
	}

	if result.Status >= 200 && result.Status < 300 {
		s.markTokenUsed(id, user)
	}
	return result, nil
}

// retryWithFreshToken evicts the stale cached token, re-logs in (bring-your-own-token users
// have no login to refresh from, so their token is only evicted, not replaced), and retries the
// call exactly once.
func (s *Service) retryWithFreshToken(id ChannelID, domain Domain, user, method, requestPath string, body []byte) (HTTPResult, error) {
	domainUser, err := findDomainUser(domain, user)
	if err != nil {
		return HTTPResult{}, err
	}
	s.evictCachedToken(id, user)
	if domainUser.IsBYOT() {
		return HTTPResult{}, fmt.Errorf("vault: %s's token appears to have gone stale; ask them to paste a fresh one into the domain and re-save it", user)
	}

	freshToken, _, err := s.refreshToken(id, domain, user)
	if err != nil {
		return HTTPResult{}, fmt.Errorf("vault: token appeared stale, re-login also failed: %w", err)
	}
	return doHTTPRequest(domain, freshToken, method, requestPath, body)
}

// isStaleTokenSignal reports whether status is a non-400 4xx, the signal a used token has gone
// stale server-side (revoked, rotated, or simply expired earlier than its computed TTL implied).
func isStaleTokenSignal(status int) bool {
	return status >= 400 && status < 500 && status != http.StatusBadRequest
}

func doHTTPRequest(domain Domain, token, method, requestPath string, body []byte) (HTTPResult, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequest(strings.ToUpper(method), domain.BaseURL+requestPath, reqBody)
	if err != nil {
		return HTTPResult{}, fmt.Errorf("vault: building request: %w", err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	injectToken(req, domain.TokenPlacement, token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return HTTPResult{}, fmt.Errorf("vault: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxLoginResponseBytes))
	if err != nil {
		return HTTPResult{}, fmt.Errorf("vault: reading response: %w", err)
	}

	return HTTPResult{
		Status:  resp.StatusCode,
		Headers: map[string][]string(resp.Header),
		Body:    parseResponseBody(resp.Header.Get("Content-Type"), respBody),
	}, nil
}

func parseResponseBody(contentType string, body []byte) any {
	if strings.Contains(contentType, "json") {
		var v any
		if err := json.Unmarshal(body, &v); err == nil {
			return v
		}
	}
	if len(body) == 0 {
		return nil
	}
	return string(body)
}

func injectToken(req *http.Request, placement TokenPlacement, token string) {
	switch placement.Kind {
	case PlacementHeader:
		req.Header.Set(placement.Name, placement.Prefix+token)
	case PlacementCookie:
		req.AddCookie(&http.Cookie{Name: placement.Name, Value: token})
	case PlacementQuery:
		q := req.URL.Query()
		q.Set(placement.Name, token)
		req.URL.RawQuery = q.Encode()
	}
}
