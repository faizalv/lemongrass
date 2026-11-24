// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import (
	"errors"
	"strings"
)

// The parser reads the statement and expression subset of Python that edit and inspection code uses. Anything it
// does not know, such as a match statement or a parenthesized with, is a parse error, and the caller falls back to
// the mention scan, so an unread construct can only keep a refusal, never lift one.

var errPyParse = errors.New("python code outside the subset the guard reads")

// pyParseError says where the parser stopped, for the replay's account of what the reading does not cover.
type pyParseError struct {
	err  error
	near string
	kind pyTokKind
}

func (e *pyParseError) Error() string { return e.err.Error() + " near " + e.near }

func (e *pyParseError) Unwrap() error { return e.err }

type pyNodeKind int

const (
	nName pyNodeKind = iota
	nStr
	nConst
	nAttr
	nSub
	nCall
	nBin
	nBool
	nUnary
	nCmp
	nTernary
	nList
	nTuple
	nSet
	nDict
	nComp
	nLambda
	nStar
	nWalrus
	nYield
	nSlice
)

type pyNode struct {
	kind pyNodeKind
	// text is a name, an attribute name, a binary or unary operator, or a string literal's value.
	text string
	// lit indexes a string literal in pyProgram.lits.
	lit  int
	kids []*pyNode
	// kw holds a call argument's keyword, "*" or "**" for an unpacked one, and "" for a positional one.
	kw []string
	// comp holds a comprehension's loops: target, iterable and conditions.
	comp []pyCompFor
}

type pyCompFor struct {
	target *pyNode
	iter   *pyNode
	conds  []*pyNode
}

type pyStmtKind int

const (
	sExpr pyStmtKind = iota
	sAssign
	sAugAssign
	sFor
	sWith
	sReturn
	sBlock
	sDef
)

type pyStmt struct {
	kind    pyStmtKind
	targets []*pyNode
	value   *pyNode
	// exprs are evaluated for their sinks only: conditions, asserts, decorators, defaults, raise.
	exprs  []*pyNode
	bodies [][]*pyStmt
	def    *pyDef
}

type pyDef struct {
	name     string
	params   []string
	defaults map[string]*pyNode
	varArgs  string
	varKw    string
	method   bool
	body     []*pyStmt
}

// pyLit is one string literal of the program.
type pyLit struct {
	value string
	names []string
}

type pyProgram struct {
	stmts   []*pyStmt
	defs    []*pyDef
	lits    []pyLit
	aliases map[string]string
	// dynamic is set when the code reaches code or names the guard cannot follow, such as eval or getattr.
	dynamic bool
}

type pyParser struct {
	toks       []pyToken
	pos        int
	prog       *pyProgram
	classDepth int
}

// pyDynamicNames are builtins that run code or resolve names at run time, which no reading of the text can follow.
var pyDynamicNames = map[string]bool{
	"eval": true, "exec": true, "compile": true, "__import__": true, "getattr": true, "setattr": true, "delattr": true,
	"globals": true, "locals": true, "vars": true, "execfile": true, "__builtins__": true,
}

// pyDynamicModules do the same once imported.
var pyDynamicModules = map[string]bool{
	"importlib": true, "runpy": true, "operator": true, "ctypes": true, "code": true, "builtins": true,
}

// pyDynamicAttrs reach a namespace by name.
var pyDynamicAttrs = map[string]bool{
	"__dict__": true, "__globals__": true, "__builtins__": true, "__getattribute__": true, "__import__": true,
}

var pyKeywords = map[string]bool{
	"and": true, "as": true, "assert": true, "async": true, "await": true, "break": true, "class": true,
	"continue": true, "def": true, "del": true, "elif": true, "else": true, "except": true, "finally": true,
	"for": true, "from": true, "global": true, "if": true, "import": true, "in": true, "is": true, "lambda": true,
	"nonlocal": true, "not": true, "or": true, "pass": true, "raise": true, "return": true, "try": true,
	"while": true, "with": true, "yield": true,
}

func pyParse(src string) (*pyProgram, error) {
	toks, err := pyTokenize(src)
	if err != nil {
		return nil, err
	}
	p := &pyParser{toks: toks, prog: &pyProgram{aliases: map[string]string{}}}
	for !p.at(pyEnd, "") {
		stmts, err := p.statement()
		if err != nil {
			return nil, &pyParseError{err: err, near: p.peek().text, kind: p.peek().kind}
		}
		p.prog.stmts = append(p.prog.stmts, stmts...)
	}
	return p.prog, nil
}

func (p *pyParser) peek() pyToken { return p.toks[p.pos] }

