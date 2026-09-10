package vault

// zero overwrites b with zero bytes; not a guarantee, since the runtime may have already copied it elsewhere.
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
