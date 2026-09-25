//go:build !linux

package vault

import "net"

// VerifyPeerUID is a no-op on platforms without SO_PEERCRED.
func VerifyPeerUID(conn net.Conn) error {
	return nil
}

// VerifyPeerBinary is a no-op on platforms without SO_PEERCRED.
func VerifyPeerBinary(conn net.Conn) error {
	return nil
}
