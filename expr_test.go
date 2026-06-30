// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import (
	"reflect"
	"testing"
)

func TestForTuple(t *testing.T) {
	if got := attr(t, `[for x in [1,2,3] : x * 10]`, nil); !reflect.DeepEqual(got, []Value{int64(10), int64(20), int64(30)}) {
		t.Errorf("for-tuple %#v", got)
	}
	if got := attr(t, `[for i, x in [5,6] : i]`, nil); !reflect.DeepEqual(got, []Value{int64(0), int64(1)}) {
		t.Errorf("for-tuple index %#v", got)
	}
	if got := attr(t, `[for x in [1,2,3,4] : x if x % 2 == 0]`, nil); !reflect.DeepEqual(got, []Value{int64(2), int64(4)}) {
		t.Errorf("for-tuple if %#v", got)
	}
	// over an object: value var
	if got := attr(t, `[for v in {a=1, b=2} : v]`, nil); !reflect.DeepEqual(got, []Value{int64(1), int64(2)}) {
		t.Errorf("for-tuple over object %#v", got)
	}
	// over an object: key, value vars
	if got := attr(t, `[for k, v in {a=1} : k]`, nil); !reflect.DeepEqual(got, []Value{"a"}) {
		t.Errorf("for-tuple object key %#v", got)
	}
}

func TestForObject(t *testing.T) {
	got := attr(t, `{for k, v in {a=1, b=2} : k => v + 10}`, nil).(*Map)
	if v, _ := got.Get("a"); v != int64(11) {
		t.Errorf("for-object a %v", v)
	}
	if v, _ := got.Get("b"); v != int64(12) {
		t.Errorf("for-object b %v", v)
	}
	// single var over a tuple
	g2 := attr(t, `{for v in ["x","y"] : v => upper(v)}`, nil).(*Map)
	if v, _ := g2.Get("x"); v != "X" {
		t.Errorf("for-object single %v", v)
	}
	// if filter
	g3 := attr(t, `{for k, v in {a=1, b=2, c=3} : k => v if v > 1}`, nil).(*Map)
	if g3.Len() != 2 {
		t.Errorf("for-object if len %d", g3.Len())
	}
	// grouping mode
	g4 := attr(t, `{for v in [1,2,3,4] : (v % 2 == 0 ? "even" : "odd") => v...}`, nil).(*Map)
	odd, _ := g4.Get("odd")
	if !reflect.DeepEqual(odd, []Value{int64(1), int64(3)}) {
		t.Errorf("grouping odd %#v", odd)
	}
	even, _ := g4.Get("even")
	if !reflect.DeepEqual(even, []Value{int64(2), int64(4)}) {
		t.Errorf("grouping even %#v", even)
	}
}

func TestSplat(t *testing.T) {
	ctx := NewContext()
	u1 := NewMap()
	u1.Set("id", int64(1))
	u1.Set("name", "a")
	u2 := NewMap()
	u2.Set("id", int64(2))
	u2.Set("name", "b")
	ctx.Variables["users"] = []Value{u1, u2}

	if got := attr(t, `users.*.id`, ctx); !reflect.DeepEqual(got, []Value{int64(1), int64(2)}) {
		t.Errorf("attr splat %#v", got)
	}
	if got := attr(t, `users[*].name`, ctx); !reflect.DeepEqual(got, []Value{"a", "b"}) {
		t.Errorf("full splat %#v", got)
	}
	// splat with trailing index
	nested := NewMap()
	nested.Set("tags", []Value{"t0", "t1"})
	ctx.Variables["rows"] = []Value{nested}
	if got := attr(t, `rows[*].tags[0]`, ctx); !reflect.DeepEqual(got, []Value{"t0"}) {
		t.Errorf("splat index %#v", got)
	}
}

func TestTemplates(t *testing.T) {
	ctx := NewContext()
	ctx.Variables["name"] = "world"
	ctx.Variables["n"] = int64(3)
	ctx.Variables["pi"] = 3.5
	ctx.Variables["ok"] = true
	cases := []struct{ src, want string }{
		{`"hi ${name}"`, "hi world"},
		{`"n=${n}"`, "n=3"},
		{`"pi=${pi}"`, "pi=3.5"},
		{`"ok=${ok}"`, "ok=true"},
		{`"${ {a=1}.a }"`, "1"},
		{`"$${literal}"`, "${literal}"},
		{`"100%%{not}"`, "100%{not}"},
		{`"a${name}b${name}c"`, "aworldbworldc"},
	}
	for _, c := range cases {
		if got := attr(t, c.src, ctx); got != c.want {
			t.Errorf("%s => %q, want %q", c.src, got, c.want)
		}
	}
}

