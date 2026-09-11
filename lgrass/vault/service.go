package vault

import (
	"database/sql"
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
	canary   *Store
	metaDir  string
	rootSalt []byte

	mu         sync.Mutex
	activeKeys map[ChannelID][]byte
	dbConns    map[ChannelID]*sql.DB
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
	canary, err := OpenStore(filepath.Join(dir, "canary"))
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
		canary:     canary,
		metaDir:    metaDir,
		rootSalt:   rootSalt,
		activeKeys: make(map[ChannelID][]byte),
		dbConns:    make(map[ChannelID]*sql.DB),
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

const (
	canaryEntryName = "verify"
	canaryPlaintext = "lemongrass"
)

var ErrWrongPassphrase = errors.New("vault: wrong passphrase")

// HasPassphrase reports whether a vault passphrase has ever been set. Independent of whether
// any connection currently exists, so an emptied-out vault still reads as already set up.
func (s *Service) HasPassphrase() (bool, error) {
	names, err := s.canary.List()
	if err != nil {
		return false, err
	}
	return len(names) > 0, nil
}

// SetPassphrase records rootSecret as the vault's passphrase, for VerifyPassphrase to check
// against later. Only meaningful the first time -- callers must check HasPassphrase first.
func (s *Service) SetPassphrase(rootSecret string) error {
	rootKey, err := DeriveKey(rootSecret, s.rootSalt)
	if err != nil {
		return err
	}
	defer zero(rootKey)
	return s.canary.Put(canaryEntryName, rootKey, []byte(canaryPlaintext))
}

// VerifyPassphrase checks rootSecret against the recorded canary. Decrypting the canary
// never touches a real connection, so a failure here can only mean the passphrase is wrong,
// not that some database happens to be unreachable.
func (s *Service) VerifyPassphrase(rootSecret string) error {
	rootKey, err := DeriveKey(rootSecret, s.rootSalt)
	if err != nil {
		return err
	}
	defer zero(rootKey)
	plain, err := s.canary.Get(canaryEntryName, rootKey)
	if err != nil {
		return ErrWrongPassphrase
	}
	defer zero(plain)
	if string(plain) != canaryPlaintext {
		return ErrWrongPassphrase
	}
	return nil
}

// ResetVault permanently discards the canary and every stored credential and channel. The
// only way back from a forgotten passphrase, since nothing encrypted under it is recoverable
// without it.
func (s *Service) ResetVault() error {
	channels, err := s.ListChannels()
	if err != nil {
		return err
	}
	for _, c := range channels {
		if err := s.Revoke(c.ID); err != nil {
			return err
		}
	}
	names, err := s.creds.List()
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := s.creds.Delete(name); err != nil {
			return err
		}
	}
	return s.canary.Delete(canaryEntryName)
}

// DeleteConnection removes a stored credential. It succeeds even if dbName was never stored.
func (s *Service) DeleteConnection(dbName string) error {
	return s.creds.Delete(dbName)
}

// TestConnection opens connString directly and pings it. No root secret involved -- the string
// is fresh out of a form and never touches the credential store, so nothing here needs
// decrypting.
func (s *Service) TestConnection(connString string) error {
	db, err := openDB(connString)
	if err != nil {
		return err
	}
	defer db.Close()
	return pingWithTimeout(db)
}

// TestConnectionSaved decrypts dbName's stored credential and pings it, to check an
// already-saved connection still works without creating a channel against it.
func (s *Service) TestConnectionSaved(rootSecret, dbName string) error {
	return s.withDecryptedConnection(rootSecret, dbName, pingWithTimeout)
}

