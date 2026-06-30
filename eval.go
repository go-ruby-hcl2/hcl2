// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import "strconv"

// Func is a function callable from HCL expressions. It receives already-evaluated
// arguments (with any trailing `...` spread already flattened) and returns a Ruby
// value or an error. Returning a [*Diagnostic] preserves source position.
type Func func(args []Value) (Value, error)

// Context carries the variables and functions an expression may reference. The
// zero value is usable; use [NewContext] for an initialised one. Variables map
// bare identifiers to Ruby values; Functions map call names to [Func].
type Context struct {
	Variables map[string]Value
	Functions map[string]Func
}

// NewContext returns an empty initialised [*Context].
func NewContext() *Context {
	return &Context{Variables: map[string]Value{}, Functions: map[string]Func{}}
}

// evaluator walks an AST against a context plus a stack of for/loop scopes.
type evaluator struct {
	ctx    *Context
	scopes []map[string]Value
}

func newEvaluator(ctx *Context) *evaluator {
	if ctx == nil {
		ctx = NewContext()
	}
	return &evaluator{ctx: ctx}
}

// lookupVar resolves a bare identifier through the loop scopes (innermost first)
// then the context variables.
func (ev *evaluator) lookupVar(name string) (Value, bool) {
	for i := len(ev.scopes) - 1; i >= 0; i-- {
		if v, ok := ev.scopes[i][name]; ok {
			return v, true
		}
	}
	if ev.ctx.Variables != nil {
		if v, ok := ev.ctx.Variables[name]; ok {
			return v, true
		}
	}
	return nil, false
}

// eval evaluates an expression node to a Ruby value.
func (ev *evaluator) eval(e expr) (Value, *Diagnostic) {
	switch n := e.(type) {
	case *litExpr:
		return n.val, nil
	case *tmplExpr:
		return ev.evalTemplate(n.raw, n.heredoc, n.pos)
	case *varExpr:
		v, ok := ev.lookupVar(n.name)
		if !ok {
			return nil, &Diagnostic{Summary: "unknown variable \"" + n.name + "\"", Pos: n.pos}
		}
		return v, nil
	case *attrExpr:
		return ev.evalAttr(n)
	case *indexExpr:
		return ev.evalIndex(n)
	case *unaryExpr:
		return ev.evalUnary(n)
	case *binaryExpr:
		return ev.evalBinary(n)
	case *condExpr:
		return ev.evalCond(n)
	case *tupleExpr:
		return ev.evalTuple(n)
	case *objectExpr:
		return ev.evalObject(n)
	case *callExpr:
		return ev.evalCall(n)
	case *forTupleExpr:
		return ev.evalForTuple(n)
	default: // *forObjectExpr — the final node kind
		return ev.evalForObject(e.(*forObjectExpr))
	}
}

func (ev *evaluator) evalAttr(n *attrExpr) (Value, *Diagnostic) {
	obj, d := ev.eval(n.obj)
	if d != nil {
		return nil, d
	}
	m, ok := obj.(*Map)
	if !ok {
		return nil, &Diagnostic{Summary: "cannot access attribute \"" + n.name + "\" on a non-object", Pos: n.pos}
	}
	v, ok := m.Get(n.name)
	if !ok {
		return nil, &Diagnostic{Summary: "object has no attribute \"" + n.name + "\"", Pos: n.pos}
	}
	return v, nil
}

func (ev *evaluator) evalIndex(n *indexExpr) (Value, *Diagnostic) {
	coll, d := ev.eval(n.coll)
	if d != nil {
		return nil, d
	}
	idx, d := ev.eval(n.idx)
	if d != nil {
		return nil, d
	}
	switch c := coll.(type) {
	case []Value:
		i, d := indexInt(idx, n.pos)
		if d != nil {
			return nil, d
		}
		if i < 0 || i >= int64(len(c)) {
			return nil, &Diagnostic{Summary: "tuple index out of range", Pos: n.pos}
		}
		return c[i], nil
	case *Map:
		key, d := indexKey(idx, n.pos)
		if d != nil {
			return nil, d
		}
		v, ok := c.Get(key)
		if !ok {
			return nil, &Diagnostic{Summary: "object has no element \"" + key + "\"", Pos: n.pos}
		}
		return v, nil
	}
	return nil, &Diagnostic{Summary: "cannot index a value of this type", Pos: n.pos}
}

