package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Service composes the vault primitives into the envelope-encryption channel lifecycle.
// A channel's own derived key wraps its own copy of a credential, separate from the
// root-encrypted base copy in creds. Service never holds a root secret as a field --
// every method that needs one takes it as a parameter and zeroes the derived root key
// before returning. Only a channel's own derived key, never the root key, is cached in
// memory between calls, and only for as long as that channel is active.
type Service struct {
	creds    *Store
	channels *Store
	metaDir  string
	rootSalt []byte

	mu         sync.Mutex
	activeKeys map[ChannelID][]byte
}

func NewService(dir string) (*Service, error) {
	creds, err := OpenStore(filepath.Join(dir, "creds"))
	if err != nil {
		return nil, err
	}
	channels, err := OpenStore(filepath.Join(dir, "channels"))
	if err != nil {
		return nil, err
	}
	metaDir := filepath.Join(dir, "meta")
	if err := os.MkdirAll(metaDir, 0o700); err != nil {
		return nil, fmt.Errorf("vault: creating %s: %w", metaDir, err)
	}
	rootSalt, err := loadOrCreateRootSalt(filepath.Join(dir, "root.salt"))
	if err != nil {
		return nil, err
	}
	return &Service{
		creds:      creds,
		channels:   channels,
		metaDir:    metaDir,
		rootSalt:   rootSalt,
		activeKeys: make(map[ChannelID][]byte),
	}, nil
}

func loadOrCreateRootSalt(path string) ([]byte, error) {
	salt, err := os.ReadFile(path)
	if err == nil {
		return salt, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("vault: reading %s: %w", path, err)
	}
	salt, err = NewSalt()
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, salt, 0o600); err != nil {
		return nil, fmt.Errorf("vault: writing %s: %w", path, err)
	}
	return salt, nil
}

// PutCredential encrypts value under the root key and stores it as dbName.
func (s *Service) PutCredential(rootSecret, dbName string, value []byte) error {
	rootKey, err := DeriveKey(rootSecret, s.rootSalt)
	if err != nil {
		return err
	}
	defer zero(rootKey)
	return s.creds.Put(dbName, rootKey, value)
}

// ListConnections returns every stored credential's name, no root secret needed -- like ListChannels, this reads names only, never a decrypted value.
func (s *Service) ListConnections() ([]string, error) {
	return s.creds.List()
}

// CreateChannel decrypts dbName's root-encrypted credential once and stores a separately-encrypted copy under a new channel id, wrapped with that channel's own derived key.
func (s *Service) CreateChannel(rootSecret, dbName string, scope Scope, ttl time.Duration) (Channel, error) {
	rootKey, err := DeriveKey(rootSecret, s.rootSalt)
	if err != nil {
		return Channel{}, err
	}
	defer zero(rootKey)

	plain, err := s.creds.Get(dbName, rootKey)
	if err != nil {
		return Channel{}, fmt.Errorf("vault: reading credential for %s: %w", dbName, err)
	}
	defer zero(plain)

	id, err := NewChannelID()
	if err != nil {
		return Channel{}, err
	}
	salt, err := NewSalt()
	if err != nil {
		return Channel{}, err
	}
	channelKey, err := DeriveKey(rootSecret, salt)
	if err != nil {
		return Channel{}, err
	}

	if err := s.channels.Put(string(id), channelKey, plain); err != nil {
		zero(channelKey)
		return Channel{}, fmt.Errorf("vault: wrapping credential for channel %s: %w", id, err)
	}

	now := time.Now()
	c := Channel{
		ID:        id,
		DBName:    dbName,
		Scope:     scope,
		Salt:      salt,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	if err := s.saveMeta(c); err != nil {
		zero(channelKey)
		s.channels.Delete(string(id))
		return Channel{}, err
	}

	s.mu.Lock()
	s.activeKeys[id] = channelKey
	s.mu.Unlock()
	return c, nil
}

// Activate re-derives an existing channel's key and refreshes its TTL, without decrypting the root-encrypted base credential again.
func (s *Service) Activate(rootSecret string, id ChannelID, ttl time.Duration) (Channel, error) {
	c, err := s.loadMeta(id)
	if err != nil {
		return Channel{}, err
	}
	channelKey, err := DeriveKey(rootSecret, c.Salt)
	if err != nil {
		return Channel{}, err
	}

	c.ExpiresAt = time.Now().Add(ttl)
	if err := s.saveMeta(c); err != nil {
		zero(channelKey)
		return Channel{}, err
	}

	s.mu.Lock()
	if old, ok := s.activeKeys[id]; ok {
		zero(old)
	}
	s.activeKeys[id] = channelKey
	s.mu.Unlock()
	return c, nil
}

// Query decrypts a channel's wrapped credential using its cached key and checks q against the channel's scope. It fails if the channel is expired, revoked, or was never activated.
func (s *Service) Query(id ChannelID, q Query) ([]byte, error) {
	c, err := s.loadMeta(id)
	if err != nil {
		return nil, err
	}
	if c.Expired(time.Now()) {
		return nil, fmt.Errorf("vault: channel %s has expired", id)
	}
	if err := c.Scope.Allow(q); err != nil {
		return nil, err
	}

	s.mu.Lock()
	channelKey, ok := s.activeKeys[id]
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("vault: channel %s is not active", id)
	}

	return s.channels.Get(string(id), channelKey)
}

// Revoke removes a channel's metadata, wrapped credential, and cached key. It succeeds even if the channel was never active.
func (s *Service) Revoke(id ChannelID) error {
	s.mu.Lock()
	if key, ok := s.activeKeys[id]; ok {
		zero(key)
		delete(s.activeKeys, id)
	}
	s.mu.Unlock()

	if err := s.channels.Delete(string(id)); err != nil {
		return err
	}
	return s.deleteMeta(id)
}

// ChannelScope returns a channel's metadata -- scope, TTL, db name -- without needing the channel to be active.
func (s *Service) ChannelScope(id ChannelID) (Channel, error) {
	return s.loadMeta(id)
}

// ListChannels reads every channel's metadata off disk -- there's no in-memory index, metaDir is the source of truth.
func (s *Service) ListChannels() ([]Channel, error) {
	entries, err := os.ReadDir(s.metaDir)
	if err != nil {
		return nil, fmt.Errorf("vault: reading %s: %w", s.metaDir, err)
	}
	channels := make([]Channel, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := ChannelID(e.Name()[:len(e.Name())-len(".json")])
		c, err := s.loadMeta(id)
		if err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, nil
}

func (s *Service) saveMeta(c Channel) error {
	b, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("vault: encoding metadata for channel %s: %w", c.ID, err)
	}
	return os.WriteFile(s.metaPath(c.ID), b, 0o600)
}

func (s *Service) loadMeta(id ChannelID) (Channel, error) {
	b, err := os.ReadFile(s.metaPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return Channel{}, ErrNotFound
	}
	if err != nil {
		return Channel{}, fmt.Errorf("vault: reading metadata for channel %s: %w", id, err)
	}
	var c Channel
	if err := json.Unmarshal(b, &c); err != nil {
		return Channel{}, fmt.Errorf("vault: decoding metadata for channel %s: %w", id, err)
	}
	return c, nil
}

func (s *Service) deleteMeta(id ChannelID) error {
	err := os.Remove(s.metaPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Service) metaPath(id ChannelID) string {
	return filepath.Join(s.metaDir, string(id)+".json")
}
