package restergate

import (
	"time"

	"github.com/faizalv/lemongrass/vault"
)

const (
	HTTPStatusActive   = "active"
	HTTPStatusInactive = "inactive"
	HTTPStatusExpired  = "expired"
)

// HTTPUserInfo is what a model may learn about a domain user: a name and its tags, never a
// login field or a token.
type HTTPUserInfo struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// HTTPChannelInfo is a channel's model-facing summary. BaseURL and Users need the channel's
// decrypted domain copy, so they are only present while the channel is active.
type HTTPChannelInfo struct {
	Name             string             `json:"name"`
	Domain           string             `json:"domain"`
	Status           string             `json:"status"`
	ExpiresAt        time.Time          `json:"expires_at"`
	RemainingSeconds int64              `json:"remaining_seconds"`
	Methods          []string           `json:"methods"`
	Exclusions       []vault.MethodPath `json:"exclusions"`
	BaseURL          string             `json:"base_url,omitempty"`
	Users            []HTTPUserInfo     `json:"users,omitempty"`
}

// HTTPChannelInfo reports channel id's name, domain, expiry, scope and, while the channel is
// active, its base URL and users.
func (g *Gate) HTTPChannelInfo(id vault.ChannelID) (HTTPChannelInfo, error) {
	c, err := g.vault.HTTPChannelScope(id)
	if err != nil {
		return HTTPChannelInfo{}, err
	}

	now := time.Now()
	info := HTTPChannelInfo{
		Name:       c.Name,
		Domain:     c.Domain,
		ExpiresAt:  c.ExpiresAt,
		Methods:    c.Scope.Methods,
		Exclusions: c.Scope.Exclusions,
	}
	if info.Methods == nil {
		info.Methods = []string{}
	}
	if info.Exclusions == nil {
		info.Exclusions = []vault.MethodPath{}
	}

	if c.Expired(now) {
		info.Status = HTTPStatusExpired
		return info, nil
	}
	info.RemainingSeconds = int64(c.ExpiresAt.Sub(now).Seconds())

	domain, err := g.vault.ChannelDomain(id)
	if err != nil {
		info.Status = HTTPStatusInactive
		return info, nil
	}
	info.Status = HTTPStatusActive
	info.BaseURL = domain.BaseURL
	info.Users = userInfos(domain)
	return info, nil
}

func userInfos(domain vault.Domain) []HTTPUserInfo {
	out := make([]HTTPUserInfo, len(domain.Users))
	for i, u := range domain.Users {
		tags := u.Tags
		if tags == nil {
			tags = []string{}
		}
		out[i] = HTTPUserInfo{Name: u.Name, Tags: tags}
	}
	return out
}

// HTTPChannelUsers lists the users of channel id's domain with their tags, under the same
// expiry and active checks as RequestHTTP.
func (g *Gate) HTTPChannelUsers(id vault.ChannelID) ([]HTTPUserInfo, error) {
	_, domain, err := g.vault.OpenHTTPChannel(id)
	if err != nil {
		return nil, err
	}
	return userInfos(domain), nil
}
