// Package dbgate runs database work for a channel: it classifies a statement, enforces the channel's scope, pools connections and executes. Credentials come from the vault and never persist here.
package dbgate

import (
	"database/sql"
	"errors"
	"sync"

	"github.com/faizalv/lemongrass/vault"
)

type Gate struct {
	vault *vault.Service

	mu    sync.Mutex
	conns map[vault.ChannelID]*sql.DB
}

// New returns a Gate reading credentials from svc and dropping a channel's pooled connection whenever the vault invalidates that channel.
func New(svc *vault.Service) *Gate {
	g := &Gate{vault: svc, conns: make(map[vault.ChannelID]*sql.DB)}
	svc.OnInvalidate(g.closeDB)
	return g
}

// TestConnection opens connString directly and pings it, with no root secret involved since the string never touches the credential store.
func (g *Gate) TestConnection(connString string) error {
	db, err := openDB(connString)
	if err != nil {
		return err
	}
	defer db.Close()
	return pingWithTimeout(db)
}

// TestConnectionSaved decrypts dbName's stored credential and pings it, to check an
// already-saved connection still works without creating a channel against it.
func (g *Gate) TestConnectionSaved(rootSecret, dbName string) error {
	return g.withConnection(rootSecret, dbName, func(db *sql.DB, _ Engine) error {
		return pingWithTimeout(db)
	})
}

// ListTables decrypts dbName's stored credential and lists the tables in its database, for
// the Channels form's table picker.
func (g *Gate) ListTables(rootSecret, dbName string) ([]string, error) {
	var tables []string
	err := g.withConnection(rootSecret, dbName, func(db *sql.DB, engine Engine) error {
		found, err := listTables(db, engine)
		if err != nil {
			return err
		}
		tables = found
		return nil
	})
	return tables, err
}

// UpdateConnection replaces the stored connection string dbName with connString and re-wraps it
// for every channel minted from it. The engine cannot change because a channel's scope was built
// against it.
func (g *Gate) UpdateConnection(rootSecret, dbName, connString string) error {
	return g.vault.ReplaceConnection(rootSecret, dbName, connString, func(old string) error {
		oldEngine, err := engineOf(old)
		if err != nil {
			return err
		}
		newEngine, err := engineOf(connString)
		if err != nil {
			return err
		}
		if oldEngine != newEngine {
			return errors.New("dbgate: connection " + dbName + " is a " + string(oldEngine) + " connection, the engine cannot change")
		}
		return nil
	})
}

// withConnection decrypts dbName's credential, opens a connection with it, passes both to fn, and always closes the connection before returning.
func (g *Gate) withConnection(rootSecret, dbName string, fn func(*sql.DB, Engine) error) error {
	return g.vault.WithConnection(rootSecret, dbName, func(connString []byte) error {
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
	})
}

// Query classifies sqlText and checks it against the channel's scope before the caller's declared tables, since scope is the real security boundary.
func (g *Gate) Query(id vault.ChannelID, declaredTables []string, sqlText string) (QueryResult, error) {
	c, connBytes, err := g.vault.OpenChannel(id)
	if err != nil {
		return QueryResult{}, err
	}
	defer vault.Zero(connBytes)
	connString := string(connBytes)

	engine, err := engineOf(connString)
	if err != nil {
		return QueryResult{}, err
	}
	stmt, err := ClassifyQuery(engine, sqlText)
	if err != nil {
		return QueryResult{}, err
	}
	if err := AllowStatement(c.Scope, stmt); err != nil {
		return QueryResult{}, err
	}
	if err := checkDeclaredTables(stmt, declaredTables); err != nil {
		return QueryResult{}, err
	}

	db, err := g.getDB(id, connString)
	if err != nil {
		return QueryResult{}, errors.New("dbgate: the stored connection could not be opened")
	}
	result, err := runQuery(db, sqlText)
	if err != nil {
		return QueryResult{}, err
	}
	if stmt.ListsTables {
		result = FilterTableList(c.Scope, result)
	}
	return result, nil
}

// getDB returns id's cached *sql.DB, opening and caching one on first use.
func (g *Gate) getDB(id vault.ChannelID, connString string) (*sql.DB, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if db, ok := g.conns[id]; ok {
		return db, nil
	}
	db, err := openDB(connString)
	if err != nil {
		return nil, err
	}
	g.conns[id] = db
	return db, nil
}

// closeDB closes and evicts id's cached *sql.DB if any, best-effort since a close error here doesn't change the caller's own outcome.
func (g *Gate) closeDB(id vault.ChannelID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if db, ok := g.conns[id]; ok {
		db.Close()
		delete(g.conns, id)
	}
}
