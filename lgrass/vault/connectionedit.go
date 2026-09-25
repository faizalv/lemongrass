package vault

import (
	"errors"
	"fmt"
	"strings"
)

// GetConnection decrypts dbName's stored connection string in full, password included, for a
// human editing it. It is never reachable from the agent or the CLI.
func (s *Service) GetConnection(rootSecret, dbName string) (string, error) {
	rootKey, err := DeriveKey(rootSecret, s.rootSalt)
	if err != nil {
		return "", err
	}
	defer zero(rootKey)

	b, err := s.creds.Get(dbName, rootKey)
	if err != nil {
		return "", fmt.Errorf("vault: reading credential for %s: %w", dbName, err)
	}
	defer zero(b)
	return string(b), nil
}

// UpdateConnection replaces the stored connection string dbName with connString, then re-wraps
// it for every db channel minted from that connection, active or not, and closes their cached
// database handles so the next query connects with the new value. The existing credential is
// decrypted first, which fails on a wrong passphrase or an unknown name before anything is
// written, and the engine cannot change because a channel's scope was built against it. A
// channel that cannot be rewritten is named in the returned error; saving the same edit again
// retries.
func (s *Service) UpdateConnection(rootSecret, dbName, connString string) error {
	old, err := s.GetConnection(rootSecret, dbName)
	if err != nil {
		return err
	}
	oldEngine, err := engineOf(old)
	if err != nil {
		return err
	}
	newEngine, err := engineOf(connString)
	if err != nil {
		return err
	}
	if oldEngine != newEngine {
		return fmt.Errorf("vault: connection %s is a %s connection, the engine cannot change", dbName, oldEngine)
	}

	plain := []byte(connString)
	defer zero(plain)
	if err := s.PutCredential(rootSecret, dbName, plain); err != nil {
		return err
	}

	channels, err := s.ListChannels()
	if err != nil {
		return err
	}
	var failed []string
	for _, c := range channels {
		if c.DBName != dbName {
			continue
		}
		if err := s.rewrapChannel(rootSecret, c, plain); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", c.ID, err))
			continue
		}
		s.closeDB(c.ID)
	}
	if len(failed) > 0 {
		return errors.New("vault: connection " + dbName + " saved, but these channels still hold the old credential, save again to retry: " + strings.Join(failed, "; "))
	}
	return nil
}

func (s *Service) rewrapChannel(rootSecret string, c Channel, plain []byte) error {
	key, err := DeriveKey(rootSecret, c.Salt)
	if err != nil {
		return err
	}
	defer zero(key)
	return s.channels.Put(string(c.ID), key, plain)
}
