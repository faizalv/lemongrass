//go:build !linux

package vault

// lockKey copies b into a fresh buffer and zeroes b. No OS-level memory locking on this platform.
func lockKey(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}
	locked := make([]byte, len(b))
	copy(locked, b)
	zero(b)
	return locked, nil
}

// lockedFree zeroes memory returned by lockKey.
func lockedFree(b []byte) {
	zero(b)
}
