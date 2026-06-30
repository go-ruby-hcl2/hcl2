// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import (
	"reflect"
	"testing"
)

// evalOK evaluates src against ctx, failing the test on any error.
func evalOK(t *testing.T, src string, ctx *Context) *Map {
	t.Helper()
	m, err := Eval(src, ctx)
	if err != nil {
		t.Fatalf("Eval(%q) error: %v", src, err)
	}
	return m
}

// attr evaluates `x = <expr>` and returns x's value.
func attr(t *testing.T, exprSrc string, ctx *Context) Value {
	t.Helper()
	m := evalOK(t, "x = "+exprSrc+"\n", ctx)
	v, ok := m.Get("x")
	if !ok {
		t.Fatalf("attr x missing for %q", exprSrc)
	}
	return v
}

func wantErr(t *testing.T, src string, ctx *Context) error {
	t.Helper()
	_, err := Eval(src, ctx)
	if err == nil {
		t.Fatalf("Eval(%q) expected error, got nil", src)
	}
	return err
}

func TestLiterals(t *testing.T) {
	cases := []struct {
		src  string
		want Value
	}{
		{`42`, int64(42)},
		{`-7`, int64(-7)},
		{`3.5`, 3.5},
		{`1e3`, float64(1000)},
		{`2.5E-1`, 0.25},
		{`true`, true},
		{`false`, false},
		{`null`, nil},
		{`"plain"`, "plain"},
		{`"a\tb\nc"`, "a\tb\nc"},
		{`"q\"q"`, "q\"q"},
		{`"back\\slash"`, "back\\slash"},
		{`"uA"`, "uA"},
		{`"U\U0001F600"`, "U\U0001F600"},
	}
	for _, c := range cases {
		if got := attr(t, c.src, nil); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s => %#v, want %#v", c.src, got, c.want)
		}
	}
}

func TestArithmetic(t *testing.T) {
	cases := []struct {
		src  string
		want Value
	}{
		{`1 + 2`, int64(3)},
		{`10 - 4`, int64(6)},
		{`3 * 4`, int64(12)},
		{`9 / 3`, int64(3)},
		{`7 / 2`, 3.5},
		{`7 % 3`, int64(1)},
		{`1.5 + 2.5`, float64(4)},
		{`2 * 1.5`, float64(3)},
		{`5.5 % 2`, 1.5},
		{`"a" + "b"`, "ab"},
		{`-(3)`, int64(-3)},
		{`-3.5`, -3.5},
		{`!true`, false},
		{`!!false`, false},
		{`2 + 3 * 4`, int64(14)},
		{`(2 + 3) * 4`, int64(20)},
	}
	for _, c := range cases {
		if got := attr(t, c.src, nil); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s => %#v, want %#v", c.src, got, c.want)
		}
	}
}

func TestComparisonLogical(t *testing.T) {
	cases := []struct {
		src  string
		want Value
	}{
		{`1 < 2`, true},
		{`2 <= 2`, true},
		{`3 > 4`, false},
		{`4 >= 4`, true},
		{`1 == 1`, true},
		{`1 == 1.0`, true},
		{`1 != 2`, true},
		{`"a" == "a"`, true},
		{`"a" == "b"`, false},
		{`true == true`, true},
		{`null == null`, true},
		{`null == 1`, false},
		{`[1,2] == [1,2]`, true},
		{`[1,2] == [1,3]`, false},
		{`[1] == [1,2]`, false},
		{`{a=1} == {a=1}`, true},
		{`{a=1} == {a=2}`, false},
		{`{a=1} == {a=1,b=2}`, false},
		{`{a=1} == {b=1}`, false},
		{`true && false`, false},
		{`true && true`, true},
		{`false || true`, true},
		{`false || false`, false},
		{`true || error_undefined_shortcircuit`, true},
		{`false && error_undefined_shortcircuit`, false},
		{`1 == "a"`, false},
		{`"a" == 1`, false},
		{`true == 1`, false},
	}
	for _, c := range cases {
		if got := attr(t, c.src, nil); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s => %#v, want %#v", c.src, got, c.want)
		}
	}
}

func TestConditional(t *testing.T) {
	if got := attr(t, `true ? "y" : "n"`, nil); got != "y" {
		t.Errorf("got %v", got)
	}
	if got := attr(t, `false ? "y" : "n"`, nil); got != "n" {
		t.Errorf("got %v", got)
	}
	if got := attr(t, `true ? 1 : 2 > 3 ? 4 : 5`, nil); got != int64(1) {
		t.Errorf("got %v", got)
	}
}

func TestCollections(t *testing.T) {
	tup := attr(t, `[1, 2, 3]`, nil).([]Value)
	if !reflect.DeepEqual(tup, []Value{int64(1), int64(2), int64(3)}) {
		t.Errorf("tuple %#v", tup)
	}
	if got := attr(t, `[]`, nil); len(got.([]Value)) != 0 {
		t.Errorf("empty tuple %#v", got)
	}
	if got := attr(t, `[1, 2,]`, nil); len(got.([]Value)) != 2 {
		t.Errorf("trailing comma %#v", got)
	}
	obj := attr(t, `{ a = 1, "b" = 2 }`, nil).(*Map)
	if v, _ := obj.Get("a"); v != int64(1) {
		t.Errorf("obj.a %v", v)
	}
	if v, _ := obj.Get("b"); v != int64(2) {
		t.Errorf("obj.b %v", v)
	}
	if got := attr(t, `{}`, nil).(*Map).Len(); got != 0 {
		t.Errorf("empty object %d", got)
	}
	// newline-separated object items and ':' separator.
	multi := evalOK(t, "x = {\n  a = 1\n  b : 2\n}\n", nil)
	mo, _ := multi.Get("x")
	if mo.(*Map).Len() != 2 {
		t.Errorf("multiline object %#v", mo)
	}
}

func TestTraversal(t *testing.T) {
	ctx := NewContext()
	inner := NewMap()
	inner.Set("c", int64(9))
	mid := NewMap()
	mid.Set("b", inner)
	ctx.Variables["a"] = mid
	ctx.Variables["list"] = []Value{int64(10), int64(20)}
	ctx.Variables["m"] = func() *Map { mm := NewMap(); mm.Set("k", "v"); return mm }()

	if got := attr(t, `a.b.c`, ctx); got != int64(9) {
		t.Errorf("a.b.c %v", got)
	}
	if got := attr(t, `list[0]`, ctx); got != int64(10) {
		t.Errorf("list[0] %v", got)
	}
	if got := attr(t, `list[1]`, ctx); got != int64(20) {
		t.Errorf("list[1] %v", got)
	}
	if got := attr(t, `m["k"]`, ctx); got != "v" {
		t.Errorf(`m["k"] %v`, got)
	}
	if got := attr(t, `a.b["c"]`, ctx); got != int64(9) {
		t.Errorf("mixed %v", got)
	}
	// `a.0` numeric-attr sugar for indexing.
	if got := attr(t, `list.0`, ctx); got != int64(10) {
		t.Errorf("list.0 %v", got)
	}
	// integer object key coercion.
	im := NewMap()
	im.Set("7", "seven")
	ctx.Variables["im"] = im
	if got := attr(t, `im[7]`, ctx); got != "seven" {
		t.Errorf("im[7] %v", got)
	}
}