// withDecryptedConnection decrypts dbName's stored credential under rootSecret, opens a
// connection with it, and passes that connection to fn. The connection is always closed and
// the credential always zeroed before returning, regardless of fn's outcome.
func (s *Service) withDecryptedConnection(rootSecret, dbName string, fn func(*sql.DB) error) error {
	rootKey, err := DeriveKey(rootSecret, s.rootSalt)
	if err != nil {
		return err
	}
	defer zero(rootKey)

	connString, err := s.creds.Get(dbName, rootKey)
	if err != nil {
		return fmt.Errorf("vault: reading credential for %s: %w", dbName, err)
	}
	defer zero(connString)

	db, err := openDB(string(connString))
	if err != nil {
		return err
	}
	defer db.Close()

	return fn(db)
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
	locked, err := lockKey(channelKey)
	if err != nil {
		zero(channelKey)
		return Channel{}, err
	}

	if err := s.channels.Put(string(id), locked, plain); err != nil {
		lockedFree(locked)
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
		lockedFree(locked)
		s.channels.Delete(string(id))
		return Channel{}, err
	}

	s.mu.Lock()
	s.activeKeys[id] = locked
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
	locked, err := lockKey(channelKey)
	if err != nil {
		zero(channelKey)
		return Channel{}, err
	}

	c.ExpiresAt = time.Now().Add(ttl)
	if err := s.saveMeta(c); err != nil {
		lockedFree(locked)
		return Channel{}, err
	}

	s.mu.Lock()
	if old, ok := s.activeKeys[id]; ok {
		lockedFree(old)
	}
	s.activeKeys[id] = locked
	s.mu.Unlock()
	return c, nil
}

// Query classifies sqlText for real, checks it against declaredTables and the channel's scope,
// and -- only once both pass -- runs it against the channel's own database. The credential
// itself never leaves this method: it's decrypted only to pick a driver and open/reuse a
// connection, then zeroed. It fails if the channel is expired, revoked, or was never activated.
func (s *Service) Query(id ChannelID, declaredTables []string, sqlText string) (QueryResult, error) {
	c, err := s.loadMeta(id)
	if err != nil {
		return QueryResult{}, err
	}
	if c.Expired(time.Now()) {
		s.closeDB(id)
		s.forgetKey(id)
		return QueryResult{}, fmt.Errorf("vault: channel %s has expired", id)
	}

	s.mu.Lock()
	channelKey, ok := s.activeKeys[id]
	s.mu.Unlock()
	if !ok {
		return QueryResult{}, fmt.Errorf("vault: channel %s is not active", id)
	}

	connString, err := s.channels.Get(string(id), channelKey)
	if err != nil {
		return QueryResult{}, err
	}
	defer zero(connString)

	engine, err := engineOf(string(connString))
	if err != nil {
		return QueryResult{}, err
	}
	stmt, err := ClassifyQuery(engine, sqlText)
	if err != nil {
		return QueryResult{}, err
	}
	if err := checkDeclaredTables(stmt, declaredTables); err != nil {
		return QueryResult{}, err
	}
	if err := c.Scope.AllowStatement(stmt); err != nil {
		return QueryResult{}, err
	}

	db, err := s.getDB(id, string(connString))
	if err != nil {
		return QueryResult{}, err
	}
	return runQuery(db, sqlText)
}

// getDB returns id's cached *sql.DB, opening and caching one on first use.
func (s *Service) getDB(id ChannelID, connString string) (*sql.DB, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if db, ok := s.dbConns[id]; ok {
		return db, nil
	}
	db, err := openDB(connString)
	if err != nil {
		return nil, err
	}
	s.dbConns[id] = db
	return db, nil
}

// closeDB closes and evicts id's cached *sql.DB, if any. Best-effort: a close error here
// doesn't change the caller's own outcome, which already has its own error to report.
func (s *Service) closeDB(id ChannelID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if db, ok := s.dbConns[id]; ok {
		db.Close()
		delete(s.dbConns, id)
	}
}

// Revoke removes a channel's metadata, wrapped credential, cached key, and cached db connection. It succeeds even if the channel was never active.
func (s *Service) Revoke(id ChannelID) error {
	s.forgetKey(id)
	s.closeDB(id)

	if err := s.channels.Delete(string(id)); err != nil {
		return err
	}
	return s.deleteMeta(id)
}

// forgetKey releases and forgets id's cached channel key, if any -- used on revocation and on
// TTL expiry, so a locked key's memory doesn't stay resident past the point it's usable.
func (s *Service) forgetKey(id ChannelID) {
	s.mu.Lock()
	if key, ok := s.activeKeys[id]; ok {
		lockedFree(key)
		delete(s.activeKeys, id)
	}
	s.mu.Unlock()
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
