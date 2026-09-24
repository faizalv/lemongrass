package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
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
	Name             string         `json:"name"`
	Domain           string         `json:"domain"`
	Status           string         `json:"status"`
	ExpiresAt        time.Time      `json:"expires_at"`
	RemainingSeconds int64          `json:"remaining_seconds"`
	Methods          []string       `json:"methods"`
	Exclusions       []MethodPath   `json:"exclusions"`
	BaseURL          string         `json:"base_url,omitempty"`
	Users            []HTTPUserInfo `json:"users,omitempty"`
}

// HTTPChannelInfo reports channel id's name, domain, expiry, scope and, while the channel is
// active, its base URL and users.
func (s *Service) HTTPChannelInfo(id ChannelID) (HTTPChannelInfo, error) {
	c, err := s.loadHTTPMeta(id)
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
		info.Exclusions = []MethodPath{}
	}

	if c.Expired(now) {
		info.Status = HTTPStatusExpired
		return info, nil
	}
	info.RemainingSeconds = int64(c.ExpiresAt.Sub(now).Seconds())

	domain, err := s.channelDomain(id)
	if err != nil {
		info.Status = HTTPStatusInactive
		return info, nil
	}
	info.Status = HTTPStatusActive
	info.BaseURL = domain.BaseURL
	info.Users = userInfos(domain)
	return info, nil
}

func userInfos(domain Domain) []HTTPUserInfo {
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
func (s *Service) HTTPChannelUsers(id ChannelID) ([]HTTPUserInfo, error) {
	_, domain, err := s.liveHTTPChannel(id)
	if err != nil {
		return nil, err
	}
	return userInfos(domain), nil
}

// channelDomain decrypts channel id's own copy of its domain config, failing when the channel
// is not active.
func (s *Service) channelDomain(id ChannelID) (Domain, error) {
	s.mu.Lock()
	channelKey, ok := s.activeKeys[id]
	s.mu.Unlock()
	if !ok {
		return Domain{}, errors.New("vault: this channel is not active")
	}

	wrapped, err := s.channels.Get(string(id), channelKey)
	if err != nil {
		return Domain{}, err
	}
	defer zero(wrapped)

	var domain Domain
	if err := json.Unmarshal(wrapped, &domain); err != nil {
		return Domain{}, fmt.Errorf("vault: decoding this channel's domain: %w", err)
	}
	return domain, nil
}
