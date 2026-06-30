// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import (
	"errors"
	"reflect"
	"testing"
)

func TestStdlibFunctions(t *testing.T) {
	cases := []struct {
		src  string
		want Value
	}{
		{`length("héllo")`, int64(5)},
		{`length([1,2,3])`, int64(3)},
		{`length({a=1,b=2})`, int64(2)},
		{`upper("abc")`, "ABC"},
		{`lower("ABC")`, "abc"},
		{`trimspace("  hi  ")`, "hi"},
		{`min(3, 1, 2)`, int64(1)},
		{`max(3, 1, 2)`, int64(3)},
		{`min(3.5, 1.0)`, float64(1)},
		{`max(1, 2.5)`, 2.5},
		{`abs(-5)`, int64(5)},
		{`abs(5)`, int64(5)},
		{`abs(-2.5)`, 2.5},
		{`floor(2.9)`, int64(2)},
		{`ceil(2.1)`, int64(3)},
		{`concat([1,2],[3])`, []Value{int64(1), int64(2), int64(3)}},
		{`join(",", ["a","b"])`, "a,b"},
		{`join("-", ["x"], ["y"])`, "x-y"},
		{`split(",", "a,b,c")`, []Value{"a", "b", "c"}},
		{`keys({b=2, a=1})`, []Value{"a", "b"}},
		{`values({b=2, a=1})`, []Value{int64(1), int64(2)}},
		{`lookup({a=1}, "a")`, int64(1)},
		{`lookup({a=1}, "z", 99)`, int64(99)},
		{`contains([1,2,3], 2)`, true},
		{`contains([1,2], 9)`, false},
		{`coalesce(null, null, 7)`, int64(7)},
		{`reverse([1,2,3])`, []Value{int64(3), int64(2), int64(1)}},
		{`tostring(42)`, "42"},
		{`tostring(true)`, "true"},
		{`tostring(1.5)`, "1.5"},
		{`tostring("s")`, "s"},
		{`tonumber("42")`, int64(42)},
		{`tonumber("1.5")`, 1.5},
		{`tonumber(3)`, int64(3)},
		{`tobool("true")`, true},
		{`tobool("false")`, false},
		{`tobool(true)`, true},
		{`format("%s-%d", "x", 5)`, "x-5"},
		{`format("%f", 1.5)`, "1.500000"},
		{`format("%g", 1.5)`, "1.5"},
		{`format("%t", true)`, "true"},
		{`format("%q", "hi")`, `"hi"`},
		{`format("%v", null)`, "null"},
		{`format("100%%")`, "100%"},
		{`format("%d", 3.0)`, "3"},
		{`jsonencode({a=1, b=[true, null, "s"]})`, `{"a":1,"b":[true,null,"s"]}`},
		{`jsonencode(1.5)`, "1.5"},
		{`jsonencode("a\nb")`, `"a\nb"`},
		{`jsondecode("{\"a\": [1, 2.5, true, null, \"s\"]}")`, func() Value {
			m := NewMap()
			m.Set("a", []Value{int64(1), 2.5, true, nil, "s"})
			return m
		}()},
		{`jsondecode("[]")`, []Value{}},
		{`jsondecode("{}")`, NewMap()},
	}
	for _, c := range cases {
		got := attr(t, c.src, nil)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s => %#v, want %#v", c.src, got, c.want)
		}
	}
}

func TestJSONStringEscapes(t *testing.T) {
	// control char and unicode escape round-trips.
	if got := attr(t, `jsonencode("\t")`, nil); got != `"\t"` {
		t.Errorf("tab encode %q", got)
	}
	if got := attr(t, "jsondecode(\"\\\"a\\\\u0041b\\\"\")", nil); got != "aAb" {
		t.Errorf("u-escape decode %q", got)
	}
	v := attr(t, "jsonencode(\"\\u0001\")", nil)
	if v != "\"\\u0001\"" {
		t.Errorf("ctrl encode %q", v)
	}
	// all simple json string escapes on decode
	if got := attr(t, "jsondecode(\"\\\"\\\\n\\\\t\\\\r\\\\b\\\\f\\\\/\\\\\\\\\\\"\")", nil); got != "\n\t\r\b\f/\\" {
		t.Errorf("escapes decode %q", got)
	}
}

func TestTryCan(t *testing.T) {
	ctx := NewContext()
	ctx.Variables["a"] = int64(1)
	if got := attr(t, `try(missing, a, 99)`, ctx); got != int64(1) {
		t.Errorf("try => %v", got)
	}
	if got := attr(t, `try(a)`, ctx); got != int64(1) {
		t.Errorf("try single => %v", got)
	}
	if got := attr(t, `can(a)`, ctx); got != true {
		t.Errorf("can true => %v", got)
	}
	if got := attr(t, `can(missing)`, ctx); got != false {
		t.Errorf("can false => %v", got)
	}
}

func TestCustomFunctions(t *testing.T) {
	ctx := NewContext()
	ctx.Functions["double"] = func(args []Value) (Value, error) {
		n := args[0].(int64)
		return n * 2, nil
	}
	ctx.Functions["boom"] = func(args []Value) (Value, error) {
		return nil, errors.New("kaboom")
	}
	if got := attr(t, `double(21)`, ctx); got != int64(42) {
		t.Errorf("custom fn => %v", got)
	}
	err := wantErr(t, "x = boom()\n", ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	// a custom function shadows a builtin.
	ctx.Functions["upper"] = func(args []Value) (Value, error) { return "SHADOW", nil }
	if got := attr(t, `upper("x")`, ctx); got != "SHADOW" {
		t.Errorf("shadow => %v", got)
	}
}

func TestVariadicSpread(t *testing.T) {
	if got := attr(t, `max(9, [1, 5]...)`, nil); got != int64(9) {
		t.Errorf("spread max => %v", got)
	}
	// spread a tuple of tuples into concat's variadic tuple arguments.
	if got := attr(t, `concat([1], [[2],[3]]...)`, nil); !reflect.DeepEqual(got, []Value{int64(1), int64(2), int64(3)}) {
		t.Errorf("spread concat => %#v", got)
	}
	// trailing comma after ...
	if got := attr(t, `max(0, [7]...,)`, nil); got != int64(7) {
		t.Errorf("spread trailing comma => %v", got)
	}
}