func (p *pyParser) peekAt(n int) pyToken {
	if p.pos+n < len(p.toks) {
		return p.toks[p.pos+n]
	}
	return p.toks[len(p.toks)-1]
}

func (p *pyParser) at(kind pyTokKind, text string) bool {
	t := p.peek()
	return t.kind == kind && (text == "" || t.text == text)
}

func (p *pyParser) atOp(text string) bool { return p.at(pyOp, text) }

func (p *pyParser) atName(text string) bool { return p.at(pyName, text) }

func (p *pyParser) next() pyToken {
	t := p.toks[p.pos]
	if t.kind != pyEnd {
		p.pos++
	}
	return t
}

func (p *pyParser) expectOp(text string) error {
	if !p.atOp(text) {
		return errPyParse
	}
	p.next()
	return nil
}

func (p *pyParser) expectName(text string) error {
	if !p.atName(text) {
		return errPyParse
	}
	p.next()
	return nil
}

func (p *pyParser) identifier() (string, error) {
	t := p.peek()
	if t.kind != pyName || pyKeywords[t.text] {
		return "", errPyParse
	}
	p.next()
	return t.text, nil
}

func (p *pyParser) statement() ([]*pyStmt, error) {
	t := p.peek()
	if t.kind == pyNewline {
		p.next()
		return nil, nil
	}
	if t.kind == pyIndent || t.kind == pyDedent {
		return nil, errPyParse
	}
	if p.atOp("@") {
		return p.decorated()
	}
	if t.kind == pyName {
		switch t.text {
		case "if", "while":
			return p.ifWhile()
		case "for":
			return p.forStmt()
		case "try":
			return p.tryStmt()
		case "with":
			return p.withStmt()
		case "def":
			return p.defStmt()
		case "class":
			return p.classStmt()
		case "async":
			p.next()
			if !p.atName("def") && !p.atName("for") && !p.atName("with") {
				return nil, errPyParse
			}
			return p.statement()
		}
	}
	return p.simpleLine()
}

// simpleLine reads the simple statements of one line, separated by semicolons.
func (p *pyParser) simpleLine() ([]*pyStmt, error) {
	var out []*pyStmt
	for {
		stmt, err := p.simple()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			out = append(out, stmt)
		}
		if p.atOp(";") {
			p.next()
			if p.at(pyNewline, "") || p.at(pyEnd, "") {
				break
			}
			continue
		}
		break
	}
	if p.at(pyNewline, "") {
		p.next()
		return out, nil
	}
	if p.at(pyEnd, "") || p.at(pyDedent, "") {
		return out, nil
	}
	return nil, errPyParse
}

func (p *pyParser) simple() (*pyStmt, error) {
	t := p.peek()
	if t.kind == pyName {
		switch t.text {
		case "pass", "break", "continue":
			p.next()
			return nil, nil
		case "return":
			p.next()
			if p.endOfSimple() {
				return nil, nil
			}
			value, err := p.testListStar()
			if err != nil {
				return nil, err
			}
			return &pyStmt{kind: sReturn, value: value}, nil
		case "raise":
			p.next()
			var exprs []*pyNode
			if !p.endOfSimple() {
				e, err := p.test()
				if err != nil {
					return nil, err
				}
				exprs = append(exprs, e)
				if p.atName("from") {
					p.next()
					e, err := p.test()
					if err != nil {
						return nil, err
					}
					exprs = append(exprs, e)
				}
			}
			return &pyStmt{kind: sExpr, exprs: exprs}, nil
		case "global", "nonlocal":
			p.next()
			for {
				if _, err := p.identifier(); err != nil {
					return nil, err
				}
				if !p.atOp(",") {
					return nil, nil
				}
				p.next()
			}
		case "del":
			p.next()
			e, err := p.testListStar()
			if err != nil {
				return nil, err
			}
			return &pyStmt{kind: sExpr, exprs: []*pyNode{e}}, nil
		case "assert":
			p.next()
			e, err := p.test()
			if err != nil {
				return nil, err
			}
			exprs := []*pyNode{e}
			if p.atOp(",") {
				p.next()
				msg, err := p.test()
				if err != nil {
					return nil, err
				}
				exprs = append(exprs, msg)
			}
			return &pyStmt{kind: sExpr, exprs: exprs}, nil
		case "import":
			p.next()
			return nil, p.importNames()
		case "from":
			p.next()
			return nil, p.fromImport()
		}
	}
	return p.exprStatement()
}

func (p *pyParser) endOfSimple() bool {
	return p.at(pyNewline, "") || p.at(pyEnd, "") || p.atOp(";") || p.at(pyDedent, "")
}

