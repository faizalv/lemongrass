// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package guard

import "strings"

// pyAccess is what code does with the path a string literal holds, ordered so a larger one wins.
type pyAccess int

const (
	accessNone pyAccess = iota
	accessRead
	// accessWrite covers writes, deletions, running a command and any call the guard does not know.
	accessWrite
)

// pyVal is what an expression may hold: the guarded literals whose text reaches it, and whether it is a pathlib path,
// whose methods read and write files where a string's do not.
type pyVal struct {
	lits []int
	path bool
}

func (v pyVal) union(o pyVal) pyVal {
	out := pyVal{path: v.path || o.path, lits: v.lits}
	for _, l := range o.lits {
		if !containsInt(out.lits, l) {
			out.lits = append(append([]int(nil), out.lits...), l)
		}
	}
	return out
}

func (v pyVal) plain() pyVal { return pyVal{lits: v.lits} }

func containsInt(list []int, n int) bool {
	for _, x := range list {
		if x == n {
			return true
		}
	}
	return false
}

type pyScope map[string]pyVal

type pyFrame struct {
	def    *pyDef
	locals pyScope
}

// pyFlow follows guarded literals through a parsed program to the calls that use them. Names are tracked without
// order (a name holds everything ever assigned to it), each function has one scope for all its calls, and the whole
// program is evaluated again until nothing grows, so a value reaches every use whichever line comes first.
type pyFlow struct {
	prog     *pyProgram
	guarded  []bool
	access   []pyAccess
	module   pyScope
	frames   map[*pyDef]*pyFrame
	returns  map[*pyDef]pyVal
	defsByNm map[string][]*pyDef
	changed  bool
}

const pyFlowRounds = 32

// pyAccesses returns, per literal, the strongest access the program makes to it, or ok false when the program
// cannot be followed and the caller must keep today's scan.
func pyAccesses(prog *pyProgram, guarded []bool) (access []pyAccess, ok bool) {
	access, _, ok = pyAccessRounds(prog, guarded)
	return access, ok
}

// pyAccessRounds is pyAccesses that also says how many rounds the flow took to settle.
func pyAccessRounds(prog *pyProgram, guarded []bool) (access []pyAccess, rounds int, ok bool) {
	if prog.dynamic {
		return nil, 0, false
	}
	f := &pyFlow{prog: prog, guarded: guarded, access: make([]pyAccess, len(prog.lits)), module: pyScope{},
		frames: map[*pyDef]*pyFrame{}, returns: map[*pyDef]pyVal{}, defsByNm: map[string][]*pyDef{}}
	for _, def := range prog.defs {
		f.frames[def] = &pyFrame{def: def, locals: pyScope{}}
		f.defsByNm[def.name] = append(f.defsByNm[def.name], def)
	}
	for round := 0; round < pyFlowRounds; round++ {
		f.changed = false
		f.block(prog.stmts, nil)
		for _, def := range prog.defs {
			frame := f.frames[def]
			for name, d := range def.defaults {
				f.bind(frame, name, f.eval(d, nil))
			}
			f.block(def.body, frame)
		}
		if !f.changed {
			return f.access, round + 1, true
		}
	}
	return nil, pyFlowRounds, false
}

func (f *pyFlow) record(v pyVal, kind pyAccess) {
	for _, l := range v.lits {
		if f.access[l] < kind {
			f.access[l] = kind
		}
	}
}

func (f *pyFlow) lookup(frame *pyFrame, name string) pyVal {
	if frame != nil {
		if v, ok := frame.locals[name]; ok {
			return v.union(f.module[name])
		}
	}
	return f.module[name]
}

func (f *pyFlow) bind(frame *pyFrame, name string, v pyVal) {
	if len(v.lits) == 0 && !v.path {
		return
	}
	scope := f.module
	if frame != nil {
		scope = frame.locals
	}
	old := scope[name]
	merged := old.union(v)
	if len(merged.lits) != len(old.lits) || merged.path != old.path {
		scope[name] = merged
		f.changed = true
	}
}

