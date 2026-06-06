package main

import (
	"path/filepath"
	"strings"
)

func extractPHP(gr *grammar, src []byte, relPath, _ string) []parsedNode {
	if strings.HasSuffix(strings.ToLower(relPath), ".blade.php") {
		lines := strings.Count(string(src), "\n") + 1
		name := filepath.Base(strings.TrimSuffix(strings.TrimSuffix(relPath, ".php"), ".blade"))
		return []parsedNode{{
			FilePath:    relPath,
			Package:     dirPackage(relPath),
			Symbol:      name,
			Kind:        "blade",
			LineStart:   1,
			LineEnd:     lines,
			Exported:    false,
			DependsOn:   []string{},
			ContentHash: hashBytes(src),
		}}
	}

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
		if symbolText == "" {
			continue
		}

		receiver, hasReceiver := findCap(caps, "receiver")
		kind := phpKind(node.nodeType, hasReceiver)

		receiverText := ""
		if hasReceiver {
			receiverText = receiver.text(src)
		}

		sig := phpSig(caps, src, node.nodeType)

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

func phpKind(nodeType string, isMethod bool) string {
	if isMethod {
		return "method"
	}
	switch nodeType {
	case "function_definition":
		return "func"
	case "class_declaration":
		return "class"
	case "interface_declaration":
		return "interface"
	case "trait_declaration":
		return "trait"
	case "enum_declaration":
		return "enum"
	}
	return "func"
}

func phpSig(caps []capture, src []byte, nodeType string) string {
	switch nodeType {
	case "function_definition", "method_declaration":
		params := ""
		if p, ok := findCap(caps, "params"); ok {
			params = p.text(src)
		}
		ret := ""
		if r, ok := findCap(caps, "return_type"); ok {
			t := strings.TrimSpace(r.text(src))
			t = strings.TrimPrefix(t, ":")
			t = strings.TrimSpace(t)
			if t != "" {
				ret = ": " + t
			}
		}
		return params + ret

	case "class_declaration":
		return phpClassSig(caps, src, "")

	case "interface_declaration":
		if ext, ok := findCap(caps, "extends"); ok {
			return strings.TrimSpace(ext.text(src))
		}
		return ""

	case "enum_declaration":
		if t, ok := findCap(caps, "enum_type"); ok {
			return ": " + strings.TrimSpace(t.text(src))
		}
		return ""
	}
	return ""
}

func phpClassSig(caps []capture, src []byte, prefix string) string {
	var parts []string
	if ext, ok := findCap(caps, "extends"); ok {
		parts = append(parts, strings.TrimSpace(ext.text(src)))
	}
	if impl, ok := findCap(caps, "implements"); ok {
		parts = append(parts, strings.TrimSpace(impl.text(src)))
	}
	return strings.TrimSpace(prefix + strings.Join(parts, " "))
}