func (p *pyParser) dottedName() (string, error) {
	name, err := p.identifier()
	if err != nil {
		return "", err
	}
	for p.atOp(".") {
		p.next()
		part, err := p.identifier()
		if err != nil {
			return "", err
		}
		name += "." + part
	}
	return name, nil
}

func (p *pyParser) importNames() error {
	for {
		module, err := p.dottedName()
		if err != nil {
			return err
		}
		if p.atName("as") {
			p.next()
			alias, err := p.identifier()
			if err != nil {
				return err
			}
			p.prog.aliases[alias] = module
		} else {
			root, _, _ := strings.Cut(module, ".")
			p.prog.aliases[root] = root
		}
		if pyDynamicModules[strings.SplitN(module, ".", 2)[0]] {
			p.prog.dynamic = true
		}
		if !p.atOp(",") {
			return nil
		}
		p.next()
	}
}

func (p *pyParser) fromImport() error {
	module := ""
	for p.atOp(".") || p.atOp("...") {
		module += p.next().text
	}
	if !p.atName("import") {
		name, err := p.dottedName()
		if err != nil {
			return err
		}
		module += name
	}
	if err := p.expectName("import"); err != nil {
		return err
	}
	if pyDynamicModules[strings.SplitN(module, ".", 2)[0]] {
		p.prog.dynamic = true
	}
	if p.atOp("*") {
		p.next()
		p.prog.dynamic = true
		return nil
	}
	paren := p.atOp("(")
	if paren {
		p.next()
	}
	for {
		if paren && p.atOp(")") {
			break
		}
		name, err := p.identifier()
		if err != nil {
			return err
		}
		alias := name
		if p.atName("as") {
			p.next()
			if alias, err = p.identifier(); err != nil {
				return err
			}
		}
		p.prog.aliases[alias] = module + "." + name
		if pyDynamicNames[name] {
			p.prog.dynamic = true
		}
		if !p.atOp(",") {
			break
		}
		p.next()
	}
	if paren {
		return p.expectOp(")")
	}
	return nil
}

var pyAugOps = map[string]bool{"+=": true, "-=": true, "*=": true, "/=": true, "//=": true, "%=": true, "@=": true,
	"&=": true, "|=": true, "^=": true, ">>=": true, "<<=": true, "**=": true}

func (p *pyParser) exprStatement() (*pyStmt, error) {
	first, err := p.testListStar()
	if err != nil {
		return nil, err
	}
	switch {
	case p.atOp("="):
		targets := []*pyNode{first}
		var value *pyNode
		for p.atOp("=") {
			p.next()
			if value, err = p.assignValue(); err != nil {
				return nil, err
			}
			if p.atOp("=") {
				targets = append(targets, value)
			}
		}
		return &pyStmt{kind: sAssign, targets: targets, value: value}, nil
	case p.peek().kind == pyOp && pyAugOps[p.peek().text]:
		p.next()
		value, err := p.assignValue()
		if err != nil {
			return nil, err
		}
		return &pyStmt{kind: sAugAssign, targets: []*pyNode{first}, value: value}, nil
	case p.atOp(":"):
		p.next()
		annotation, err := p.test()
		if err != nil {
			return nil, err
		}
		if !p.atOp("=") {
			return &pyStmt{kind: sExpr, exprs: []*pyNode{annotation}}, nil
		}
		p.next()
		value, err := p.assignValue()
		if err != nil {
			return nil, err
		}
		return &pyStmt{kind: sAssign, targets: []*pyNode{first}, value: value, exprs: []*pyNode{annotation}}, nil
	}
	return &pyStmt{kind: sExpr, exprs: []*pyNode{first}}, nil
}

func (p *pyParser) assignValue() (*pyNode, error) {
	if p.atName("yield") {
		return p.yieldExpr()
	}
	return p.testListStar()
}

func (p *pyParser) suite() ([]*pyStmt, error) {
	if err := p.expectOp(":"); err != nil {
		return nil, err
	}
	if !p.at(pyNewline, "") {
		return p.simpleLine()
	}
	p.next()
	if !p.at(pyIndent, "") {
		return nil, errPyParse
	}
	p.next()
	var body []*pyStmt
	for !p.at(pyDedent, "") {
		if p.at(pyEnd, "") {
			return nil, errPyParse
		}
		stmts, err := p.statement()
		if err != nil {
			return nil, err
		}
		body = append(body, stmts...)
	}
	p.next()
	return body, nil
}