// bindTarget stores a value in an assignment target. An attribute target is stored under its dotted text, and an
// item of a container makes the container hold the value.
func (f *pyFlow) bindTarget(t *pyNode, v pyVal, frame *pyFrame) {
	switch t.kind {
	case nName:
		f.bind(frame, t.text, v)
	case nAttr:
		if dotted := pyDotted(t); dotted != "" {
			f.bind(frame, dotted, v)
		}
		f.eval(t.kids[0], frame)
	case nSub:
		for _, k := range t.kids[1:] {
			f.eval(k, frame)
		}
		f.bindTarget(t.kids[0], v, frame)
	case nTuple, nList:
		for _, k := range t.kids {
			f.bindTarget(k, v, frame)
		}
	case nStar:
		f.bindTarget(t.kids[0], v, frame)
	default:
		f.eval(t, frame)
	}
}

func (f *pyFlow) block(stmts []*pyStmt, frame *pyFrame) {
	for _, s := range stmts {
		for _, e := range s.exprs {
			f.eval(e, frame)
		}
		switch s.kind {
		case sAssign:
			v := f.eval(s.value, frame)
			for _, t := range s.targets {
				f.bindTarget(t, v, frame)
			}
		case sAugAssign:
			v := f.eval(s.value, frame).union(f.eval(s.targets[0], frame))
			f.bindTarget(s.targets[0], v, frame)
		case sFor:
			f.bindTarget(s.targets[0], f.eval(s.value, frame), frame)
		case sWith:
			v := f.eval(s.value, frame)
			for _, t := range s.targets {
				f.bindTarget(t, v, frame)
			}
		case sReturn:
			if frame != nil {
				f.addReturn(frame.def, f.eval(s.value, frame))
			}
		}
		for _, body := range s.bodies {
			f.block(body, frame)
		}
	}
}

func (f *pyFlow) addReturn(def *pyDef, v pyVal) {
	old := f.returns[def]
	merged := old.union(v)
	if len(merged.lits) != len(old.lits) || merged.path != old.path {
		f.returns[def] = merged
		f.changed = true
	}
}

// pyDotted is the dotted text of a name or an attribute chain of names, or "" for anything else.
func pyDotted(n *pyNode) string {
	switch n.kind {
	case nName:
		return n.text
	case nAttr:
		if base := pyDotted(n.kids[0]); base != "" {
			return base + "." + n.text
		}
	}
	return ""
}

// qualified resolves a callee through the program's imports: `osp.join` after `import os.path as osp` is
// os.path.join. A dotted name whose root is not imported is a variable, and gives "".
func (f *pyFlow) qualified(n *pyNode) string {
	dotted := pyDotted(n)
	if dotted == "" {
		return ""
	}
	root, rest, _ := strings.Cut(dotted, ".")
	module, imported := f.prog.aliases[root]
	if !imported {
		if rest == "" {
			return root
		}
		return ""
	}
	if rest == "" {
		return module
	}
	return module + "." + rest
}

