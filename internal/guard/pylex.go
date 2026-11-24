// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

type pyTokKind int

const (
	pyName pyTokKind = iota
	pyNumber
	pyString
	pyOp
	pyNewline
	pyIndent
	pyDedent
	pyEnd
)

// pyToken is one Python token. A string's text is its value with simple escapes decoded and every f-string
// replacement field left out, so a path written around a field ("{home}/.aws/credentials") keeps its literal part;
// names lists the names the fields read.
type pyToken struct {
	kind  pyTokKind
	text  string
	names []string
}

var errPyLex = errors.New("python code the guard cannot tokenize")

// pyOps lists Python's operators longest first, so the scan takes the longest match.
var pyOps = []string{
	"**=", "//=", ">>=", "<<=", "...",
	"->", ":=", "==", "!=", "<=", ">=", "**", "//", "<<", ">>", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "@=",
	"+", "-", "*", "/", "%", "@", "&", "|", "^", "~", "<", ">", "(", ")", "[", "]", "{", "}", ",", ":", ";", ".", "=",
}

type pyLexer struct {
	src     string
	i       int
	depth   int
	indents []int
	out     []pyToken
}

// pyTokenize splits Python source into tokens with logical newlines and indentation, the way Python's own tokenizer
// does for the subset the guard reads. Comments are dropped: what a comment names is never an access.
func pyTokenize(src string) ([]pyToken, error) {
	lx := &pyLexer{src: src, indents: []int{0}}
	if err := lx.run(); err != nil {
		return nil, err
	}
	return lx.out, nil
}

func (lx *pyLexer) emit(kind pyTokKind, text string) {
	lx.out = append(lx.out, pyToken{kind: kind, text: text})
}

func (lx *pyLexer) run() error {
	atLineStart := true
	for {
		if atLineStart && lx.depth == 0 {
			if done, err := lx.indentation(); err != nil || done {
				if err != nil {
					return err
				}
				break
			}
			atLineStart = false
		}
		if lx.i >= len(lx.src) {
			break
		}
		c := lx.src[lx.i]
		switch {
		case c == '#':
			for lx.i < len(lx.src) && lx.src[lx.i] != '\n' {
				lx.i++
			}
		case c == '\\' && lx.i+1 < len(lx.src) && (lx.src[lx.i+1] == '\n' || lx.src[lx.i+1] == '\r'):
			lx.i += 2
			if lx.i < len(lx.src) && lx.src[lx.i-1] == '\r' && lx.src[lx.i] == '\n' {
				lx.i++
			}
		case c == '\n' || c == '\r':
			lx.i++
			if lx.depth == 0 {
				lx.newline()
				atLineStart = true
			}
		case c == ' ' || c == '\t' || c == '\f':
			lx.i++
		case c == '"' || c == '\'':
			tok, err := lx.str("")
			if err != nil {
				return err
			}
			lx.out = append(lx.out, tok)
		case c >= '0' && c <= '9' || c == '.' && lx.i+1 < len(lx.src) && lx.src[lx.i+1] >= '0' && lx.src[lx.i+1] <= '9':
			start := lx.i
			for lx.i < len(lx.src) && (isPyIdentByte(lx.src[lx.i]) || lx.src[lx.i] == '.' ||
				(lx.src[lx.i] == '+' || lx.src[lx.i] == '-') && (lx.src[lx.i-1] == 'e' || lx.src[lx.i-1] == 'E')) {
				lx.i++
			}
			lx.emit(pyNumber, lx.src[start:lx.i])
		case isPyIdentStart(lx.src[lx.i:]):
			start := lx.i
			for lx.i < len(lx.src) {
				r, size := utf8.DecodeRuneInString(lx.src[lx.i:])
				if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
					break
				}
				lx.i += size
			}
			word := lx.src[start:lx.i]
			if lx.i < len(lx.src) && (lx.src[lx.i] == '"' || lx.src[lx.i] == '\'') && isPyStringPrefix(word) {
				tok, err := lx.str(strings.ToLower(word))
				if err != nil {
					return err
				}
				lx.out = append(lx.out, tok)
				continue
			}
			lx.emit(pyName, word)
		default:
			op := ""
			for _, candidate := range pyOps {
				if strings.HasPrefix(lx.src[lx.i:], candidate) {
					op = candidate
					break
				}
			}
			if op == "" {
				return errPyLex
			}
			lx.i += len(op)
			switch op {
			case "(", "[", "{":
				lx.depth++
			case ")", "]", "}":
				if lx.depth == 0 {
					return errPyLex
				}
				lx.depth--
			}
			lx.emit(pyOp, op)
		}
	}
	if lx.depth != 0 {
		return errPyLex
	}
	lx.newline()
	for len(lx.indents) > 1 {
		lx.indents = lx.indents[:len(lx.indents)-1]
		lx.emit(pyDedent, "")
	}
	lx.emit(pyEnd, "")
	return nil
}