func (p *pyParser) ifWhile() ([]*pyStmt, error) {
	stmt := &pyStmt{kind: sBlock}
	p.next()
	for {
		cond, err := p.namedExpr()
		if err != nil {
			return nil, err
		}
		stmt.exprs = append(stmt.exprs, cond)
		body, err := p.suite()
		if err != nil {
			return nil, err
		}
		stmt.bodies = append(stmt.bodies, body)
		if !p.atName("elif") {
			break
		}
		p.next()
	}
	if p.atName("else") {
		p.next()
		body, err := p.suite()
		if err != nil {
			return nil, err
		}
		stmt.bodies = append(stmt.bodies, body)
	}
	return []*pyStmt{stmt}, nil
}

func (p *pyParser) forStmt() ([]*pyStmt, error) {
	p.next()
	target, err := p.exprList()
	if err != nil {
		return nil, err
	}
	if err := p.expectName("in"); err != nil {
		return nil, err
	}
	iter, err := p.testListStar()
	if err != nil {
		return nil, err
	}
	body, err := p.suite()
	if err != nil {
		return nil, err
	}
	stmt := &pyStmt{kind: sFor, targets: []*pyNode{target}, value: iter, bodies: [][]*pyStmt{body}}
	if p.atName("else") {
		p.next()
		orelse, err := p.suite()
		if err != nil {
			return nil, err
		}
		stmt.bodies = append(stmt.bodies, orelse)
	}
	return []*pyStmt{stmt}, nil
}

func (p *pyParser) tryStmt() ([]*pyStmt, error) {
	p.next()
	stmt := &pyStmt{kind: sBlock}
	body, err := p.suite()
	if err != nil {
		return nil, err
	}
	stmt.bodies = append(stmt.bodies, body)
	for p.atName("except") || p.atName("else") || p.atName("finally") {
		if p.next().text == "except" {
			if p.atOp("*") {
				p.next()
			}
			if !p.atOp(":") {
				e, err := p.test()
				if err != nil {
					return nil, err
				}
				stmt.exprs = append(stmt.exprs, e)
				if p.atName("as") {
					p.next()
					if _, err := p.identifier(); err != nil {
						return nil, err
					}
				}
			}
		}
		body, err := p.suite()
		if err != nil {
			return nil, err
		}
		stmt.bodies = append(stmt.bodies, body)
	}
	return []*pyStmt{stmt}, nil
}

func (p *pyParser) withStmt() ([]*pyStmt, error) {
	p.next()
	var out []*pyStmt
	for {
		e, err := p.test()
		if err != nil {
			return nil, err
		}
		item := &pyStmt{kind: sWith, value: e}
		if p.atName("as") {
			p.next()
			target, err := p.exprList()
			if err != nil {
				return nil, err
			}
			item.targets = []*pyNode{target}
		}
		out = append(out, item)
		if !p.atOp(",") {
			break
		}
		p.next()
	}
	body, err := p.suite()
	if err != nil {
		return nil, err
	}
	return append(out, &pyStmt{kind: sBlock, bodies: [][]*pyStmt{body}}), nil
}

func (p *pyParser) decorated() ([]*pyStmt, error) {
	var decorators []*pyNode
	for p.atOp("@") {
		p.next()
		d, err := p.namedExpr()
		if err != nil {
			return nil, err
		}
		decorators = append(decorators, d)
		if !p.at(pyNewline, "") {
			return nil, errPyParse
		}
		p.next()
	}
	if p.atName("async") {
		p.next()
	}
	if !p.atName("def") && !p.atName("class") {
		return nil, errPyParse
	}
	stmts, err := p.statement()
	if err != nil {
		return nil, err
	}
	return append([]*pyStmt{{kind: sExpr, exprs: decorators}}, stmts...), nil
}

func (p *pyParser) defStmt() ([]*pyStmt, error) {
	p.next()
	name, err := p.identifier()
	if err != nil {
		return nil, err
	}
	def := &pyDef{name: name, defaults: map[string]*pyNode{}, method: p.classDepth > 0}
	if p.atOp("[") {
		return nil, errPyParse
	}
	if err := p.expectOp("("); err != nil {
		return nil, err
	}
	var annotations []*pyNode
	for !p.atOp(")") {
		star := ""
		if p.atOp("*") || p.atOp("**") || p.atOp("/") {
			star = p.next().text
			if star == "/" || star == "*" && (p.atOp(",") || p.atOp(")")) {
				if p.atOp(",") {
					p.next()
				}
				continue
			}
		}
		param, err := p.identifier()
		if err != nil {
			return nil, err
		}
		if p.atOp(":") {
			p.next()
			a, err := p.test()
			if err != nil {
				return nil, err
			}
			annotations = append(annotations, a)
		}
		switch star {
		case "*":
			def.varArgs = param
		case "**":
			def.varKw = param
		default:
			def.params = append(def.params, param)
		}
		if p.atOp("=") {
			p.next()
			d, err := p.test()
			if err != nil {
				return nil, err
			}
			def.defaults[param] = d
		}
		if !p.atOp(",") {
			break
		}
		p.next()
	}
	if err := p.expectOp(")"); err != nil {
		return nil, err
	}
	if p.atOp("->") {
		p.next()
		a, err := p.test()
		if err != nil {
			return nil, err
		}
		annotations = append(annotations, a)
	}
	depth := p.classDepth
	p.classDepth = 0
	body, err := p.suite()
	p.classDepth = depth
	if err != nil {
		return nil, err
	}
	def.body = body
	p.prog.defs = append(p.prog.defs, def)
	return []*pyStmt{{kind: sDef, def: def, exprs: annotations}}, nil
}

