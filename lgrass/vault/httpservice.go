package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CreateHTTPChannel decrypts domainName's root-encrypted config once and stores a
// separately-encrypted copy under a new channel id, wrapped with that channel's own derived
// key -- the same pattern CreateChannel uses for a db credential.
func (s *Service) CreateHTTPChannel(rootSecret, name, domainName string, scope HTTPScope, ttl time.Duration) (HTTPChannel, error) {
	if name == "" {
		return HTTPChannel{}, ErrEmptyChannelName
	}

	domain, err := s.getDomain(rootSecret, domainName)
	if err != nil {
		return HTTPChannel{}, err
	}
	plain, err := json.Marshal(domain)
	if err != nil {
		return HTTPChannel{}, fmt.Errorf("vault: encoding domain %s: %w", domainName, err)
	}
	defer Zero(plain)

	id, err := NewChannelID()
	if err != nil {
		return HTTPChannel{}, err
	}
	salt, err := NewSalt()
	if err != nil {
		return HTTPChannel{}, err
	}
	channelKey, err := DeriveKey(rootSecret, salt)
	if err != nil {
		return HTTPChannel{}, err
	}
	locked, err := LockKey(channelKey)
	if err != nil {
		Zero(channelKey)
		return HTTPChannel{}, err
	}

	if err := s.channels.Put(string(id), locked, plain); err != nil {
		LockedFree(locked)
		return HTTPChannel{}, fmt.Errorf("vault: wrapping domain for channel %s: %w", id, err)
	}

	now := time.Now()
	c := HTTPChannel{
		ID:        id,
		Name:      name,
		Domain:    domainName,
		Scope:     scope,
		Salt:      salt,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	if err := s.saveHTTPMeta(c); err != nil {
		LockedFree(locked)
		s.channels.Delete(string(id))
		return HTTPChannel{}, err
	}

	s.mu.Lock()
	s.activeKeys[id] = locked
	s.mu.Unlock()
	return c, nil
}

// ActivateHTTP re-derives an existing HTTP channel's key and refreshes its TTL, without
// decrypting the root-encrypted domain config again.
func (s *Service) ActivateHTTP(rootSecret string, id ChannelID, ttl time.Duration) (HTTPChannel, error) {
	c, err := s.loadHTTPMeta(id)
	if err != nil {
		return HTTPChannel{}, err
	}
	channelKey, err := DeriveKey(rootSecret, c.Salt)
	if err != nil {
		return HTTPChannel{}, err
	}
	locked, err := LockKey(channelKey)
	if err != nil {
		Zero(channelKey)
		return HTTPChannel{}, err
	}

	c.ExpiresAt = time.Now().Add(ttl)
	if err := s.saveHTTPMeta(c); err != nil {
		LockedFree(locked)
		return HTTPChannel{}, err
	}

	s.mu.Lock()
	if old, ok := s.activeKeys[id]; ok {
		LockedFree(old)
	}
	s.activeKeys[id] = locked
	s.mu.Unlock()
	return c, nil
}

// RevokeHTTP removes an HTTP channel's metadata, wrapped domain config, and cached key,
// succeeding even if the channel was never active.
func (s *Service) RevokeHTTP(id ChannelID) error {
	s.forgetKey(id)

	if err := s.channels.Delete(string(id)); err != nil {
		return err
	}
	return s.deleteHTTPMeta(id)
}

// HTTPChannelScope returns an HTTP channel's metadata without needing the channel to be active.
func (s *Service) HTTPChannelScope(id ChannelID) (HTTPChannel, error) {
	return s.loadHTTPMeta(id)
}

// ListHTTPChannels reads every HTTP channel's metadata off disk, since httpMetaDir is the
// source of truth rather than an in-memory index.
func (s *Service) ListHTTPChannels() ([]HTTPChannel, error) {
	entries, err := os.ReadDir(s.httpMetaDir)
	if err != nil {
		return nil, fmt.Errorf("vault: reading %s: %w", s.httpMetaDir, err)
	}
	channels := make([]HTTPChannel, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := ChannelID(e.Name()[:len(e.Name())-len(".json")])
		c, err := s.loadHTTPMeta(id)
		if err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, nil
}

func (s *Service) saveHTTPMeta(c HTTPChannel) error {
	b, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("vault: encoding metadata for channel %s: %w", c.ID, err)
	}
	return os.WriteFile(s.httpMetaPath(c.ID), b, 0o600)
}

func (s *Service) loadHTTPMeta(id ChannelID) (HTTPChannel, error) {
	b, err := os.ReadFile(s.httpMetaPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return HTTPChannel{}, ErrNotFound
	}
	if err != nil {
		return HTTPChannel{}, fmt.Errorf("vault: reading metadata for channel %s: %w", id, err)
	}
	var c HTTPChannel
	if err := json.Unmarshal(b, &c); err != nil {
		return HTTPChannel{}, fmt.Errorf("vault: decoding metadata for channel %s: %w", id, err)
	}
	return c, nil
}

func (s *Service) deleteHTTPMeta(id ChannelID) error {
	err := os.Remove(s.httpMetaPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Service) httpMetaPath(id ChannelID) string {
	return filepath.Join(s.httpMetaDir, string(id)+".json")
}
