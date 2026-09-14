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

// Service never holds a root secret as a field, only a channel's own derived key cached in memory while that channel is active.
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

// ListConnections lists stored credential names only, never a decrypted value, without needing a root secret.
func (s *Service) ListConnections() ([]string, error) {
	return s.creds.List()
}

const (
	canaryEntryName = "verify"
	canaryPlaintext = "lemongrass"
)

var ErrWrongPassphrase = errors.New("vault: wrong passphrase")

// HasPassphrase reports whether a vault passphrase has ever been set, independent of whether any connection currently exists.
func (s *Service) HasPassphrase() (bool, error) {
	names, err := s.canary.List()
	if err != nil {
		return false, err
	}
	return len(names) > 0, nil
}

// SetPassphrase records rootSecret as the vault's passphrase, meaningful only the first time, so callers must check HasPassphrase first.
func (s *Service) SetPassphrase(rootSecret string) error {
	rootKey, err := DeriveKey(rootSecret, s.rootSalt)
	if err != nil {
		return err
	}
	defer zero(rootKey)
	return s.canary.Put(canaryEntryName, rootKey, []byte(canaryPlaintext))
}

// VerifyPassphrase checks rootSecret against the recorded canary, never touching a real connection, so a failure here always means the passphrase is wrong.
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

// TestConnection opens connString directly and pings it, with no root secret involved since the string never touches the credential store.
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
	return s.withDecryptedConnection(rootSecret, dbName, func(db *sql.DB, _ Engine) error {
		return pingWithTimeout(db)
	})
}

// ListTables decrypts dbName's stored credential and lists the tables in its database, for
// the Channels form's table picker.
func (s *Service) ListTables(rootSecret, dbName string) ([]string, error) {
	var tables []string
	err := s.withDecryptedConnection(rootSecret, dbName, func(db *sql.DB, engine Engine) error {
		found, err := listTables(db, engine)
		if err != nil {
			return err
		}
		tables = found
		return nil
	})
	return tables, err
}

// withDecryptedConnection decrypts dbName's credential, opens a connection with it, passes both to fn, and always closes the connection and zeroes the credential before returning.
func (s *Service) withDecryptedConnection(rootSecret, dbName string, fn func(*sql.DB, Engine) error) error {
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

	engine, err := engineOf(string(connString))
	if err != nil {
		return err
	}

	db, err := openDB(string(connString))
	if err != nil {
		return err
	}
	defer db.Close()

	return fn(db, engine)
}

// CreateChannel decrypts dbName's root-encrypted credential once and stores a separately-encrypted copy under a new channel id, wrapped with that channel's own derived key.
func (s *Service) CreateChannel(rootSecret, name, dbName string, scope Scope, ttl time.Duration) (Channel, error) {
	if name == "" {
		return Channel{}, ErrEmptyChannelName
	}

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
		Name:      name,
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

// Query classifies sqlText and checks it against the channel's scope before the caller's declared tables, since scope is the real security boundary.
func (s *Service) Query(id ChannelID, declaredTables []string, sqlText string) (QueryResult, error) {
	c, err := s.loadMeta(id)
	if err != nil {
		return QueryResult{}, err
	}
	if c.Expired(time.Now()) {
		s.closeDB(id)
		s.forgetKey(id)
		return QueryResult{}, fmt.Errorf("vault: channel %s has expired. Ask the channel owner to reactivate it or share a fresh channel id.", id)
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
	if err := c.Scope.AllowStatement(stmt); err != nil {
		return QueryResult{}, err
	}
	if err := checkDeclaredTables(stmt, declaredTables); err != nil {
		return QueryResult{}, err
	}

	db, err := s.getDB(id, string(connString))
	if err != nil {
		return QueryResult{}, err
	}
	result, err := runQuery(db, sqlText)
	if err != nil {
		return QueryResult{}, err
	}
	if stmt.ListsTables {
		result = c.Scope.FilterTableList(result)
	}
	return result, nil
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

// closeDB closes and evicts id's cached *sql.DB if any, best-effort since a close error here doesn't change the caller's own outcome.
func (s *Service) closeDB(id ChannelID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if db, ok := s.dbConns[id]; ok {
		db.Close()
		delete(s.dbConns, id)
	}
}

// Revoke removes a channel's metadata, wrapped credential, cached key, and cached db connection, succeeding even if the channel was never active.
func (s *Service) Revoke(id ChannelID) error {
	s.forgetKey(id)
	s.closeDB(id)

	if err := s.channels.Delete(string(id)); err != nil {
		return err
	}
	return s.deleteMeta(id)
}

// forgetKey releases id's cached channel key on revocation or TTL expiry, so a locked key's memory doesn't stay resident past the point it's usable.
func (s *Service) forgetKey(id ChannelID) {
	s.mu.Lock()
	if key, ok := s.activeKeys[id]; ok {
		lockedFree(key)
		delete(s.activeKeys, id)
	}
	s.mu.Unlock()
}

// ChannelScope returns a channel's metadata without needing the channel to be active.
func (s *Service) ChannelScope(id ChannelID) (Channel, error) {
	return s.loadMeta(id)
}

// ListChannels reads every channel's metadata off disk, since metaDir is the source of truth rather than an in-memory index.
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