func (p *pyParser) classStmt() ([]*pyStmt, error) {
	p.next()
	if _, err := p.identifier(); err != nil {
		return nil, err
	}
	var bases []*pyNode
	if p.atOp("(") {
		call, err := p.callArgs(&pyNode{kind: nName, text: "object"})
		if err != nil {
			return nil, err
		}
		bases = call.kids[1:]
	}
	p.classDepth++
	body, err := p.suite()
	p.classDepth--
	if err != nil {
		return nil, err
	}
	return []*pyStmt{{kind: sBlock, exprs: bases, bodies: [][]*pyStmt{body}}}, nil
}

// testListStar reads one expression or a tuple of them, starred items allowed.
func (p *pyParser) testListStar() (*pyNode, error) {
	first, err := p.starOrTest()
	if err != nil {
		return nil, err
	}
	if !p.atOp(",") {
		return first, nil
	}
	tuple := &pyNode{kind: nTuple, kids: []*pyNode{first}}
	for p.atOp(",") {
		p.next()
		if p.endOfTuple() {
			break
		}
		e, err := p.starOrTest()
		if err != nil {
			return nil, err
		}
		tuple.kids = append(tuple.kids, e)
	}
	return tuple, nil
}

func (p *pyParser) endOfTuple() bool {
	t := p.peek()
	if t.kind != pyOp {
		return t.kind == pyNewline || t.kind == pyEnd || t.kind == pyDedent || t.kind == pyName && t.text == "in"
	}
	switch t.text {
	case "=", ")", "]", "}", ":", ";":
		return true
	}
	return pyAugOps[t.text]
}

func (p *pyParser) starOrTest() (*pyNode, error) {
	if p.atOp("*") {
		p.next()
		e, err := p.bitOr()
		if err != nil {
			return nil, err
		}
		return &pyNode{kind: nStar, kids: []*pyNode{e}}, nil
	}
	return p.namedExpr()
}

// exprList reads assignment targets of for loops and comprehensions, stopping before `in`.
func (p *pyParser) exprList() (*pyNode, error) {
	var items []*pyNode
	for {
		var e *pyNode
		var err error
		if p.atOp("*") {
			p.next()
			e, err = p.bitOr()
			e = &pyNode{kind: nStar, kids: []*pyNode{e}}
		} else {
			e, err = p.bitOr()
		}
		if err != nil {
			return nil, err
		}
		items = append(items, e)
		if !p.atOp(",") {
			break
		}
		p.next()
		if p.atName("in") || p.atName("as") || p.atOp(":") {
			break
		}
	}
	if len(items) == 1 {
		return items[0], nil
	}
	return &pyNode{kind: nTuple, kids: items}, nil
}

func (p *pyParser) namedExpr() (*pyNode, error) {
	if p.peek().kind == pyName && p.peekAt(1).kind == pyOp && p.peekAt(1).text == ":=" {
		name := p.next().text
		p.next()
		value, err := p.test()
		if err != nil {
			return nil, err
		}
		return &pyNode{kind: nWalrus, text: name, kids: []*pyNode{value}}, nil
	}
	return p.test()
}

func (p *pyParser) test() (*pyNode, error) {
	if p.atName("lambda") {
		return p.lambda()
	}
	body, err := p.orTest()
	if err != nil {
		return nil, err
	}
	if !p.atName("if") {
		return body, nil
	}
	p.next()
	cond, err := p.orTest()
	if err != nil {
		return nil, err
	}
	if err := p.expectName("else"); err != nil {
		return nil, err
	}
	orelse, err := p.test()
	if err != nil {
		return nil, err
	}
	return &pyNode{kind: nTernary, kids: []*pyNode{body, cond, orelse}}, nil
}

