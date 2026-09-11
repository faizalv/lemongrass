//go:build linux

package vault

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// lockKey copies b into a locked, anonymous memory mapping the OS will never write to swap,
// then zeroes b. The caller must release the result with lockedFree once it's no longer needed.
func lockKey(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}
	locked, err := unix.Mmap(-1, 0, len(b), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANONYMOUS)
	if err != nil {
		return nil, fmt.Errorf("vault: mmap for locked key: %w", err)
	}
	if err := unix.Mlock(locked); err != nil {
		unix.Munmap(locked)
		return nil, fmt.Errorf("vault: mlock for locked key: %w", err)
	}
	// Best-effort: excludes the page from core dumps too. Not fatal if the kernel doesn't support it.
	unix.Madvise(locked, unix.MADV_DONTDUMP)
	copy(locked, b)
	zero(b)
	return locked, nil
}

// lockedFree zeroes and releases memory returned by lockKey.
func lockedFree(b []byte) {
	if len(b) == 0 {
		return
	}
	zero(b)
	unix.Munlock(b)
	unix.Munmap(b)
}
