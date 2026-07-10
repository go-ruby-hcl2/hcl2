// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import (
	"reflect"
	"testing"
)

// attrExprByName parses src and returns the exported expression of the named
// top-level attribute, failing the test if it is absent.
func attrExprByName(t *testing.T, src, name string) Expr {
	t.Helper()
	body, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	for _, a := range body.Attributes {
		if a.Name == name {
			return a.Expression()
		}
	}
	t.Fatalf("attribute %q not found", name)
	return nil
}

// TestExpressionAllKinds parses one document exercising every expression kind
// and asserts that Attribute.Expression yields the matching exported tree.
func TestExpressionAllKinds(t *testing.T) {
	const src = `
lit_int   = 42
lit_float = 3.5
lit_true  = true
lit_false = false
lit_null  = null
str       = "hello"
heredoc   = <<EOT
line
EOT
var_ref   = foo
call       = max(1, 2)
call_spread = max(args...)
attr  = foo.bar
index = foo[0]
neg   = -x
bnot  = !x
op_add = a + b
op_sub = a - b
op_mul = a * b
op_div = a / b
op_mod = a % b
op_eq  = a == b
op_ne  = a != b
op_lt  = a < b
op_le  = a <= b
op_gt  = a > b
op_ge  = a >= b
op_and = a && b
op_or  = a || b
cond  = c ? t : e
tuple = [1, 2, 3]
object = { a = 1, b = 2 }
for_tuple      = [for v in list : v]
for_tuple_cond = [for i, v in list : v if i]
for_object       = {for k, v in m : k => v}
for_object_group = {for v in m : v => v... if v}
`

	v := func(name string) VarExpr { return VarExpr{Name: name} }
	bin := func(op string) BinaryExpr { return BinaryExpr{Op: op, Left: v("a"), Right: v("b")} }

	tests := []struct {
		name string
		want Expr
	}{
		{"lit_int", LiteralExpr{Value: int64(42)}},
		{"lit_float", LiteralExpr{Value: float64(3.5)}},
		{"lit_true", LiteralExpr{Value: true}},
		{"lit_false", LiteralExpr{Value: false}},
		{"lit_null", LiteralExpr{Value: nil}},
		{"str", TemplateExpr{Raw: "hello", Heredoc: false}},
		{"heredoc", TemplateExpr{Raw: "line\n", Heredoc: true}},
		{"var_ref", v("foo")},
		{"call", CallExpr{Name: "max", Args: []Expr{LiteralExpr{Value: int64(1)}, LiteralExpr{Value: int64(2)}}, Expand: false}},
		{"call_spread", CallExpr{Name: "max", Args: []Expr{v("args")}, Expand: true}},
		{"attr", AttrExpr{Obj: v("foo"), Name: "bar"}},
		{"index", IndexExpr{Coll: v("foo"), Idx: LiteralExpr{Value: int64(0)}}},
		{"neg", UnaryExpr{Op: "-", Operand: v("x")}},
		{"bnot", UnaryExpr{Op: "!", Operand: v("x")}},
		{"op_add", bin("+")},
		{"op_sub", bin("-")},
		{"op_mul", bin("*")},
		{"op_div", bin("/")},
		{"op_mod", bin("%")},
		{"op_eq", bin("==")},
		{"op_ne", bin("!=")},
		{"op_lt", bin("<")},
		{"op_le", bin("<=")},
		{"op_gt", bin(">")},
		{"op_ge", bin(">=")},
		{"op_and", bin("&&")},
		{"op_or", bin("||")},
		{"cond", CondExpr{Cond: v("c"), Then: v("t"), Else: v("e")}},
		{"tuple", TupleExpr{Items: []Expr{
			LiteralExpr{Value: int64(1)}, LiteralExpr{Value: int64(2)}, LiteralExpr{Value: int64(3)},
		}}},
		{"object", ObjectExpr{
			Keys: []Expr{LiteralExpr{Value: "a"}, LiteralExpr{Value: "b"}},
			Vals: []Expr{LiteralExpr{Value: int64(1)}, LiteralExpr{Value: int64(2)}},
		}},
		{"for_tuple", ForTupleExpr{
			KeyVar: "", ValVar: "v", Coll: v("list"), Body: v("v"), Cond: nil,
		}},
		{"for_tuple_cond", ForTupleExpr{
			KeyVar: "i", ValVar: "v", Coll: v("list"), Body: v("v"), Cond: v("i"),
		}},
		{"for_object", ForObjectExpr{
			KeyVar: "k", ValVar: "v", Coll: v("m"), KeyExpr: v("k"), ValExpr: v("v"), Cond: nil, Group: false,
		}},
		{"for_object_group", ForObjectExpr{
			KeyVar: "", ValVar: "v", Coll: v("m"), KeyExpr: v("v"), ValExpr: v("v"), Cond: v("v"), Group: true,
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := attrExprByName(t, src, tc.name)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Expression() for %s =\n  %#v\nwant\n  %#v", tc.name, got, tc.want)
			}
		})
	}
}

// TestOpTextCoverage asserts every operator token maps to its canonical text,
// covering the whole opText table directly.
func TestOpTextCoverage(t *testing.T) {
	want := map[tokKind]string{
		tPlus: "+", tMinus: "-", tStar: "*", tSlash: "/", tPercent: "%",
		tEq: "==", tNe: "!=", tLt: "<", tLe: "<=", tGt: ">", tGe: ">=",
		tAnd: "&&", tOr: "||", tBang: "!",
	}
	if len(opText) != len(want) {
		t.Fatalf("opText has %d entries, want %d", len(opText), len(want))
	}
	for k, w := range want {
		if got := opText[k]; got != w {
			t.Errorf("opText[%d] = %q, want %q", k, got, w)
		}
	}
}

// TestExprNodeSealed exercises the sealed marker method on every exported node
// type, confirming each concrete type satisfies Expr.
func TestExprNodeSealed(t *testing.T) {
	nodes := []Expr{
		LiteralExpr{}, TemplateExpr{}, VarExpr{}, CallExpr{}, AttrExpr{},
		IndexExpr{}, UnaryExpr{}, BinaryExpr{}, CondExpr{}, TupleExpr{},
		ObjectExpr{}, ForTupleExpr{}, ForObjectExpr{},
	}
	for _, n := range nodes {
		n.exprNode() // must be callable; presence is the assertion
	}
	if len(nodes) != 13 {
		t.Fatalf("expected 13 exported node kinds, got %d", len(nodes))
	}
}

// TestToExprNil asserts a nil private node converts to a nil Expr.
func TestToExprNil(t *testing.T) {
	if got := toExpr(nil); got != nil {
		t.Fatalf("toExpr(nil) = %#v, want nil", got)
	}
	if got := toExprs(nil); got != nil {
		t.Fatalf("toExprs(nil) = %#v, want nil", got)
	}
}
