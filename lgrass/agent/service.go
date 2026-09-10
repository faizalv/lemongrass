// Package agent maps short, model-facing channel ids to the vault's real channel ids and forwards query requests to the vault, holding no decryption power of its own.
package agent

import (
	"errors"
	"fmt"
	"sync"

	"github.com/faizalv/lemongrass/vault"
)

var ErrNoSuchChannel = errors.New("agent: no such channel")

// Service holds a vault.Client and the short-id-to-real-channel-id mapping, never a root secret or credential material.
type Service struct {
	vaultClient *vault.Client

	mu   sync.Mutex
	byID map[string]vault.ChannelID
}

func NewService(vaultClient *vault.Client) *Service {
	return &Service{
		vaultClient: vaultClient,
		byID:        make(map[string]vault.ChannelID),
	}
}

// RegisterChannel confirms realID exists at the vault and mints a short model-facing id mapped to it, taking no root secret of its own.
func (s *Service) RegisterChannel(realID vault.ChannelID) (string, error) {
	if _, err := s.vaultClient.ChannelScope(realID); err != nil {
		return "", fmt.Errorf("agent: registering channel %s: %w", realID, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		id, err := newShortID()
		if err != nil {
			return "", err
		}
		if _, exists := s.byID[id]; exists {
			continue
		}
		s.byID[id] = realID
		return id, nil
	}
}

// Query resolves shortID to its real vault channel and forwards the query, returning the same error for an unregistered, forgotten, or mistyped id.
func (s *Service) Query(shortID, table, operation string) ([]byte, error) {
	realID, ok := s.lookup(shortID)
	if !ok {
		return nil, ErrNoSuchChannel
	}
	return s.vaultClient.Query(realID, table, operation)
}

func (s *Service) lookup(shortID string) (vault.ChannelID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byID[shortID]
	return id, ok
}

// Forget drops shortID's mapping and succeeds even if shortID was never registered.
func (s *Service) Forget(shortID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byID, shortID)
}