func indexInt(v Value, pos Pos) (int64, *Diagnostic) {
	switch x := v.(type) {
	case int64:
		return x, nil
	case float64:
		if x == float64(int64(x)) {
			return int64(x), nil
		}
	}
	return 0, &Diagnostic{Summary: "tuple index must be a whole number", Pos: pos}
}

func indexKey(v Value, pos Pos) (string, *Diagnostic) {
	switch x := v.(type) {
	case string:
		return x, nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	}
	return "", &Diagnostic{Summary: "object key must be a string", Pos: pos}
}

func (ev *evaluator) evalUnary(n *unaryExpr) (Value, *Diagnostic) {
	v, d := ev.eval(n.operand)
	if d != nil {
		return nil, d
	}
	if n.op == tMinus {
		switch x := v.(type) {
		case int64:
			return -x, nil
		case float64:
			return -x, nil
		}
		return nil, &Diagnostic{Summary: "unary '-' requires a number", Pos: n.pos}
	}
	// tBang
	b, ok := v.(bool)
	if !ok {
		return nil, &Diagnostic{Summary: "unary '!' requires a bool", Pos: n.pos}
	}
	return !b, nil
}

func (ev *evaluator) evalCond(n *condExpr) (Value, *Diagnostic) {
	cv, d := ev.eval(n.cond)
	if d != nil {
		return nil, d
	}
	b, ok := cv.(bool)
	if !ok {
		return nil, &Diagnostic{Summary: "conditional predicate must be a bool", Pos: n.pos}
	}
	if b {
		return ev.eval(n.then)
	}
	return ev.eval(n.els)
}

func (ev *evaluator) evalTuple(n *tupleExpr) (Value, *Diagnostic) {
	out := make([]Value, 0, len(n.items))
	for _, it := range n.items {
		v, d := ev.eval(it)
		if d != nil {
			return nil, d
		}
		out = append(out, v)
	}
	return out, nil
}

func (ev *evaluator) evalObject(n *objectExpr) (Value, *Diagnostic) {
	m := NewMap()
	for i, ke := range n.keys {
		kv, d := ev.eval(ke)
		if d != nil {
			return nil, d
		}
		key, d := indexKey(kv, ke.exprPos())
		if d != nil {
			return nil, d
		}
		vv, d := ev.eval(n.vals[i])
		if d != nil {
			return nil, d
		}
		m.Set(key, vv)
	}
	return m, nil
}

// pushScope binds loop variables for the duration of fn.
func (ev *evaluator) pushScope(scope map[string]Value, fn func() *Diagnostic) *Diagnostic {
	ev.scopes = append(ev.scopes, scope)
	d := fn()
	ev.scopes = ev.scopes[:len(ev.scopes)-1]
	return d
}

// iterateCollection iterates a tuple or object, binding valVar (and keyVar if
// non-empty) per element and invoking fn within a fresh scope.
func (ev *evaluator) iterateCollection(coll Value, keyVar, valVar string, pos Pos, fn func() *Diagnostic) *Diagnostic {
	switch c := coll.(type) {
	case []Value:
		for i, v := range c {
			scope := map[string]Value{valVar: v}
			if keyVar != "" {
				scope[keyVar] = int64(i)
			}
			if d := ev.pushScope(scope, fn); d != nil {
				return d
			}
		}
		return nil
	case *Map:
		for _, pr := range c.Pairs() {
			scope := map[string]Value{}
			if keyVar != "" {
				scope[keyVar] = pr.Key
				scope[valVar] = pr.Val
			} else {
				scope[valVar] = pr.Val
			}
			if d := ev.pushScope(scope, fn); d != nil {
				return d
			}
		}
		return nil
	}
	return &Diagnostic{Summary: "for-expression requires a tuple or object", Pos: pos}
}

