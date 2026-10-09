package vault

import (
	"path"
	"strings"
	"time"
)

// channelLike is implemented by both Channel and HTTPChannel, letting shared machinery (audit
// stamping, an expiry sweep) work across both kinds without a type switch.
type channelLike interface {
	Kind() string
	Expired(now time.Time) bool
}

var (
	_ channelLike = Channel{}
	_ channelLike = HTTPChannel{}
)

// MethodPath pairs an HTTP method with a path pattern, matched together as one exclusion so a
// verb can be blocked on one endpoint without losing access to other verbs on that same path.
type MethodPath struct {
	Method      string
	PathPattern string
}

// HTTPScope is a method allow-list plus an opt-in exclusion list that overrides even a granted
// method, the REST analog of Scope's table allow-list.
type HTTPScope struct {
	Methods    []string
	Exclusions []MethodPath
}

// Allow reports whether method is granted and no exclusion matches method and requestPath
// together. PathPattern uses path.Match semantics, so "*" matches one path segment, not "/".
func (s HTTPScope) Allow(method, requestPath string) bool {
	if !containsMethod(s.Methods, method) {
		return false
	}
	for _, ex := range s.Exclusions {
		if !strings.EqualFold(ex.Method, method) {
			continue
		}
		if matched, err := path.Match(ex.PathPattern, requestPath); err == nil && matched {
			return false
		}
	}
	return true
}

func containsMethod(methods []string, method string) bool {
	for _, m := range methods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

// HTTPChannel is the HTTP-channel sibling of Channel: a genuinely separate type since a
// domain's many-users shape and method+exclusion scope don't fit Channel's
// DBName/Scope/Port shape.
type HTTPChannel struct {
	ID        ChannelID
	Name      string
	Domain    string
	Scope     HTTPScope
	Salt      []byte
	CreatedAt time.Time
	ExpiresAt time.Time
}

func (c HTTPChannel) Expired(now time.Time) bool {
	return !now.Before(c.ExpiresAt)
}

func (c HTTPChannel) Kind() string {
	return "http"
}