func (p *pyParser) lambda() (*pyNode, error) {
	p.next()
	node := &pyNode{kind: nLambda}
	for !p.atOp(":") {
		if p.atOp("*") || p.atOp("**") || p.atOp("/") {
			p.next()
			if p.atOp(",") {
				p.next()
				continue
			}
			if p.atOp(":") {
				break
			}
		}
		if _, err := p.identifier(); err != nil {
			return nil, err
		}
		if p.atOp("=") {
			p.next()
			d, err := p.test()
			if err != nil {
				return nil, err
			}
			node.kids = append(node.kids, d)
		}
		if !p.atOp(",") {
			break
		}
		p.next()
	}
	if err := p.expectOp(":"); err != nil {
		return nil, err
	}
	body, err := p.test()
	if err != nil {
		return nil, err
	}
	node.kids = append(node.kids, body)
	return node, nil
}

func (p *pyParser) orTest() (*pyNode, error)  { return p.boolOp("or", p.andTest) }
func (p *pyParser) andTest() (*pyNode, error) { return p.boolOp("and", p.notTest) }

func (p *pyParser) boolOp(op string, operand func() (*pyNode, error)) (*pyNode, error) {
	first, err := operand()
	if err != nil {
		return nil, err
	}
	if !p.atName(op) {
		return first, nil
	}
	node := &pyNode{kind: nBool, text: op, kids: []*pyNode{first}}
	for p.atName(op) {
		p.next()
		e, err := operand()
		if err != nil {
			return nil, err
		}
		node.kids = append(node.kids, e)
	}
	return node, nil
}

func (p *pyParser) notTest() (*pyNode, error) {
	if p.atName("not") {
		p.next()
		e, err := p.notTest()
		if err != nil {
			return nil, err
		}
		return &pyNode{kind: nUnary, text: "not", kids: []*pyNode{e}}, nil
	}
	return p.comparison()
}

func (p *pyParser) compOp() bool {
	t := p.peek()
	if t.kind == pyOp {
		switch t.text {
		case "<", ">", "==", ">=", "<=", "!=":
			p.next()
			return true
		}
		return false
	}
	if t.kind != pyName {
		return false
	}
	switch {
	case t.text == "in":
		p.next()
		return true
	case t.text == "is":
		p.next()
		if p.atName("not") {
			p.next()
		}
		return true
	case t.text == "not" && p.peekAt(1).kind == pyName && p.peekAt(1).text == "in":
		p.next()
		p.next()
		return true
	}
	return false
}

func (p *pyParser) comparison() (*pyNode, error) {
	first, err := p.bitOr()
	if err != nil {
		return nil, err
	}
	var node *pyNode
	for p.compOp() {
		if node == nil {
			node = &pyNode{kind: nCmp, kids: []*pyNode{first}}
		}
		e, err := p.bitOr()
		if err != nil {
			return nil, err
		}
		node.kids = append(node.kids, e)
	}
	if node == nil {
		return first, nil
	}
	return node, nil
}

// pyBinLevels are the binary operator precedence levels, loosest first.
var pyBinLevels = [][]string{{"|"}, {"^"}, {"&"}, {"<<", ">>"}, {"+", "-"}, {"*", "/", "//", "%", "@"}}

func (p *pyParser) bitOr() (*pyNode, error) { return p.binLevel(0) }

func (p *pyParser) binLevel(level int) (*pyNode, error) {
	if level == len(pyBinLevels) {
		return p.unary()
	}
	left, err := p.binLevel(level + 1)
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		matched := false
		if t.kind == pyOp {
			for _, op := range pyBinLevels[level] {
				if t.text == op {
					matched = true
				}
			}
		}
		if !matched {
			return left, nil
		}
		p.next()
		right, err := p.binLevel(level + 1)
		if err != nil {
			return nil, err
		}
		left = &pyNode{kind: nBin, text: t.text, kids: []*pyNode{left, right}}
	}
}

func (p *pyParser) unary() (*pyNode, error) {
	if p.atOp("-") || p.atOp("+") || p.atOp("~") {
		op := p.next().text
		e, err := p.unary()
		if err != nil {
			return nil, err
		}
		return &pyNode{kind: nUnary, text: op, kids: []*pyNode{e}}, nil
	}
	return p.power()
}

func (p *pyParser) power() (*pyNode, error) {
	if p.atName("await") {
		p.next()
	}
	base, err := p.primary()
	if err != nil {
		return nil, err
	}
	if !p.atOp("**") {
		return base, nil
	}
	p.next()
	exp, err := p.unary()
	if err != nil {
		return nil, err
	}
	return &pyNode{kind: nBin, text: "**", kids: []*pyNode{base, exp}}, nil
}

