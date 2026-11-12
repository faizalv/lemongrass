// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package restergate

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/faizalv/lemongrass/internal/vault"
)

// credentialHeaders carry credentials and cannot be set on a single call.
var credentialHeaders = []string{"Authorization", "Proxy-Authorization", "Cookie"}

func setHeaders(req *http.Request, headers []vault.Header) {
	for _, h := range headers {
		req.Header.Set(h.Name, h.Value)
	}
}

// checkCallHeaders refuses a caller's header that is malformed, repeated, carries a credential, or is set by the proxy or the rester.
// ownAuth is for a human call with no user, where no token is sent and the human supplies the auth:
// the credential headers and the token placement's header are then allowed.
func checkCallHeaders(rester vault.ResolvedRester, headers []vault.Header, ownAuth bool) error {
	reserved := rester
	if ownAuth {
		reserved.TokenPlacement = vault.TokenPlacement{}
	}
	seen := make(map[string]bool, len(headers))
	for _, h := range headers {
		if err := vault.CheckHeader(h); err != nil {
			return fmt.Errorf("restergate: %w", err)
		}
		if reserved.ReservesHeader(h.Name) || (!ownAuth && isCredentialHeader(h.Name)) {
			return fmt.Errorf("restergate: header %q is set by the proxy and cannot be set on a call", h.Name)
		}
		if rester.SetsHeader(h.Name) {
			return fmt.Errorf("restergate: header %q is set by the rester and cannot be set on a call", h.Name)
		}
		key := http.CanonicalHeaderKey(h.Name)
		if seen[key] {
			return fmt.Errorf("restergate: header %q is given more than once", h.Name)
		}
		seen[key] = true
	}
	return nil
}

func isCredentialHeader(name string) bool {
	for _, c := range credentialHeaders {
		if strings.EqualFold(c, name) {
			return true
		}
	}
	return false
}
