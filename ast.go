// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

// expr is the interface implemented by every expression AST node. exprPos
// returns the node's source position for diagnostics.
type expr interface {
	exprPos() Pos
}

// posMarker is embedded by every expression node to carry its source position
// and satisfy [expr] with a single shared accessor (rather than one method per
// node kind).
type posMarker struct {
	pos Pos
}

func (m posMarker) exprPos() Pos { return m.pos }

// litExpr is a number, bool, or null literal — its value is already a final
// Ruby value (int64, float64, bool, or nil).
type litExpr struct {
	posMarker
	val Value
}

// tmplExpr is a quoted-string or heredoc template (raw inner bytes, interpreted
// at evaluation time). heredoc keeps backslashes literal; quoted strings expand
// escapes.
type tmplExpr struct {
	posMarker
	raw     string
	heredoc bool
}

// varExpr is a bare-identifier variable reference.
type varExpr struct {
	posMarker
	name string
}

// callExpr is a function call. expand marks a trailing `...` spread on the last
// argument.
type callExpr struct {
	posMarker
	name   string
	args   []expr
	expand bool
}

// attrExpr is `obj.name` attribute access.
type attrExpr struct {
	posMarker
	obj  expr
	name string
}

// indexExpr is `coll[idx]` index/element access.
type indexExpr struct {
	posMarker
	coll expr
	idx  expr
}

// unaryExpr is prefix `-` or `!`.
type unaryExpr struct {
	posMarker
	op      tokKind
	operand expr
}

// binaryExpr is any binary operator.
type binaryExpr struct {
	posMarker
	op   tokKind
	l, r expr
}

// condExpr is the `c ? t : f` conditional.
type condExpr struct {
	posMarker
	cond, then, els expr
}

// tupleExpr is `[a, b, c]`.
type tupleExpr struct {
	posMarker
	items []expr
}

// objectExpr is `{ k = v, "k2" = v2 }`. keys and vals are parallel.
type objectExpr struct {
	posMarker
	keys []expr
	vals []expr
}

// forTupleExpr is `[for v in coll : body if cond]` (keyVar optional).
type forTupleExpr struct {
	posMarker
	keyVar string // index var (empty if absent)
	valVar string
	coll   expr
	body   expr
	cond   expr // optional
}

// forObjectExpr is `{for k, v in coll : keyE => valE if cond}`. group marks a
// trailing `...` grouping mode.
type forObjectExpr struct {
	posMarker
	keyVar string
	valVar string
	coll   expr
	keyE   expr
	valE   expr
	cond   expr // optional
	group  bool
}

// Body is a parsed HCL2 configuration body: an ordered list of attributes and
// blocks, expressions left unevaluated for lazy decoding. [Parse] returns the
// document root *Body.
type Body struct {
	Attributes []*Attribute
	Blocks     []*Block
}

// Attribute is a `name = expr` body entry. Its expression is stored unevaluated.
type Attribute struct {
	Name string
	expr expr
	Pos  Pos
}

// Block is a `type "label"... { ... }` body entry with a nested [*Body].
type Block struct {
	Type   string
	Labels []string
	Body   *Body
	Pos    Pos
}
