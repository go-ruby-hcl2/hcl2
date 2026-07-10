// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

// This file exposes a read-only, exported view of the HCL2 expression AST.
//
// The parser keeps its native expression nodes ([litExpr], [binaryExpr], …)
// unexported so that evaluation stays an internal concern. Transpiler and
// code-generation front-ends, however, need to read the *structure* of an
// expression rather than its evaluated result. [Attribute.Expression] converts
// the private tree into the exported [Expr] tree defined here; the private
// nodes and the [Eval] path are left untouched.
//
// [Expr] is a sealed interface: its marker method [Expr.exprNode] is
// unexported, so the only implementations are the concrete node types declared
// in this package. A consumer may type-switch over them exhaustively.

// Expr is the exported HCL2 expression AST: a read-only view intended for
// transpilers and other tools that inspect expression structure. It is sealed —
// only the concrete node types in this package implement it.
type Expr interface{ exprNode() }

// LiteralExpr is a number, bool, or null literal. Value is already a final Ruby
// value (int64, float64, bool, or nil).
type LiteralExpr struct{ Value Value }

// TemplateExpr is a quoted-string or heredoc template. Raw holds the raw inner
// bytes (interpreted at evaluation time); Heredoc is true for a heredoc.
type TemplateExpr struct {
	Raw     string
	Heredoc bool
}

// VarExpr is a bare-identifier variable reference.
type VarExpr struct{ Name string }

// CallExpr is a function call. Expand marks a trailing `...` spread on the last
// argument.
type CallExpr struct {
	Name   string
	Args   []Expr
	Expand bool
}

// AttrExpr is `Obj.Name` attribute access.
type AttrExpr struct {
	Obj  Expr
	Name string
}

// IndexExpr is `Coll[Idx]` index/element access.
type IndexExpr struct{ Coll, Idx Expr }

// UnaryExpr is prefix `-` or `!`. Op is the operator source text.
type UnaryExpr struct {
	Op      string
	Operand Expr
}

// BinaryExpr is any binary operator. Op is the operator source text, e.g. "+",
// "==", "&&", "<=".
type BinaryExpr struct {
	Op          string
	Left, Right Expr
}

// CondExpr is the `Cond ? Then : Else` conditional.
type CondExpr struct{ Cond, Then, Else Expr }

// TupleExpr is `[a, b, c]`.
type TupleExpr struct{ Items []Expr }

// ObjectExpr is `{ k = v, "k2" = v2 }`. Keys and Vals are parallel slices.
type ObjectExpr struct{ Keys, Vals []Expr }

// ForTupleExpr is `[for ValVar in Coll : Body if Cond]`. KeyVar is the optional
// index variable (empty if absent) and Cond may be nil.
type ForTupleExpr struct {
	KeyVar, ValVar string
	Coll, Body     Expr
	Cond           Expr // may be nil
}

// ForObjectExpr is `{for KeyVar, ValVar in Coll : KeyExpr => ValExpr if Cond}`.
// KeyVar may be empty, Cond may be nil, and Group marks a trailing `...`
// grouping mode.
type ForObjectExpr struct {
	KeyVar, ValVar         string
	Coll, KeyExpr, ValExpr Expr
	Cond                   Expr // may be nil
	Group                  bool
}

func (LiteralExpr) exprNode()   {}
func (TemplateExpr) exprNode()  {}
func (VarExpr) exprNode()       {}
func (CallExpr) exprNode()      {}
func (AttrExpr) exprNode()      {}
func (IndexExpr) exprNode()     {}
func (UnaryExpr) exprNode()     {}
func (BinaryExpr) exprNode()    {}
func (CondExpr) exprNode()      {}
func (TupleExpr) exprNode()     {}
func (ObjectExpr) exprNode()    {}
func (ForTupleExpr) exprNode()  {}
func (ForObjectExpr) exprNode() {}

// opText maps an operator token kind to its canonical source text. Every
// operator the parser can place on a [unaryExpr] or [binaryExpr] node has an
// entry, so the exported [UnaryExpr.Op] / [BinaryExpr.Op] strings are stable.
var opText = map[tokKind]string{
	tPlus:    "+",
	tMinus:   "-",
	tStar:    "*",
	tSlash:   "/",
	tPercent: "%",
	tEq:      "==",
	tNe:      "!=",
	tLt:      "<",
	tLe:      "<=",
	tGt:      ">",
	tGe:      ">=",
	tAnd:     "&&",
	tOr:      "||",
	tBang:    "!",
}

// Expression returns the attribute's parsed expression as an exported [Expr]
// tree. The result is a fresh read-only view; mutating it does not affect the
// attribute or its evaluation.
func (a *Attribute) Expression() Expr { return toExpr(a.expr) }

// toExpr recursively converts a private expression node into its exported [Expr]
// counterpart. A nil node (an absent optional sub-expression, such as a
// for-comprehension without an `if` clause) converts to a nil Expr.
func toExpr(e expr) Expr {
	switch n := e.(type) {
	case *litExpr:
		return LiteralExpr{Value: n.val}
	case *tmplExpr:
		return TemplateExpr{Raw: n.raw, Heredoc: n.heredoc}
	case *varExpr:
		return VarExpr{Name: n.name}
	case *callExpr:
		return CallExpr{Name: n.name, Args: toExprs(n.args), Expand: n.expand}
	case *attrExpr:
		return AttrExpr{Obj: toExpr(n.obj), Name: n.name}
	case *indexExpr:
		return IndexExpr{Coll: toExpr(n.coll), Idx: toExpr(n.idx)}
	case *unaryExpr:
		return UnaryExpr{Op: opText[n.op], Operand: toExpr(n.operand)}
	case *binaryExpr:
		return BinaryExpr{Op: opText[n.op], Left: toExpr(n.l), Right: toExpr(n.r)}
	case *condExpr:
		return CondExpr{Cond: toExpr(n.cond), Then: toExpr(n.then), Else: toExpr(n.els)}
	case *tupleExpr:
		return TupleExpr{Items: toExprs(n.items)}
	case *objectExpr:
		return ObjectExpr{Keys: toExprs(n.keys), Vals: toExprs(n.vals)}
	case *forTupleExpr:
		return ForTupleExpr{
			KeyVar: n.keyVar,
			ValVar: n.valVar,
			Coll:   toExpr(n.coll),
			Body:   toExpr(n.body),
			Cond:   toExpr(n.cond),
		}
	case *forObjectExpr:
		return ForObjectExpr{
			KeyVar:  n.keyVar,
			ValVar:  n.valVar,
			Coll:    toExpr(n.coll),
			KeyExpr: toExpr(n.keyE),
			ValExpr: toExpr(n.valE),
			Cond:    toExpr(n.cond),
			Group:   n.group,
		}
	}
	// A nil (absent optional) node, or — unreachable in practice — an unknown
	// node kind, maps to a nil Expr.
	return nil
}

// toExprs converts a slice of private nodes into exported [Expr] values,
// preserving order. A nil or empty input yields a nil slice.
func toExprs(es []expr) []Expr {
	if len(es) == 0 {
		return nil
	}
	out := make([]Expr, len(es))
	for i, e := range es {
		out[i] = toExpr(e)
	}
	return out
}