func TestTemplateDirectives(t *testing.T) {
	ctx := NewContext()
	ctx.Variables["flag"] = true
	ctx.Variables["items"] = []Value{"a", "b", "c"}
	ctx.Variables["m"] = func() *Map { mm := NewMap(); mm.Set("k", "v"); return mm }()

	if got := attr(t, `"%{ if flag }on%{ else }off%{ endif }"`, ctx); got != "on" {
		t.Errorf("if-true %q", got)
	}
	ctx.Variables["flag"] = false
	if got := attr(t, `"%{ if flag }on%{ else }off%{ endif }"`, ctx); got != "off" {
		t.Errorf("if-false %q", got)
	}
	// if without else, false => empty
	if got := attr(t, `"x%{ if flag }Y%{ endif }z"`, ctx); got != "xz" {
		t.Errorf("if-no-else %q", got)
	}
	// for over tuple
	if got := attr(t, `"%{ for i in items }${i}-%{ endfor }"`, ctx); got != "a-b-c-" {
		t.Errorf("for-tuple-dir %q", got)
	}
	// for over object with k,v
	if got := attr(t, `"%{ for k, v in m }${k}=${v}%{ endfor }"`, ctx); got != "k=v" {
		t.Errorf("for-object-dir %q", got)
	}
	// nested for
	if got := attr(t, `"%{ for x in items }[%{ for y in items }${y}%{ endfor }]%{ endfor }"`, ctx); got != "[abc][abc][abc]" {
		t.Errorf("nested-for %q", got)
	}
}

func TestHeredoc(t *testing.T) {
	// plain heredoc keeps indentation and backslashes literal.
	m := evalOK(t, "x = <<EOT\nline1\nline2\nEOT\n", nil)
	v, _ := m.Get("x")
	if v != "line1\nline2\n" {
		t.Errorf("plain heredoc %q", v)
	}
	// indented heredoc strips common leading whitespace.
	m = evalOK(t, "x = <<-EOT\n    a\n      b\n    EOT\n", nil)
	v, _ = m.Get("x")
	if v != "a\n  b\n" {
		t.Errorf("indent heredoc %q", v)
	}
	// interpolation works inside heredocs.
	ctx := NewContext()
	ctx.Variables["w"] = "W"
	m = evalOK(t, "x = <<EOT\nhi ${w}\nEOT\n", ctx)
	v, _ = m.Get("x")
	if v != "hi W\n" {
		t.Errorf("heredoc interp %q", v)
	}
	// backslash stays literal in heredoc.
	m = evalOK(t, "x = <<EOT\na\\nb\nEOT\n", nil)
	v, _ = m.Get("x")
	if v != "a\\nb\n" {
		t.Errorf("heredoc backslash %q", v)
	}
}

func TestBlocks(t *testing.T) {
	src := `
service "web" {
  port = 8080
}
service "api" {
  port = 9090
}
settings {
  debug = true
}
`
	m := evalOK(t, src, nil)
	svc, _ := m.Get("service")
	sm := svc.(*Map)
	web, _ := sm.Get("web")
	if p, _ := web.(*Map).Get("port"); p != int64(8080) {
		t.Errorf("web port %v", p)
	}
	api, _ := sm.Get("api")
	if p, _ := api.(*Map).Get("port"); p != int64(9090) {
		t.Errorf("api port %v", p)
	}
	// unlabelled block nests directly.
	st, _ := m.Get("settings")
	if d, _ := st.(*Map).Get("debug"); d != true {
		t.Errorf("settings debug %v", d)
	}
}

func TestBlocksCollect(t *testing.T) {
	// repeated unlabelled blocks of the same type collect into a tuple.
	src := `
rule {
  n = 1
}
rule {
  n = 2
}
`
	m := evalOK(t, src, nil)
	rules, _ := m.Get("rule")
	tup, ok := rules.([]Value)
	if !ok || len(tup) != 2 {
		t.Fatalf("rules not collected: %#v", rules)
	}
	if n, _ := tup[0].(*Map).Get("n"); n != int64(1) {
		t.Errorf("rule[0].n %v", n)
	}
	// multi-label block.
	src2 := `
route "a" "b" {
  to = "x"
}
`
	m2 := evalOK(t, src2, nil)
	route, _ := m2.Get("route")
	a, _ := route.(*Map).Get("a")
	b, _ := a.(*Map).Get("b")
	if to, _ := b.(*Map).Get("to"); to != "x" {
		t.Errorf("multi-label %v", to)
	}
}

func TestParseOnly(t *testing.T) {
	body, err := Parse("a = 1\nb \"l\" {\n c = 2\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(body.Attributes) != 1 || body.Attributes[0].Name != "a" {
		t.Errorf("attributes %#v", body.Attributes)
	}
	if len(body.Blocks) != 1 || body.Blocks[0].Type != "b" || body.Blocks[0].Labels[0] != "l" {
		t.Errorf("blocks %#v", body.Blocks)
	}
	// lazy per-attribute decode.
	v, ok, err := body.Attr("a", nil)
	if err != nil || !ok || v != int64(1) {
		t.Errorf("Attr a => %v %v %v", v, ok, err)
	}
	if _, ok, _ := body.Attr("missing", nil); ok {
		t.Errorf("missing attr reported present")
	}
}

func TestEvalExpr(t *testing.T) {
	ctx := NewContext()
	ctx.Variables["k"] = int64(5)
	v, err := EvalExpr("k + 1", ctx)
	if err != nil || v != int64(6) {
		t.Errorf("EvalExpr => %v %v", v, err)
	}
}