func (f *pyFlow) eval(n *pyNode, frame *pyFrame) pyVal {
	if n == nil {
		return pyVal{}
	}
	switch n.kind {
	case nStr:
		v := pyVal{}
		if f.guarded[n.lit] {
			v.lits = []int{n.lit}
		}
		for _, name := range f.prog.lits[n.lit].names {
			v = v.union(f.lookup(frame, name).plain())
		}
		return v
	case nName:
		return f.lookup(frame, n.text)
	case nConst:
		return pyVal{}
	case nAttr:
		v := f.eval(n.kids[0], frame)
		if dotted := pyDotted(n); dotted != "" {
			v = v.union(f.lookup(frame, dotted))
		}
		switch n.text {
		case "parent", "parents", "anchor":
			return v
		}
		return v.plain()
	case nSub:
		v := f.eval(n.kids[0], frame)
		for _, k := range n.kids[1:] {
			f.eval(k, frame)
		}
		return v
	case nCall:
		return f.call(n, frame)
	case nBin:
		l, r := f.eval(n.kids[0], frame), f.eval(n.kids[1], frame)
		v := l.union(r)
		v.path = n.text == "/" && (l.path || r.path)
		return v
	case nBool, nList, nTuple, nSet, nDict, nStar, nYield, nSlice:
		var v pyVal
		for _, k := range n.kids {
			v = v.union(f.eval(k, frame))
		}
		return v
	case nUnary:
		v := f.eval(n.kids[0], frame)
		if n.text == "not" {
			return pyVal{}
		}
		return v
	case nCmp:
		for _, k := range n.kids {
			f.eval(k, frame)
		}
		return pyVal{}
	case nTernary:
		f.eval(n.kids[1], frame)
		return f.eval(n.kids[0], frame).union(f.eval(n.kids[2], frame))
	case nWalrus:
		v := f.eval(n.kids[0], frame)
		f.bind(frame, n.text, v)
		return v
	case nComp:
		for _, loop := range n.comp {
			f.bindTarget(loop.target, f.eval(loop.iter, frame), frame)
			for _, c := range loop.conds {
				f.eval(c, frame)
			}
		}
		var v pyVal
		for _, k := range n.kids {
			v = v.union(f.eval(k, frame))
		}
		return v
	case nLambda:
		// A lambda's parameters and callers are not followed, so whatever its body reaches counts as used by a call
		// the guard does not know.
		var v pyVal
		for _, k := range n.kids {
			v = v.union(f.eval(k, frame))
		}
		f.record(v, accessWrite)
		return v
	}
	return pyVal{}
}

func (f *pyFlow) call(n *pyNode, frame *pyFrame) pyVal {
	callee := n.kids[0]
	args := make([]pyVal, len(n.kids)-1)
	for i, a := range n.kids[1:] {
		args[i] = f.eval(a, frame)
	}
	call := pyCall{node: n, args: args, kw: n.kw}

	if callee.kind == nName {
		if defs := f.defsByNm[callee.text]; len(defs) > 0 {
			if _, imported := f.prog.aliases[callee.text]; !imported {
				return f.callDefs(defs, call, false)
			}
		}
	}
	if q := f.qualified(callee); q != "" {
		if spec, ok := pyFuncs[q]; ok {
			return f.apply(spec, call, frame)
		}
		if callee.kind == nName || f.prog.aliases[strings.SplitN(pyDotted(callee), ".", 2)[0]] != "" {
			return f.unknown(call, pyVal{})
		}
	}
	if callee.kind != nAttr {
		f.eval(callee, frame)
		return f.unknown(call, pyVal{})
	}
	recv := f.eval(callee.kids[0], frame)
	method := callee.text
	var out pyVal
	handled := false
	if defs := f.methodDefs(method); len(defs) > 0 {
		out = f.callDefs(defs, call, true)
		handled = true
	}
	if recv.path {
		if spec, ok := pyPathMethods[method]; ok {
			return out.union(f.applyPathMethod(spec, recv, call))
		}
		return out.union(f.unknown(call, recv))
	}
	if spec, ok := pyMethods[method]; ok {
		return out.union(f.applyMethod(spec, callee.kids[0], recv, call, frame))
	}
	if handled {
		return out
	}
	return f.unknown(call, recv)
}

func (f *pyFlow) methodDefs(name string) []*pyDef {
	var out []*pyDef
	for _, d := range f.defsByNm[name] {
		if d.method {
			out = append(out, d)
		}
	}
	return out
}

type pyCall struct {
	node *pyNode
	args []pyVal
	kw   []string
}