func (ev *evaluator) evalForTuple(n *forTupleExpr) (Value, *Diagnostic) {
	coll, d := ev.eval(n.coll)
	if d != nil {
		return nil, d
	}
	out := []Value{}
	d = ev.iterateCollection(coll, n.keyVar, n.valVar, n.pos, func() *Diagnostic {
		if n.cond != nil {
			keep, d := ev.evalBoolCond(n.cond)
			if d != nil {
				return d
			}
			if !keep {
				return nil
			}
		}
		v, d := ev.eval(n.body)
		if d != nil {
			return d
		}
		out = append(out, v)
		return nil
	})
	if d != nil {
		return nil, d
	}
	return out, nil
}

func (ev *evaluator) evalForObject(n *forObjectExpr) (Value, *Diagnostic) {
	coll, d := ev.eval(n.coll)
	if d != nil {
		return nil, d
	}
	m := NewMap()
	// grouped holds, per key, the accumulated tuple in grouping mode.
	grouped := map[string][]Value{}
	d = ev.iterateCollection(coll, n.keyVar, n.valVar, n.pos, func() *Diagnostic {
		if n.cond != nil {
			keep, d := ev.evalBoolCond(n.cond)
			if d != nil {
				return d
			}
			if !keep {
				return nil
			}
		}
		kv, d := ev.eval(n.keyE)
		if d != nil {
			return d
		}
		key, d := indexKey(kv, n.keyE.exprPos())
		if d != nil {
			return d
		}
		vv, d := ev.eval(n.valE)
		if d != nil {
			return d
		}
		if n.group {
			grouped[key] = append(grouped[key], vv)
			if _, seen := m.Get(key); !seen {
				m.Set(key, nil) // reserve order slot
			}
			return nil
		}
		if _, dup := m.Get(key); dup {
			return &Diagnostic{Summary: "duplicate object key \"" + key + "\" in for-expression", Pos: n.pos}
		}
		m.Set(key, vv)
		return nil
	})
	if d != nil {
		return nil, d
	}
	if n.group {
		for _, pr := range m.Pairs() {
			m.Set(pr.Key, grouped[pr.Key])
		}
	}
	return m, nil
}

// evalBoolCond evaluates an expression that must yield a bool.
func (ev *evaluator) evalBoolCond(e expr) (bool, *Diagnostic) {
	v, d := ev.eval(e)
	if d != nil {
		return false, d
	}
	b, ok := v.(bool)
	if !ok {
		return false, &Diagnostic{Summary: "condition must be a bool", Pos: e.exprPos()}
	}
	return b, nil
}

// docToMap evaluates a parsed body into a Ruby Hash. Blocks of the same type
// collect; a single-labelled block nests under type then label.
func (ev *evaluator) docToMap(b *Body) (*Map, *Diagnostic) {
	m := NewMap()
	for _, a := range b.Attributes {
		v, d := ev.eval(a.expr)
		if d != nil {
			return nil, d
		}
		m.Set(a.Name, v)
	}
	for _, blk := range b.Blocks {
		inner, d := ev.docToMap(blk.Body)
		if d != nil {
			return nil, d
		}
		labelled := inner
		// Nest under each label, innermost last.
		for i := len(blk.Labels) - 1; i >= 0; i-- {
			wrap := NewMap()
			wrap.Set(blk.Labels[i], labelled)
			labelled = wrap
		}
		mergeBlock(m, blk.Type, labelled, len(blk.Labels) > 0)
	}
	return m, nil
}

// mergeBlock inserts a block's value under its type, collecting repeated blocks
// into a tuple (HCL's "blocks of the same type collect") while merging labelled
// blocks that share a type into one object.
func mergeBlock(m *Map, typ string, val *Map, labelled bool) {
	existing, ok := m.Get(typ)
	if !ok {
		if labelled {
			m.Set(typ, val)
		} else {
			m.Set(typ, val)
		}
		return
	}
	if labelled {
		// Merge the label maps so `t "a" {}` and `t "b" {}` coexist.
		if em, ok := existing.(*Map); ok {
			for _, pr := range val.Pairs() {
				em.Set(pr.Key, pr.Val)
			}
			return
		}
	}
	// Unlabelled repeats (or a type used both ways) accumulate into a tuple.
	switch ex := existing.(type) {
	case []Value:
		m.Set(typ, append(ex, val))
	default:
		m.Set(typ, []Value{ex, val})
	}
}
