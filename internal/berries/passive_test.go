// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package berries

import "testing"

func checkSet(fs []Finding) map[string]string {
	out := map[string]string{}
	for _, f := range fs {
		out[f.Check] = f.Detail
	}
	return out
}

func TestPassiveChecks(t *testing.T) {
	clean := map[string][]string{"Cache-Control": {"private, no-store"}, "X-Content-Type-Options": {"nosniff"}}
	if fs := passiveChecks(Response{Status: 200, Headers: clean, Body: invoice}, "GET", true); len(fs) != 0 {
		t.Fatalf("clean response flagged: %+v", fs)
	}

	bare := passiveChecks(Response{Status: 200, Headers: map[string][]string{}, Body: invoice}, "GET", true)
	got := checkSet(bare)
	if got[CheckNoStore] == "" || got[CheckNoSniff] == "" {
		t.Errorf("bare response: %v", got)
	}
	if fs := passiveChecks(Response{Status: 200, Headers: map[string][]string{}, Body: []any{}}, "GET", true); len(fs) != 0 {
		t.Errorf("empty body flagged for caching: %+v", fs)
	}

	cookies := map[string][]string{"Cache-Control": {"no-store"}, "X-Content-Type-Options": {"nosniff"},
		"Set-Cookie": {"sid=1; Path=/; HttpOnly; Secure", "pref=dark; Path=/"}}
	fs := passiveChecks(Response{Status: 200, Headers: cookies, Body: invoice}, "GET", true)
	if len(fs) != 1 || fs[0].Check != CheckCookieFlags || fs[0].discriminator != "pref" {
		t.Errorf("cookies: %+v", fs)
	}
	if fs := passiveChecks(Response{Status: 200, Headers: map[string][]string{"Set-Cookie": {"sid=1; HttpOnly"}}, Body: nil}, "GET", false); len(fs) != 0 {
		t.Errorf("Secure required on plain http: %+v", fs)
	}

	cors := map[string][]string{"Access-Control-Allow-Origin": {"*"}, "Access-Control-Allow-Credentials": {"true"}}
	if checkSet(passiveChecks(Response{Status: 204, Headers: cors}, "OPTIONS", true))[CheckCORSCredentials] == "" {
		t.Error("wildcard origin with credentials not flagged")
	}
}

func TestErrorDisclosureSignatures(t *testing.T) {
	bodies := map[string]any{
		"python": "Traceback (most recent call last):\n  File \"app.py\", line 3",
		"go":     "panic: boom\n\ngoroutine 1 [running]:\nmain.main()",
		"node":   map[string]any{"stack": "TypeError: x\n    at handler (/srv/app/routes/user.js:12:7)"},
		"java":   "java.lang.NullPointerException\n\tat com.shop.Api.get(Api.java:42)",
		"php":    "#0 /var/www/app/Http/Kernel.php(128): handle()",
		"sql":    map[string]any{"error": "SQLSTATE[42S02]: Base table or view not found"},
		"page":   "<html><head><title>Whoops! There was an error.</title>",
	}
	for name, body := range bodies {
		if checkSet(passiveChecks(Response{Status: 500, Body: body}, "GET", true))[CheckErrorDisclosure] == "" {
			t.Errorf("%s trace not flagged", name)
		}
	}
	for _, body := range []any{"at noon we ship", map[string]any{"note": "see line 3 of the invoice"}} {
		if fs := passiveChecks(Response{Status: 500, Body: body}, "GET", true); len(fs) != 0 {
			t.Errorf("plain text flagged: %#v -> %+v", body, fs)
		}
	}
}
