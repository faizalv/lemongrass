package vault

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// TokenPlacementKind says which part of an outgoing request carries the token.
type TokenPlacementKind string

const (
	PlacementHeader TokenPlacementKind = "header"
	PlacementCookie TokenPlacementKind = "cookie"
	PlacementQuery  TokenPlacementKind = "query"
)

// TokenPlacement says where an obtained token is injected on an outgoing call. Prefix is only
// meaningful for PlacementHeader, e.g. Name "Authorization", Prefix "Bearer ".
type TokenPlacement struct {
	Kind   TokenPlacementKind
	Name   string
	Prefix string
}

// Domain is one HTTP backend's shape: how to log a user in, where the token lives in the
// response, how to learn its lifetime, and where to inject it -- one Store entry holding many
// users, unlike a db Connection's one name to one connection string.
type Domain struct {
	BaseURL       string
	LoginEndpoint string // empty when every user on this domain is bring-your-own-token
	TokenPath     string // JSON path into the login response, e.g. "access_token" or "data.token"

	// TTLOrigin is a JSON path into the login response (e.g. "expires_in"), the literal
	// "jwt-exp" to decode the token itself as a JWT and read its exp claim, or "" to fall
	// back to FixedTTLSeconds.
	TTLOrigin       string
	FixedTTLSeconds int64

	TokenPlacement TokenPlacement
	Users          []DomainUser
}

// DomainUser is a login user (Fields carries whatever LoginEndpoint needs, e.g.
// username/password) or a bring-your-own-token user (Token is pre-supplied and LoginEndpoint
// is never called for it) -- never both. Tags are free text a human sets to say what the user
// is for, and are the only part of a user a model is ever shown besides its name.
type DomainUser struct {
	Name   string
	Fields map[string]string
	Token  string
	Tags   []string
}

func (u DomainUser) IsBYOT() bool {
	return u.Token != ""
}

const domainKeyPrefix = "domains-"

func domainKey(name string) string {
	return domainKeyPrefix + name
}

// PutDomain encrypts d under the root key and stores it as name, alongside credentials in the
// same Store but under a distinct key prefix so domain and db connection names never collide.
func (s *Service) PutDomain(rootSecret, name string, d Domain) error {
	rootKey, err := DeriveKey(rootSecret, s.rootSalt)
	if err != nil {
		return err
	}
	defer zero(rootKey)

	b, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("vault: encoding domain %s: %w", name, err)
	}
	return s.creds.Put(domainKey(name), rootKey, b)
}

// ListDomains lists stored domain names only, never a decrypted value, without needing a root secret.
func (s *Service) ListDomains() ([]string, error) {
	names, err := s.creds.List()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if rest, ok := strings.CutPrefix(n, domainKeyPrefix); ok {
			out = append(out, rest)
		}
	}
	return out, nil
}

// DeleteDomain removes a stored domain. It succeeds even if name was never stored.
func (s *Service) DeleteDomain(name string) error {
	return s.creds.Delete(domainKey(name))
}

// getDomain decrypts name's stored domain config.
func (s *Service) getDomain(rootSecret, name string) (Domain, error) {
	rootKey, err := DeriveKey(rootSecret, s.rootSalt)
	if err != nil {
		return Domain{}, err
	}
	defer zero(rootKey)

	b, err := s.creds.Get(domainKey(name), rootKey)
	if err != nil {
		return Domain{}, fmt.Errorf("vault: reading domain %s: %w", name, err)
	}
	defer zero(b)

	var d Domain
	if err := json.Unmarshal(b, &d); err != nil {
		return Domain{}, fmt.Errorf("vault: decoding domain %s: %w", name, err)
	}
	return d, nil
}