func (lx *pyLexer) newline() {
	if n := len(lx.out); n > 0 && lx.out[n-1].kind != pyNewline && lx.out[n-1].kind != pyIndent && lx.out[n-1].kind != pyDedent {
		lx.emit(pyNewline, "")
	}
}

// indentation reads the leading whitespace of a line, skipping blank and comment-only lines, and emits the indent
// and dedent tokens it implies. done is true at the end of the source.
func (lx *pyLexer) indentation() (done bool, err error) {
	for {
		col := 0
		for lx.i < len(lx.src) && (lx.src[lx.i] == ' ' || lx.src[lx.i] == '\t' || lx.src[lx.i] == '\f') {
			if lx.src[lx.i] == '\t' {
				col = (col/8 + 1) * 8
			} else if lx.src[lx.i] == ' ' {
				col++
			}
			lx.i++
		}
		if lx.i >= len(lx.src) {
			return true, nil
		}
		switch lx.src[lx.i] {
		case '\n', '\r':
			lx.i++
			continue
		case '#':
			for lx.i < len(lx.src) && lx.src[lx.i] != '\n' {
				lx.i++
			}
			continue
		}
		top := lx.indents[len(lx.indents)-1]
		if col > top {
			lx.indents = append(lx.indents, col)
			lx.emit(pyIndent, "")
		}
		for col < lx.indents[len(lx.indents)-1] {
			lx.indents = lx.indents[:len(lx.indents)-1]
			lx.emit(pyDedent, "")
		}
		if col != lx.indents[len(lx.indents)-1] {
			return false, errPyLex
		}
		return false, nil
	}
}

func isPyIdentByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func isPyIdentStart(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return r == '_' || unicode.IsLetter(r)
}

// isPyStringPrefix is Python's string prefixes in any case: r, u, b, f and t, and the pairs that combine.
func isPyStringPrefix(word string) bool {
	switch strings.ToLower(word) {
	case "r", "u", "b", "f", "t", "br", "rb", "fr", "rf", "tr", "rt":
		return true
	}
	return false
}

// str reads a string literal at lx.i, the quote itself, with prefix already consumed.
func (lx *pyLexer) str(prefix string) (pyToken, error) {
	raw := strings.Contains(prefix, "r")
	formatted := strings.Contains(prefix, "f") || strings.Contains(prefix, "t")
	quote := lx.src[lx.i : lx.i+1]
	if strings.HasPrefix(lx.src[lx.i:], quote+quote+quote) {
		quote = quote + quote + quote
	}
	lx.i += len(quote)
	var value strings.Builder
	var names []string
	for {
		if lx.i >= len(lx.src) {
			return pyToken{}, errPyLex
		}
		if strings.HasPrefix(lx.src[lx.i:], quote) {
			lx.i += len(quote)
			return pyToken{kind: pyString, text: value.String(), names: names}, nil
		}
		c := lx.src[lx.i]
		if len(quote) == 1 && (c == '\n' || c == '\r') {
			return pyToken{}, errPyLex
		}
		if c == '\\' && lx.i+1 < len(lx.src) {
			next := lx.src[lx.i+1]
			lx.i += 2
			if raw {
				value.WriteByte('\\')
				value.WriteByte(next)
				continue
			}
			lx.escape(&value, next, strings.Contains(prefix, "b"))
			continue
		}
		if formatted && (c == '{' || c == '}') {
			if lx.i+1 < len(lx.src) && lx.src[lx.i+1] == c {
				value.WriteByte(c)
				lx.i += 2
				continue
			}
			if c == '}' {
				return pyToken{}, errPyLex
			}
			fieldNames, err := lx.field()
			if err != nil {
				return pyToken{}, err
			}
			names = append(names, fieldNames...)
			continue
		}
		value.WriteByte(c)
		lx.i++
	}
}

