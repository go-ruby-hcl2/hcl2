// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

// evalCall evaluates a function call. `try` and `can` are special forms whose
// arguments are evaluated lazily with errors suppressed; everything else has its
// arguments evaluated (with `...` spread flattened) before dispatch to a
// caller-supplied function or a built-in.
func (ev *evaluator) evalCall(n *callExpr) (Value, *Diagnostic) {
	switch n.name {
	case "try":
		return ev.evalTry(n)
	case "can":
		return ev.evalCan(n)
	}
	args, d := ev.evalArgs(n)
	if d != nil {
		return nil, d
	}
	if ev.ctx.Functions != nil {
		if fn, ok := ev.ctx.Functions[n.name]; ok {
			v, err := fn(args)
			if err != nil {
				return nil, toDiag(err, n.pos)
			}
			return v, nil
		}
	}
	if fn, ok := builtins[n.name]; ok {
		v, err := fn(args)
		if err != nil {
			return nil, toDiag(err, n.pos)
		}
		return v, nil
	}
	return nil, &Diagnostic{Summary: "call to unknown function \"" + n.name + "\"", Pos: n.pos}
}

// evalArgs evaluates the argument expressions, flattening a trailing `...`
// spread (which must evaluate to a tuple).
func (ev *evaluator) evalArgs(n *callExpr) ([]Value, *Diagnostic) {
	args := make([]Value, 0, len(n.args))
	for i, ae := range n.args {
		v, d := ev.eval(ae)
		if d != nil {
			return nil, d
		}
		if n.expand && i == len(n.args)-1 {
			tup, ok := v.([]Value)
			if !ok {
				return nil, &Diagnostic{Summary: "the spread '...' argument must be a tuple", Pos: n.pos}
			}
			args = append(args, tup...)
			continue
		}
		args = append(args, v)
	}
	return args, nil
}

// evalTry returns the first argument that evaluates without error.
func (ev *evaluator) evalTry(n *callExpr) (Value, *Diagnostic) {
	if len(n.args) == 0 {
		return nil, &Diagnostic{Summary: "try() requires at least one argument", Pos: n.pos}
	}
	var last *Diagnostic
	for _, ae := range n.args {
		v, d := ev.eval(ae)
		if d == nil {
			return v, nil
		}
		last = d
	}
	return nil, last
}

// evalCan reports whether its single argument evaluates without error.
func (ev *evaluator) evalCan(n *callExpr) (Value, *Diagnostic) {
	if len(n.args) != 1 {
		return nil, &Diagnostic{Summary: "can() requires exactly one argument", Pos: n.pos}
	}
	_, d := ev.eval(n.args[0])
	return d == nil, nil
}

// toDiag adapts an error returned by a function to a positioned [*Diagnostic].
func toDiag(err error, pos Pos) *Diagnostic {
	if d, ok := err.(*Diagnostic); ok {
		return d
	}
	return &Diagnostic{Summary: err.Error(), Pos: pos}
}
