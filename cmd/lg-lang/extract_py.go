package main

import (
	"strings"
)

func extractPython(gr *grammar, src []byte, relPath, _ string) []parsedNode {
	matches := gr.extract(src)
	pkg := dirPackage(relPath)
	var nodes []parsedNode

	for _, caps := range matches {
		node, hasNode := findCap(caps, "node")
		sym, hasSym := findCap(caps, "symbol")
		if !hasNode || !hasSym {
			continue
		}

		symbolText := sym.text(src)
		if symbolText == "" || isSingleUnderscore(symbolText) {
			continue
		}

		receiver, hasReceiver := findCap(caps, "receiver")
		receiverText := ""

		kind := pyNodeKind(node.nodeType, hasReceiver)

		if hasReceiver {
			receiverText = receiver.text(src)
		}

		sig := pySig(caps, src, node.nodeType, hasReceiver)

		nodes = append(nodes, parsedNode{
			FilePath:    relPath,
			Package:     pkg,
			Symbol:      symbolText,
			Kind:        kind,
			LineStart:   node.startLine,
			LineEnd:     node.endLine,
			Receiver:    receiverText,
			Signature:   sig,
			Exported:    true,
			DependsOn:   []string{},
			ContentHash: hashBytes(src[node.startByte:node.endByte]),
		})
	}
	return nodes
}

func pyNodeKind(nodeType string, isMethod bool) string {
	if nodeType == "class_definition" {
		return "class"
	}
	if isMethod {
		return "method"
	}
	return "func"
}

func pySig(caps []capture, src []byte, nodeType string, isMethod bool) string {
	if nodeType == "class_definition" {
		if sc, ok := findCap(caps, "superclasses"); ok {
			t := strings.TrimSpace(sc.text(src))
			t = strings.TrimPrefix(t, "(")
			t = strings.TrimSuffix(t, ")")
			return strings.TrimSpace(t)
		}
		return ""
	}

	params := ""
	if p, ok := findCap(caps, "params"); ok {
		params = pyFilterParams(p.text(src), isMethod)
	}
	ret := ""
	if r, ok := findCap(caps, "return_type"); ok {
		t := strings.TrimSpace(r.text(src))
		t = strings.TrimPrefix(t, "->")
		t = strings.TrimSpace(t)
		if t != "" {
			ret = " -> " + t
		}
	}
	return "(" + params + ")" + ret
}

func pyFilterParams(raw string, isMethod bool) string {
	raw = strings.TrimPrefix(raw, "(")
	raw = strings.TrimSuffix(raw, ")")
	parts := strings.Split(raw, ",")
	var out []string
	skipFirst := isMethod
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if skipFirst && (p == "self" || p == "cls") {
			skipFirst = false
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, ", ")
}

func isSingleUnderscore(name string) bool {
	return strings.HasPrefix(name, "_") && !strings.HasPrefix(name, "__")
}