// escape decodes the escape whose letter is next, lx.i standing just past it. An escape Python does not know, and
// \N{name}, are kept as written, as Python keeps an unknown one.
func (lx *pyLexer) escape(value *strings.Builder, next byte, bytesLit bool) {
	simple := map[byte]byte{'n': '\n', 't': '\t', 'r': '\r', 'a': '\a', 'b': '\b', 'f': '\f', 'v': '\v',
		'\\': '\\', '\'': '\'', '"': '"'}
	if c, ok := simple[next]; ok {
		value.WriteByte(c)
		return
	}
	switch {
	case next == '\n':
		return
	case next >= '0' && next <= '7':
		n := int(next - '0')
		for k := 0; k < 2 && lx.i < len(lx.src) && lx.src[lx.i] >= '0' && lx.src[lx.i] <= '7'; k++ {
			n = n*8 + int(lx.src[lx.i]-'0')
			lx.i++
		}
		writeCode(value, n, bytesLit)
		return
	case next == 'x' || !bytesLit && (next == 'u' || next == 'U'):
		width := map[byte]int{'x': 2, 'u': 4, 'U': 8}[next]
		if lx.i+width <= len(lx.src) {
			n := 0
			valid := true
			for _, c := range lx.src[lx.i : lx.i+width] {
				d := strings.IndexRune("0123456789abcdef", unicode.ToLower(c))
				if d < 0 {
					valid = false
					break
				}
				n = n*16 + d
			}
			if valid {
				lx.i += width
				writeCode(value, n, bytesLit)
				return
			}
		}
	}
	value.WriteByte('\\')
	value.WriteByte(next)
}

// writeCode writes a decoded escape: a code point in a str literal, a byte in a bytes literal.
func writeCode(value *strings.Builder, n int, bytesLit bool) {
	if bytesLit || n < utf8.RuneSelf {
		value.WriteByte(byte(n))
		return
	}
	value.WriteRune(rune(n))
}

// field reads an f-string replacement field from its opening brace through its closing one and returns the names
// its expression reads. Nested strings, such as a dict key in quotes, are read whole so a brace in them does not
// end the field.
func (lx *pyLexer) field() ([]string, error) {
	lx.i++
	start := lx.i
	depth := 0
	end := -1
	for lx.i < len(lx.src) {
		c := lx.src[lx.i]
		switch {
		case c == '"' || c == '\'':
			if _, err := lx.str(""); err != nil {
				return nil, err
			}
			continue
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']':
			depth--
		case c == '}':
			if depth == 0 {
				if end < 0 {
					end = lx.i
				}
				lx.i++
				return pyFieldNames(lx.src[start:end]), nil
			}
			depth--
		case (c == ':' || c == '!') && depth == 0 && end < 0 && !strings.HasPrefix(lx.src[lx.i:], "!="):
			end = lx.i
		}
		lx.i++
	}
	return nil, errPyLex
}

// pyFieldNames returns the names an f-string field's expression reads, leaving out attribute names after a dot.
func pyFieldNames(expr string) []string {
	toks, err := pyTokenize(expr)
	if err != nil {
		return nil
	}
	var names []string
	for i, tok := range toks {
		if tok.kind == pyName && (i == 0 || toks[i-1].text != ".") {
			names = append(names, tok.text)
		}
		names = append(names, tok.names...)
	}
	return names
}
