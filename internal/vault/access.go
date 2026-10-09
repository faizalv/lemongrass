package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrChannelExpired  = errors.New("vault: this channel has expired. Ask the channel owner to reactivate it or share a fresh channel id.")
	ErrChannelInactive = errors.New("vault: this channel is not active")
)

// OpenChannel returns an active db channel's metadata and its decrypted connection string. The caller must Zero the returned bytes. An expired channel has its cached key released and fails with ErrChannelExpired.
func (s *Service) OpenChannel(id ChannelID) (Channel, []byte, error) {
	c, err := s.loadMeta(id)
	if err != nil {
		return Channel{}, nil, err
	}
	if c.Expired(time.Now()) {
		s.forgetKey(id)
		return Channel{}, nil, ErrChannelExpired
	}
	secret, err := s.channelSecret(id)
	if err != nil {
		return Channel{}, nil, err
	}
	return c, secret, nil
}

// OpenHTTPChannel returns an active HTTP channel's metadata and its decrypted domain config. An expired channel has its cached key released and fails with ErrChannelExpired.
func (s *Service) OpenHTTPChannel(id ChannelID) (HTTPChannel, Domain, error) {
	c, err := s.loadHTTPMeta(id)
	if err != nil {
		return HTTPChannel{}, Domain{}, err
	}
	if c.Expired(time.Now()) {
		s.forgetKey(id)
		return HTTPChannel{}, Domain{}, ErrChannelExpired
	}
	domain, err := s.ChannelDomain(id)
	if err != nil {
		return HTTPChannel{}, Domain{}, err
	}
	return c, domain, nil
}

// ChannelDomain decrypts channel id's own copy of its domain config, failing when the channel is not active.
func (s *Service) ChannelDomain(id ChannelID) (Domain, error) {
	wrapped, err := s.channelSecret(id)
	if err != nil {
		return Domain{}, err
	}
	defer Zero(wrapped)

	var domain Domain
	if err := json.Unmarshal(wrapped, &domain); err != nil {
		return Domain{}, fmt.Errorf("vault: decoding this channel's domain: %w", err)
	}
	return domain, nil
}

// WithConnection decrypts dbName's stored connection string under rootSecret, passes it to fn, and zeroes it before returning.
func (s *Service) WithConnection(rootSecret, dbName string, fn func(connString []byte) error) error {
	rootKey, err := DeriveKey(rootSecret, s.rootSalt)
	if err != nil {
		return err
	}
	defer Zero(rootKey)

	connString, err := s.creds.Get(dbName, rootKey)
	if err != nil {
		return fmt.Errorf("vault: reading credential for %s: %w", dbName, err)
	}
	defer Zero(connString)
	return fn(connString)
}

// OnInvalidate registers fn to run whenever a channel's cached state must be dropped: revocation, expiry, or a rewrap after an edit.
func (s *Service) OnInvalidate(fn func(ChannelID)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onInvalid = append(s.onInvalid, fn)
}

func (s *Service) invalidate(id ChannelID) {
	s.mu.Lock()
	hooks := append([](func(ChannelID)){}, s.onInvalid...)
	s.mu.Unlock()
	for _, fn := range hooks {
		fn(id)
	}
}

func (s *Service) channelSecret(id ChannelID) ([]byte, error) {
	s.mu.Lock()
	channelKey, ok := s.activeKeys[id]
	s.mu.Unlock()
	if !ok {
		return nil, ErrChannelInactive
	}
	return s.channels.Get(string(id), channelKey)
}