func (p *pyParser) primary() (*pyNode, error) {
	node, err := p.atom()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.atOp("."):
			p.next()
			name, err := p.identifierOrKeyword()
			if err != nil {
				return nil, err
			}
			if pyDynamicAttrs[name] {
				p.prog.dynamic = true
			}
			node = &pyNode{kind: nAttr, text: name, kids: []*pyNode{node}}
		case p.atOp("("):
			if node, err = p.callArgs(node); err != nil {
				return nil, err
			}
		case p.atOp("["):
			if node, err = p.subscript(node); err != nil {
				return nil, err
			}
		default:
			return node, nil
		}
	}
}

// identifierOrKeyword reads an attribute name, which may be any word, such as `.match` or `.import_module`.
func (p *pyParser) identifierOrKeyword() (string, error) {
	t := p.peek()
	if t.kind != pyName {
		return "", errPyParse
	}
	p.next()
	return t.text, nil
}

func (p *pyParser) callArgs(callee *pyNode) (*pyNode, error) {
	p.next()
	call := &pyNode{kind: nCall, kids: []*pyNode{callee}}
	for !p.atOp(")") {
		kw := ""
		var arg *pyNode
		var err error
		switch {
		case p.atOp("*") || p.atOp("**"):
			kw = p.next().text
			arg, err = p.test()
		case p.peek().kind == pyName && p.peekAt(1).kind == pyOp && p.peekAt(1).text == "=":
			kw = p.next().text
			p.next()
			arg, err = p.test()
		default:
			arg, err = p.namedExpr()
			if err == nil && (p.atName("for") || p.atName("async")) {
				arg, err = p.comprehension(arg, nil, nTuple)
			}
		}
		if err != nil {
			return nil, err
		}
		call.kids = append(call.kids, arg)
		call.kw = append(call.kw, kw)
		if !p.atOp(",") {
			break
		}
		p.next()
	}
	return call, p.expectOp(")")
}

func (p *pyParser) subscript(obj *pyNode) (*pyNode, error) {
	p.next()
	node := &pyNode{kind: nSub, kids: []*pyNode{obj}}
	for !p.atOp("]") {
		item, err := p.sliceItem()
		if err != nil {
			return nil, err
		}
		node.kids = append(node.kids, item)
		if !p.atOp(",") {
			break
		}
		p.next()
	}
	return node, p.expectOp("]")
}

func (p *pyParser) sliceItem() (*pyNode, error) {
	slice := &pyNode{kind: nSlice}
	if !p.atOp(":") {
		e, err := p.starOrTest()
		if err != nil {
			return nil, err
		}
		if !p.atOp(":") {
			return e, nil
		}
		slice.kids = append(slice.kids, e)
	}
	for p.atOp(":") {
		p.next()
		if p.atOp(":") || p.atOp(",") || p.atOp("]") {
			continue
		}
		e, err := p.test()
		if err != nil {
			return nil, err
		}
		slice.kids = append(slice.kids, e)
	}
	return slice, nil
}

func (p *pyParser) comprehension(elt, value *pyNode, kind pyNodeKind) (*pyNode, error) {
	node := &pyNode{kind: nComp, kids: []*pyNode{elt}}
	if value != nil {
		node.kids = append(node.kids, value)
	}
	for p.atName("for") || p.atName("async") {
		if p.atName("async") {
			p.next()
		}
		p.next()
		target, err := p.exprList()
		if err != nil {
			return nil, err
		}
		if err := p.expectName("in"); err != nil {
			return nil, err
		}
		iter, err := p.orTest()
		if err != nil {
			return nil, err
		}
		loop := pyCompFor{target: target, iter: iter}
		for p.atName("if") {
			p.next()
			cond, err := p.orTestNoTernary()
			if err != nil {
				return nil, err
			}
			loop.conds = append(loop.conds, cond)
		}
		node.comp = append(node.comp, loop)
	}
	return node, nil
}

func (p *pyParser) orTestNoTernary() (*pyNode, error) {
	if p.peek().kind == pyName && p.peekAt(1).kind == pyOp && p.peekAt(1).text == ":=" {
		return p.namedExpr()
	}
	return p.orTest()
}

func (p *pyParser) yieldExpr() (*pyNode, error) {
	p.next()
	if p.atName("from") {
		p.next()
	}
	node := &pyNode{kind: nYield}
	if p.atOp(")") || p.endOfSimple() || p.atOp("=") {
		return node, nil
	}
	e, err := p.testListStar()
	if err != nil {
		return nil, err
	}
	node.kids = []*pyNode{e}
	return node, nil
}

