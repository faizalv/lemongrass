package session

import (
	"crypto/rand"
	"database/sql"
	"errors"
)

const (
	threadIDLength   = 6
	threadIDAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	threadIDAttempts = 20
)

type queryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

// A random 6 character id that no thread in the store uses yet.
func newThreadID(q queryer) (string, error) {
	for i := 0; i < threadIDAttempts; i++ {
		buf := make([]byte, threadIDLength)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for j, b := range buf {
			buf[j] = threadIDAlphabet[int(b)%len(threadIDAlphabet)]
		}
		id := string(buf)
		var one int
		err := q.QueryRow(`SELECT 1 FROM lg_threads WHERE id = ?`, id).Scan(&one)
		if err == sql.ErrNoRows {
			return id, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("session: no free thread id found")
}