// TestDomainLogin attempts a real login for user against domain's LoginEndpoint, or validates
// a bring-your-own-token user's pasted token, with no root secret and no stored state involved
// -- the same fresh-value check TestConnection does for a db connection string.
func (s *Service) TestDomainLogin(domain Domain, user DomainUser) error {
	if user.IsBYOT() {
		_, err := byotExpiry(domain, user)
		return err
	}
	_, _, err := loginHTTP(domain, user)
	return err
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// loginHTTP POSTs user.Fields as a JSON body to domain.LoginEndpoint and extracts the token and
// its computed expiry from the response.
func loginHTTP(domain Domain, user DomainUser) (token string, expiresAt time.Time, err error) {
	if domain.LoginEndpoint == "" {
		return "", time.Time{}, errors.New("vault: domain has no login endpoint, only bring-your-own-token users are supported")
	}
	body, err := json.Marshal(user.Fields)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("vault: encoding login body: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, domain.BaseURL+domain.LoginEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("vault: building login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := *httpClient
	client.CheckRedirect = sameHostRedirectsOnly
	resp, err := client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("vault: logging in: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxLoginResponseBytes))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("vault: reading login response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("vault: login returned status %d", resp.StatusCode)
	}

	token, err = jsonPathString(respBody, domain.TokenPath)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("vault: extracting token: %w", err)
	}

	expiresAt, err = ttlFromLoginResponse(domain, respBody, token)
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// byotExpiry computes a bring-your-own-token user's expiry: from the token itself when
// TTLOrigin is "jwt-exp", otherwise from FixedTTLSeconds, since there's no login response to
// pull a JSON-path TTL from.
func byotExpiry(domain Domain, user DomainUser) (time.Time, error) {
	if user.Token == "" {
		return time.Time{}, errors.New("vault: bring-your-own-token user has no token")
	}
	switch domain.TTLOrigin {
	case "jwt-exp":
		return jwtExpiry(user.Token)
	case "":
		return time.Now().Add(time.Duration(domain.FixedTTLSeconds) * time.Second), nil
	default:
		return time.Time{}, fmt.Errorf("vault: TTLOrigin %q needs a login response, which a bring-your-own-token user never has -- use \"jwt-exp\" or a fixed TTL", domain.TTLOrigin)
	}
}

// ttlFromLoginResponse computes a login user's expiry from TTLOrigin: a JSON path into
// respBody, the literal "jwt-exp" to decode token itself, or FixedTTLSeconds when TTLOrigin is empty.
func ttlFromLoginResponse(domain Domain, respBody []byte, token string) (time.Time, error) {
	switch domain.TTLOrigin {
	case "":
		return time.Now().Add(time.Duration(domain.FixedTTLSeconds) * time.Second), nil
	case "jwt-exp":
		return jwtExpiry(token)
	default:
		raw, err := jsonPathString(respBody, domain.TTLOrigin)
		if err != nil {
			return time.Time{}, fmt.Errorf("vault: extracting TTL: %w", err)
		}
		seconds, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("vault: TTLOrigin %q did not yield a number: %w", domain.TTLOrigin, err)
		}
		return time.Now().Add(time.Duration(seconds * float64(time.Second))), nil
	}
}

// jwtExpiry decodes a JWT's payload segment (no signature verification -- the vault trusts the
// issuing domain it just logged into, not the token as an untrusted credential) and reads its
// numeric exp claim.
func jwtExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("vault: token is not a JWT (expected 3 dot-separated parts, got %d)", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("vault: decoding JWT payload: %w", err)
	}
	var claims struct {
		Exp float64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, fmt.Errorf("vault: decoding JWT claims: %w", err)
	}
	if claims.Exp == 0 {
		return time.Time{}, errors.New("vault: JWT has no exp claim")
	}
	return time.Unix(int64(claims.Exp), 0), nil
}

// jsonPathString reads a dot-separated path (e.g. "data.token") out of a JSON object and
// renders the leaf as a string, whatever its JSON type.
func jsonPathString(data []byte, path string) (string, error) {
	var tree any
	if err := json.Unmarshal(data, &tree); err != nil {
		return "", fmt.Errorf("vault: decoding JSON: %w", err)
	}
	node := tree
	for _, key := range strings.Split(path, ".") {
		m, ok := node.(map[string]any)
		if !ok {
			return "", fmt.Errorf("vault: path %q: %q is not an object", path, key)
		}
		val, ok := m[key]
		if !ok {
			return "", fmt.Errorf("vault: path %q: %q not found", path, key)
		}
		node = val
	}
	switch v := node.(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("vault: path %q resolved to an unsupported type %T", path, node)
	}
}

const maxLoginResponseBytes = 1 << 20 // 1MiB, generous for a login response
