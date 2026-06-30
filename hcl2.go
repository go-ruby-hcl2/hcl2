// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import "os"

// Parse parses an HCL2 native-syntax document into a lazy [*Body] without
// evaluating any expression, mirroring hclsyntax.ParseConfig. A syntax error is
// returned as [Diagnostics].
func Parse(src string) (*Body, error) {
	p := newParser(src)
	body, d := p.parseDocument()
	if d != nil {
		return nil, Diagnostics{d}
	}
	return body, nil
}

// Eval parses src and evaluates it against ctx, returning the document as a Ruby
// Hash (an insertion-ordered [*Map]). A nil ctx is treated as an empty context.
// A syntax or evaluation error is returned as [Diagnostics].
func Eval(src string, ctx *Context) (*Map, error) {
	body, err := Parse(src)
	if err != nil {
		return nil, err
	}
	ev := newEvaluator(ctx)
	m, d := ev.docToMap(body)
	if d != nil {
		return nil, Diagnostics{d}
	}
	return m, nil
}

// EvalExpr parses and evaluates a single expression string against ctx, the
// standalone-expression entry point a host uses for one-off interpolations.
func EvalExpr(src string, ctx *Context) (Value, error) {
	p := newParser(src)
	e, d := p.parseExpr()
	if d != nil {
		return nil, Diagnostics{d}
	}
	if p.tok.kind != tEOF {
		return nil, Diagnostics{&Diagnostic{Summary: "unexpected trailing tokens", Pos: p.tok.pos}}
	}
	ev := newEvaluator(ctx)
	v, d := ev.eval(e)
	if d != nil {
		return nil, Diagnostics{d}
	}
	return v, nil
}

// EvalFile reads path and evaluates it as an HCL2 document against ctx.
func EvalFile(path string, ctx *Context) (*Map, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Eval(string(b), ctx)
}

// Attr returns the unevaluated expression source position and evaluates an
// attribute by name against ctx, supporting HCL's lazy per-attribute decoding.
// It reports whether the attribute exists.
func (b *Body) Attr(name string, ctx *Context) (Value, bool, error) {
	for _, a := range b.Attributes {
		if a.Name == name {
			ev := newEvaluator(ctx)
			v, d := ev.eval(a.expr)
			if d != nil {
				return nil, true, Diagnostics{d}
			}
			return v, true, nil
		}
	}
	return nil, false, nil
}

// interpretStringKey interprets a quoted block label (escapes expanded, no
// interpolation evaluated — labels are static).
func interpretStringKey(raw string, pos Pos) (string, *Diagnostic) {
	ev := newEvaluator(nil)
	return ev.evalTemplate(raw, false, pos)
}