func (p *pyParser) atom() (*pyNode, error) {
	t := p.peek()
	switch t.kind {
	case pyNumber:
		p.next()
		return &pyNode{kind: nConst}, nil
	case pyString:
		var value strings.Builder
		var names []string
		for p.at(pyString, "") {
			s := p.next()
			value.WriteString(s.text)
			names = append(names, s.names...)
		}
		p.prog.lits = append(p.prog.lits, pyLit{value: value.String(), names: names})
		return &pyNode{kind: nStr, text: value.String(), lit: len(p.prog.lits) - 1}, nil
	case pyName:
		switch t.text {
		case "True", "False", "None":
			p.next()
			return &pyNode{kind: nConst}, nil
		case "yield":
			return nil, errPyParse
		}
		if pyKeywords[t.text] {
			return nil, errPyParse
		}
		p.next()
		if pyDynamicNames[t.text] {
			p.prog.dynamic = true
		}
		return &pyNode{kind: nName, text: t.text}, nil
	case pyOp:
		switch t.text {
		case "...":
			p.next()
			return &pyNode{kind: nConst}, nil
		case "(":
			return p.parenAtom()
		case "[":
			return p.listAtom()
		case "{":
			return p.braceAtom()
		}
	}
	return nil, errPyParse
}

func (p *pyParser) parenAtom() (*pyNode, error) {
	p.next()
	if p.atOp(")") {
		p.next()
		return &pyNode{kind: nTuple}, nil
	}
	if p.atName("yield") {
		e, err := p.yieldExpr()
		if err != nil {
			return nil, err
		}
		return e, p.expectOp(")")
	}
	first, err := p.starOrTest()
	if err != nil {
		return nil, err
	}
	if p.atName("for") || p.atName("async") {
		comp, err := p.comprehension(first, nil, nTuple)
		if err != nil {
			return nil, err
		}
		return comp, p.expectOp(")")
	}
	if p.atOp(")") {
		p.next()
		return first, nil
	}
	tuple := &pyNode{kind: nTuple, kids: []*pyNode{first}}
	for p.atOp(",") {
		p.next()
		if p.atOp(")") {
			break
		}
		e, err := p.starOrTest()
		if err != nil {
			return nil, err
		}
		tuple.kids = append(tuple.kids, e)
	}
	return tuple, p.expectOp(")")
}

func (p *pyParser) listAtom() (*pyNode, error) {
	p.next()
	list := &pyNode{kind: nList}
	if p.atOp("]") {
		p.next()
		return list, nil
	}
	first, err := p.starOrTest()
	if err != nil {
		return nil, err
	}
	if p.atName("for") || p.atName("async") {
		comp, err := p.comprehension(first, nil, nList)
		if err != nil {
			return nil, err
		}
		return comp, p.expectOp("]")
	}
	list.kids = append(list.kids, first)
	for p.atOp(",") {
		p.next()
		if p.atOp("]") {
			break
		}
		e, err := p.starOrTest()
		if err != nil {
			return nil, err
		}
		list.kids = append(list.kids, e)
	}
	return list, p.expectOp("]")
}

func (p *pyParser) braceAtom() (*pyNode, error) {
	p.next()
	if p.atOp("}") {
		p.next()
		return &pyNode{kind: nDict}, nil
	}
	node := &pyNode{kind: nSet}
	for {
		if p.atOp("**") {
			p.next()
			e, err := p.bitOr()
			if err != nil {
				return nil, err
			}
			node.kind = nDict
			node.kids = append(node.kids, e)
		} else {
			key, err := p.starOrTest()
			if err != nil {
				return nil, err
			}
			if p.atOp(":") {
				p.next()
				value, err := p.test()
				if err != nil {
					return nil, err
				}
				node.kind = nDict
				if len(node.kids) == 0 && (p.atName("for") || p.atName("async")) {
					comp, err := p.comprehension(key, value, nDict)
					if err != nil {
						return nil, err
					}
					return comp, p.expectOp("}")
				}
				node.kids = append(node.kids, key, value)
			} else {
				if len(node.kids) == 0 && (p.atName("for") || p.atName("async")) {
					comp, err := p.comprehension(key, nil, nSet)
					if err != nil {
						return nil, err
					}
					return comp, p.expectOp("}")
				}
				node.kids = append(node.kids, key)
			}
		}
		if !p.atOp(",") {
			break
		}
		p.next()
		if p.atOp("}") {
			break
		}
	}
	return node, p.expectOp("}")
}
