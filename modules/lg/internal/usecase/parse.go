package usecase

import (
	"fmt"
	"strings"

	reconentity "github.com/faizalv/lemongrass/modules/recon/entity"
)

func formatAnnotate(n reconentity.SemanticNode) string {
	desc := n.Description
	if desc == "" {
		desc = n.Signature
	}
	if desc == "" {
		desc = n.Kind
	}
	if n.Status == "stale" {
		desc = "[STALE] " + desc
	}
	calls := ""
	if len(n.Calls) > 0 {
		calls = ":[" + strings.Join(n.Calls, ",") + "]"
	}
	return fmt.Sprintf("%s:%s:%d-%d:\"%s\":%s%s",
		n.FilePath, n.Symbol, n.LineStart, n.LineEnd, desc, n.ReturnType, calls)
}

func parseRef(s string) (filePath, symbol, kind string, err error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) < 3 {
		err = fmt.Errorf("expected path:symbol:kind, got %q", s)
		return
	}
	filePath, symbol, kind = parts[0], parts[1], parts[2]
	return
}

func parseAnnotateFormat(s string) (filePath, symbol, kind, description, returnType string, calls []string, err error) {
	parts := strings.SplitN(s, ":", 4)
	if len(parts) < 4 {
		err = fmt.Errorf("invalid annotate format")
		return
	}
	filePath = parts[0]
	symbol = parts[1]
	kind = parts[2]

	rest := parts[3]
	if !strings.HasPrefix(rest, `"`) {
		err = fmt.Errorf("expected quoted description")
		return
	}
	rest = rest[1:]
	closeIdx := strings.Index(rest, `"`)
	if closeIdx < 0 {
		err = fmt.Errorf("unclosed description quote")
		return
	}
	description = rest[:closeIdx]
	rest = rest[closeIdx+1:]

	if !strings.HasPrefix(rest, ":") {
		return
	}
	rest = rest[1:]

	colonIdx := strings.LastIndex(rest, ":")
	if colonIdx >= 0 {
		rt := rest[:colonIdx]
		depsStr := rest[colonIdx+1:]
		if rt != "nil" {
			returnType = rt
		}
		if depsStr != "nil" {
			for _, d := range strings.Split(depsStr, ",") {
				if t := strings.TrimSpace(d); t != "" {
					calls = append(calls, t)
				}
			}
		}
	} else {
		if rest != "nil" {
			returnType = rest
		}
	}
	return
}
