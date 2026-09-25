package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// GetDomain decrypts name's stored domain config in full, passwords and tokens included, for a
// human editing it. It is never reachable from the agent or the CLI.
func (s *Service) GetDomain(rootSecret, name string) (Domain, error) {
	return s.getDomain(rootSecret, name)
}

// UpdateDomain replaces the stored domain name with d, then re-wraps d for every HTTP channel
// minted from that domain, active or not, and invalidates their cached state so the next call logs
// in with the new config. The existing domain is decrypted first, which fails on a wrong
// passphrase or an unknown name before anything is written. A channel that cannot be rewritten
// is named in the returned error; saving the same edit again retries it.
func (s *Service) UpdateDomain(rootSecret, name string, d Domain) error {
	if _, err := s.getDomain(rootSecret, name); err != nil {
		return err
	}
	if err := s.PutDomain(rootSecret, name, d); err != nil {
		return err
	}

	plain, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("vault: encoding domain %s: %w", name, err)
	}
	defer Zero(plain)

	channels, err := s.ListHTTPChannels()
	if err != nil {
		return err
	}
	var failed []string
	for _, c := range channels {
		if c.Domain != name {
			continue
		}
		if err := s.rewrapHTTPChannel(rootSecret, c, plain); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", c.ID, err))
			continue
		}
		s.invalidate(c.ID)
	}
	if len(failed) > 0 {
		return errors.New("vault: domain " + name + " saved, but these channels still hold the old config, save again to retry: " + strings.Join(failed, "; "))
	}
	return nil
}

func (s *Service) rewrapHTTPChannel(rootSecret string, c HTTPChannel, plain []byte) error {
	key, err := DeriveKey(rootSecret, c.Salt)
	if err != nil {
		return err
	}
	defer Zero(key)
	return s.channels.Put(string(c.ID), key, plain)
}