// arg returns the value of the argument at position index or with keyword name, and whether one was given.
func (c pyCall) arg(index int, name string) (pyVal, *pyNode, bool) {
	pos := 0
	for i, kw := range c.kw {
		switch {
		case kw == "" && pos == index:
			return c.args[i], c.node.kids[i+1], true
		case kw == "":
			pos++
		case name != "" && kw == name:
			return c.args[i], c.node.kids[i+1], true
		}
	}
	return pyVal{}, nil, false
}

func (c pyCall) all() pyVal {
	var v pyVal
	for _, a := range c.args {
		v = v.union(a)
	}
	return v
}

// unknown is a call the guard has no entry for: every guarded literal it is given may be a path it opens.
func (f *pyFlow) unknown(call pyCall, recv pyVal) pyVal {
	v := call.all().union(recv)
	f.record(v, accessWrite)
	return v.plain()
}

// callDefs binds a call's arguments to the parameters of the program's own functions of that name and returns what
// they return.
func (f *pyFlow) callDefs(defs []*pyDef, call pyCall, viaAttr bool) pyVal {
	var out pyVal
	for _, def := range defs {
		frame := f.frames[def]
		params := def.params
		if viaAttr && def.method && len(params) > 0 {
			params = params[1:]
		}
		pos := 0
		for i, kw := range call.kw {
			v := call.args[i]
			switch kw {
			case "":
				if pos < len(params) {
					f.bind(frame, params[pos], v)
				} else if def.varArgs != "" {
					f.bind(frame, def.varArgs, v)
				} else {
					f.record(v, accessWrite)
				}
				pos++
			case "*":
				for _, p := range params[min(pos, len(params)):] {
					f.bind(frame, p, v)
				}
				if def.varArgs != "" {
					f.bind(frame, def.varArgs, v)
				}
			case "**":
				for _, p := range params {
					f.bind(frame, p, v)
				}
				if def.varKw != "" {
					f.bind(frame, def.varKw, v)
				}
			default:
				if containsString(params, kw) {
					f.bind(frame, kw, v)
				} else if def.varKw != "" {
					f.bind(frame, def.varKw, v)
				} else {
					f.record(v, accessWrite)
				}
			}
		}
		out = out.union(f.returns[def])
	}
	return out
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (f *pyFlow) apply(spec pyFuncSpec, call pyCall, frame *pyFrame) pyVal {
	if spec.kind != accessNone {
		kind := spec.kind
		if spec.mode != nil {
			kind = spec.mode(call)
		}
		if len(spec.paths) == 0 {
			f.record(call.all(), kind)
		}
		for _, p := range spec.paths {
			if v, _, ok := call.arg(p.index, p.name); ok {
				k := kind
				if p.kind != accessNone {
					k = p.kind
				}
				f.record(v, k)
			}
		}
	}
	switch spec.ret {
	case retArgs:
		return call.all().plain()
	case retPath:
		v := call.all()
		v.path = true
		return v
	}
	return pyVal{}
}

func (f *pyFlow) applyPathMethod(spec pyFuncSpec, recv pyVal, call pyCall) pyVal {
	kind := spec.kind
	if spec.mode != nil {
		kind = spec.mode(call)
	}
	f.record(recv, kind)
	for _, p := range spec.paths {
		if v, _, ok := call.arg(p.index, p.name); ok {
			f.record(v, p.kind)
		}
	}
	switch spec.ret {
	case retArgs:
		return recv.union(call.all()).plain()
	case retPath:
		v := recv.union(call.all())
		v.path = true
		return v
	}
	return pyVal{}
}

func (f *pyFlow) applyMethod(spec pyMethodSpec, recvNode *pyNode, recv pyVal, call pyCall, frame *pyFrame) pyVal {
	switch spec {
	case methodCarry:
		return recv.union(call.all()).plain()
	case methodStore:
		f.bindTarget(recvNode, call.all(), frame)
		return pyVal{}
	case methodRecv:
		return recv.plain()
	}
	return pyVal{}
}
