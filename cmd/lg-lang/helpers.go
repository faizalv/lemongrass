package main

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

func hashLines(src []byte, start, end int) string {
	lines := strings.Split(string(src), "\n")
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return ""
	}
	block := strings.Join(lines[start-1:end], "\n")
	h := sha256.Sum256([]byte(block))
	return hex.EncodeToString(h[:])
}

func hashBytes(src []byte) string {
	h := sha256.Sum256(src)
	return hex.EncodeToString(h[:])
}

func dirPackage(relPath string) string {
	dir := filepath.ToSlash(filepath.Dir(relPath))
	if dir == "." {
		return ""
	}
	return dir
}
