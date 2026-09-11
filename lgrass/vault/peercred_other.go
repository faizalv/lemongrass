//go:build !linux

package vault

import "net"

// verifyPeerUID is a no-op on platforms without SO_PEERCRED.
func verifyPeerUID(conn net.Conn) error {
	return nil
}

// verifyPeerBinary is a no-op on platforms without SO_PEERCRED.
func verifyPeerBinary(conn net.Conn) error {
	return nil
}
