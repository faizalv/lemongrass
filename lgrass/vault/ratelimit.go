package vault

import (
	"errors"
	"sync"
	"time"
)

var ErrLockedOut = errors.New("vault: too many failed attempts, locked out")

// FailureLimiter locks out further attempts after too many failures within a window.
// It exists because the vault's unix socket is reachable by anything running as the
// same OS user, so the operations that take a root secret over that socket have no
// other rate limit standing between a wrong guess and the next one.
type FailureLimiter struct {
	maxFailures int
	window      time.Duration
	lockout     time.Duration

	mu          sync.Mutex
	failures    []time.Time
	lockedUntil time.Time
}

func NewFailureLimiter(maxFailures int, window, lockout time.Duration) *FailureLimiter {
	return &FailureLimiter{maxFailures: maxFailures, window: window, lockout: lockout}
}

func (f *FailureLimiter) Check() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if time.Now().Before(f.lockedUntil) {
		return ErrLockedOut
	}
	return nil
}

func (f *FailureLimiter) RecordFailure() {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-f.window)
	kept := f.failures[:0]
	for _, t := range f.failures {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	kept = append(kept, now)
	f.failures = kept

	if len(f.failures) >= f.maxFailures {
		f.lockedUntil = now.Add(f.lockout)
		f.failures = nil
	}
}

func (f *FailureLimiter) RecordSuccess() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures = nil
}
