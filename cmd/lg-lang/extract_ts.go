package main

import (
	"bytes"
	"path/filepath"
	"strings"
)

func extractTS(gr *grammar, src []byte, relPath, _ string, _ string) []parsedNode {
	matches := gr.extract(src)
	pkg := dirPackage(relPath)
	var nodes []parsedNode

	for _, caps := range matches {
		// Regular TS files: only exported symbols
		if _, ok := findCap(caps, "export"); !ok {
			continue
		}
		if n := tsBuildNode(caps, src, relPath, pkg, false, 0); n.Symbol != "" {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

func extractVue(vueGr *grammar, tsGr *grammar, src []byte, relPath string) []parsedNode {
	if vueGr == nil {
		return nil
	}
	componentName := strings.TrimSuffix(filepath.Base(relPath), ".vue")
	pkg := dirPackage(relPath)

	vueMatches := vueGr.extract(src)
	var nodes []parsedNode

	for _, caps := range vueMatches {
		switch {
		case hasCap(caps, "script_block"):
			block, _ := findCap(caps, "script_block")
			scriptText, hasText := findCap(caps, "script_text")
			if !hasText {
				// Empty <script></script> -- emit a legacy node with no content
				nodes = append(nodes, parsedNode{
					FilePath: relPath, Package: pkg,
					Symbol: componentName, Kind: "vue-setup-legacy",
					LineStart: block.startLine, LineEnd: block.endLine,
					Exported: true, DependsOn: []string{},
					ContentHash: hashBytes(src[block.startByte:block.endByte]),
				})
				continue
			}
			startTagBytes := src[block.startByte:scriptText.startByte]
			isSetup := bytes.Contains(startTagBytes, []byte("setup"))
			scriptSrc := src[scriptText.startByte:scriptText.endByte]
			lineOffset := scriptText.startLine - 1

			if !isSetup {
				nodes = append(nodes, parsedNode{
					FilePath: relPath, Package: pkg,
					Symbol: componentName, Kind: "vue-setup-legacy",
					LineStart: block.startLine, LineEnd: block.endLine,
					Exported: true, DependsOn: []string{},
					ContentHash: hashBytes(src[block.startByte:block.endByte]),
				})
				continue
			}

			props, emits, expose := vueDetectMacros(scriptSrc)
			var sigParts []string
			if props != "" {
				sigParts = append(sigParts, "props: "+props)
			}
			if emits != "" {
				sigParts = append(sigParts, "emits: "+emits)
			}
			if expose != "" {
				sigParts = append(sigParts, "expose: "+expose)
			}
			nodes = append(nodes, parsedNode{
				FilePath: relPath, Package: pkg,
				Symbol: componentName, Kind: "vue-setup",
				LineStart: block.startLine, LineEnd: block.endLine,
				Signature: strings.Join(sigParts, " "),
				Exported:  true, DependsOn: []string{},
				ContentHash: hashBytes(src[block.startByte:block.endByte]),
			})

			if tsGr != nil {
				for _, tsCaps := range tsGr.extract(scriptSrc) {
					if n := tsBuildNode(tsCaps, scriptSrc, relPath, pkg, true, lineOffset); n.Symbol != "" {
						n.Symbol = componentName + "." + n.Symbol
						n.Kind = "vue-method"
						n.Receiver = componentName
						nodes = append(nodes, n)
					}
				}
			}

		case hasCap(caps, "template_block"):
			tmpl, _ := findCap(caps, "template_block")
			nodes = append(nodes, parsedNode{
				FilePath: relPath, Package: pkg,
				Symbol: componentName, Kind: "vue-template",
				LineStart: tmpl.startLine, LineEnd: tmpl.endLine,
				Exported: true, DependsOn: []string{},
				ContentHash: hashBytes(src[tmpl.startByte:tmpl.endByte]),
			})

		case hasCap(caps, "style_block"):
			style, _ := findCap(caps, "style_block")
			sig := ""
			blockBytes := src[style.startByte:style.endByte]
			if bytes.Contains(blockBytes, []byte("scoped")) {
				sig = "scoped"
			} else if bytes.Contains(blockBytes, []byte("module")) {
				sig = "module"
			}
			nodes = append(nodes, parsedNode{
				FilePath: relPath, Package: pkg,
				Symbol: componentName, Kind: "vue-style",
				LineStart: style.startLine, LineEnd: style.endLine,
				Signature: sig,
				Exported:  true, DependsOn: []string{},
				ContentHash: hashBytes(src[style.startByte:style.endByte]),
			})
		}
	}
	return nodes
}

// tsBuildNode maps a TS query match to a parsedNode.
// inVueSetup=true means we are inside a <script setup> block.
// lineOffset is added to line numbers when in a Vue script block.
func tsBuildNode(caps []capture, src []byte, relPath, pkg string, inVueSetup bool, lineOffset int) parsedNode {
	node, hasNode := findCap(caps, "node")
	sym, hasSym := findCap(caps, "symbol")
	if !hasNode || !hasSym {
		return parsedNode{}
	}

	symbolText := sym.text(src)
	if symbolText == "" {
		return parsedNode{}
	}

	kind := tsKind(node.nodeType, caps, src)
	if kind == "" {
		return parsedNode{}
	}

	// Skip primitive const exports in non-Vue context
	if !inVueSetup && kind == "const" {
		if val, ok := findCap(caps, "value"); ok {
			if isTSPrimitive(val.nodeType) {
				return parsedNode{}
			}
		}
	}

	// In Vue script setup, arrow/func consts are methods (handled by caller)
	// Non-method consts that are reactive calls are already excluded by query
	// structure (they have call_expression values, not arrow_function)

	sig := tsSig(caps, src)

	return parsedNode{
		FilePath:    relPath,
		Package:     pkg,
		Symbol:      symbolText,
		Kind:        kind,
		LineStart:   node.startLine + lineOffset,
		LineEnd:     node.endLine + lineOffset,
		Signature:   sig,
		Exported:    true,
		DependsOn:   []string{},
		ContentHash: hashBytes(src[node.startByte:node.endByte]),
	}
}

func tsKind(nodeType string, caps []capture, src []byte) string {
	switch nodeType {
	case "function_declaration", "generator_function_declaration":
		return "func"
	case "class_declaration", "abstract_class_declaration":
		return "class"
	case "interface_declaration":
		return "interface"
	case "type_alias_declaration":
		return "type"
	case "enum_declaration":
		return "enum"
	case "variable_declarator":
		val, ok := findCap(caps, "value")
		if !ok {
			return "const"
		}
		if val.nodeType == "arrow_function" || val.nodeType == "function_expression" {
			return "func"
		}
		return "const"
	}
	return ""
}

func tsSig(caps []capture, src []byte) string {
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
	if params != "" {
		return params + ret
	}
	// Class/interface heritage
	if h, ok := findCap(caps, "heritage"); ok {
		return strings.TrimSpace(h.text(src))
	}
	if e, ok := findCap(caps, "extends"); ok {
		return strings.TrimSpace(e.text(src))
	}
	return ""
}

func isTSPrimitive(nodeType string) bool {
	switch nodeType {
	case "string", "number", "true", "false", "null", "undefined", "template_string":
		return true
	}
	return false
}

func hasCap(caps []capture, name string) bool {
	_, ok := findCap(caps, name)
	return ok
}

// vueDetectMacros scans raw script source for defineProps/defineEmits/defineExpose
// and returns a short summary of each.
func vueDetectMacros(src []byte) (props, emits, expose string) {
	props = vueExtractMacroArg(src, "defineProps")
	emits = vueExtractMacroArg(src, "defineEmits")
	expose = vueExtractMacroArg(src, "defineExpose")
	return
}

func vueExtractMacroArg(src []byte, name string) string {
	idx := bytes.Index(src, []byte(name))
	if idx == -1 {
		return ""
	}
	rest := src[idx+len(name):]
	if len(rest) == 0 {
		return ""
	}
	// Find first < or (
	var open, close byte
	start := -1
	for i := 0; i < len(rest) && i < 10; i++ {
		if rest[i] == '<' {
			open, close = '<', '>'
			start = i
			break
		}
		if rest[i] == '(' {
			open, close = '(', ')'
			start = i
			break
		}
	}
	if start == -1 {
		return ""
	}
	depth := 0
	for i := start; i < len(rest) && i < start+512; i++ {
		if rest[i] == open {
			depth++
		} else if rest[i] == close {
			depth--
			if depth == 0 {
				result := strings.TrimSpace(string(rest[start+1 : i]))
				if len(result) > 80 {
					result = result[:77] + "..."
				}
				return result
			}
		}
	}
	return ""
}
