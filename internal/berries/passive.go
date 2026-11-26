// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package berries

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// Check ids of the passive tier. Passive checks read responses a run already got and send nothing of their own.
const (
	CheckNoStore         = "passive.no-store"
	CheckNoSniff         = "passive.nosniff"
	CheckCookieFlags     = "passive.cookie-flags"
	CheckCORSCredentials = "passive.cors-credentials"
	CheckErrorDisclosure = "passive.error-disclosure"
)

var passiveCheckIDs = []string{CheckNoStore, CheckNoSniff, CheckCookieFlags, CheckCORSCredentials, CheckErrorDisclosure}

// errorSignatures are traces and debug pages a framework prints when an error reaches the client. Each is specific
// enough that ordinary content does not match it.
var errorSignatures = []struct {
	name string
	re   *regexp.Regexp
}{
	{"Python traceback", regexp.MustCompile(`Traceback \(most recent call last\)`)},
	{"Go panic trace", regexp.MustCompile(`goroutine \d+ \[running\]`)},
	{"Node stack trace", regexp.MustCompile(`\n\s+at [\w$.<>]+ \(?[\w./\\:-]+\.(?:js|mjs|cjs|ts):\d+:\d+\)?`)},
	{"Java stack trace", regexp.MustCompile(`\n\s+at [\w$.]+\([\w$]+\.java:\d+\)`)},
	{".NET exception", regexp.MustCompile(`System\.[\w.]*Exception:|   at [\w.]+\(.*\) in .+:line \d+`)},
	{"PHP stack trace", regexp.MustCompile(`#\d+ /[\w/.-]+\.php\(\d+\)`)},
	{"SQL error", regexp.MustCompile(`SQLSTATE\[|You have an error in your SQL syntax|ORA-\d{5}|PG::\w+Error`)},
	{"framework debug page", regexp.MustCompile(`<title>[^<]*(?:Whoops!|Werkzeug Debugger|DisallowedHost|Exception at /)`)},
}

// passiveChecks returns the passive findings one response shows. The caller fills the route and evidence.
func passiveChecks(resp Response, method string, https bool) []Finding {
	var out []Finding
	h := http.Header(resp.Headers)
	ok := is2xx(resp.Status)
	content := ok && method == "GET" && !Empty(resp.Body)

	if content && !headerHasToken(h.Values("Cache-Control"), "no-store") {
		out = append(out, Finding{Check: CheckNoStore, State: StateConfirmed,
			Detail: fmt.Sprintf("an authenticated %d response with content has no Cache-Control: no-store (got %q)", resp.Status, strings.Join(h.Values("Cache-Control"), ", "))})
	}
	if content && !strings.EqualFold(strings.TrimSpace(h.Get("X-Content-Type-Options")), "nosniff") {
		out = append(out, Finding{Check: CheckNoSniff, State: StateConfirmed,
			Detail: fmt.Sprintf("a %d response with content has no X-Content-Type-Options: nosniff", resp.Status)})
	}
	for _, c := range h.Values("Set-Cookie") {
		name, _, _ := strings.Cut(c, "=")
		attrs := strings.ToLower(c)
		var missing []string
		if !strings.Contains(attrs, "httponly") {
			missing = append(missing, "HttpOnly")
		}
		if https && !strings.Contains(attrs, "secure") {
			missing = append(missing, "Secure")
		}
		if len(missing) > 0 {
			name = strings.TrimSpace(name)
			out = append(out, Finding{Check: CheckCookieFlags, State: StateConfirmed, discriminator: name,
				Detail: fmt.Sprintf("cookie %s is set without %s", name, strings.Join(missing, " and "))})
		}
	}
	if strings.TrimSpace(h.Get("Access-Control-Allow-Origin")) == "*" && strings.EqualFold(strings.TrimSpace(h.Get("Access-Control-Allow-Credentials")), "true") {
		out = append(out, Finding{Check: CheckCORSCredentials, State: StateConfirmed,
			Detail: "Access-Control-Allow-Origin is * while Access-Control-Allow-Credentials is true"})
	}
	if name := errorSignature(resp.Body); name != "" {
		out = append(out, Finding{Check: CheckErrorDisclosure, State: StateConfirmed, discriminator: name,
			Detail: fmt.Sprintf("a %d response carries a %s", resp.Status, name)})
	}
	return out
}

func headerHasToken(values []string, token string) bool {
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

func errorSignature(body any) string {
	var text string
	switch b := body.(type) {
	case nil:
		return ""
	case string:
		text = b
	default:
		data, err := json.Marshal(b)
		if err != nil {
			return ""
		}
		// A JSON string escapes newlines, which the stack trace signatures expect to see raw.
		text = strings.ReplaceAll(string(data), `\n`, "\n")
	}
	for _, sig := range errorSignatures {
		if sig.re.MatchString(text) {
			return sig.name
		}
	}
	return ""
}
