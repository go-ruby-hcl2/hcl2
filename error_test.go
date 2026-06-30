// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import (
	"strings"
	"testing"
)

// parseErr expects Parse to fail and returns the message.
func parseErr(t *testing.T, src string) string {
	t.Helper()
	_, err := Parse(src)
	if err == nil {
		t.Fatalf("Parse(%q) expected error", src)
	}
	return err.Error()
}

func TestLexErrors(t *testing.T) {
	cases := []struct{ src, want string }{
		{`a = "unterminated`, "unterminated string"},
		{"a = /* open", "unterminated block comment"},
		{"a = 1 & 2", "did you mean '&&'"},
		{"a = 1 | 2", "did you mean '||'"},
		{"a = @", "invalid character"},
		{"a = <<-", "heredoc delimiter must be an identifier"},
		{"a = <<EOT extra\n", "unexpected characters after heredoc introducer"},
		{"a = <<EOT\nbody no end\n", "unterminated heredoc"},
		{"a = <<EOT", "unterminated heredoc"},
	}
	for _, c := range cases {
		if got := parseErr(t, c.src); !strings.Contains(got, c.want) {
			t.Errorf("%q => %q, want contains %q", c.src, got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct{ src, want string }{
		{`= 1`, "expected attribute or block name"},
		{`a`, "expected"},
		{`123 = 1`, "expected attribute or block name"},
		{"a = 1 b = 2", "expected newline after body item"},
		{"a {\n", "expected '}'"},
		{`a = (1`, "expected ')'"},
		{`a = [1 2]`, "expected ',' or ']'"},
		{`a = {x 1}`, "expected '=' or ':'"},
		{`a = {x = 1 ! 2}`, "expected ',' or newline"},
		{`a = 1 ? 2`, "expected ':'"},
		{`a = b.[0]`, "expected attribute name after '.'"},
		{`a = f(1 2)`, "expected ',' or ')'"},
		{`a = b[*][*]`, "chained splats are not supported"},
		{`a = b.*.*`, "chained splats are not supported"},
		{`a = [for = in xs : x]`, "expected loop variable"},
		{`a = [for x, = in xs : x]`, "expected second loop variable"},
		{`a = [for x xs : x]`, "expected 'in'"},
		{`a = [for x in xs x]`, "expected ':'"},
		{`a = {for x in xs : x}`, "expected '=>'"},
		{`a = )`, "expected expression"},
		{`a = b[*` + "\n", "expected ']' after splat"},
		{`a = b[1`, "expected ']' after index"},
		{`a = b.*[`, "expected expression"},
		{`a = b.*.`, "expected attribute name after '.'"},
		{"a = b\nc \"x\nbad", "unterminated string"},
	}
	for _, c := range cases {
		if got := parseErr(t, c.src); !strings.Contains(got, c.want) {
			t.Errorf("%q => %q, want contains %q", c.src, got, c.want)
		}
	}
}

func TestEvalErrors(t *testing.T) {
	ctx := NewContext()
	ctx.Variables["s"] = "str"
	ctx.Variables["n"] = int64(3)
	ctx.Variables["tup"] = []Value{int64(1)}
	ctx.Variables["obj"] = func() *Map { m := NewMap(); m.Set("k", "v"); return m }()

	cases := []struct{ src, want string }{
		{`x = undefined_var` + "\n", "unknown variable"},
		{`x = s.field` + "\n", "non-object"},
		{`x = obj.missing` + "\n", "no attribute"},
		{`x = tup[5]` + "\n", "out of range"},
		{`x = tup["k"]` + "\n", "whole number"},
		{`x = tup[1.5]` + "\n", "whole number"},
		{`x = obj[true]` + "\n", "object key must be a string"},
		{`x = obj["nope"]` + "\n", "no element"},
		{`x = n[0]` + "\n", "cannot index"},
		{`x = -s` + "\n", "unary '-' requires a number"},
		{`x = !n` + "\n", "unary '!' requires a bool"},
		{`x = n ? 1 : 2` + "\n", "predicate must be a bool"},
		{`x = s + n` + "\n", "arithmetic requires numbers"},
		{`x = 1 / 0` + "\n", "division by zero"},
		{`x = 1 % 0` + "\n", "modulo by zero"},
		{`x = 1.0 / 0` + "\n", "division by zero"},
		{`x = s < n` + "\n", "comparison requires numbers"},
		{`x = s && true` + "\n", "logical operator requires bool"},
		{`x = true && n` + "\n", "logical operator requires bool"},
		{`x = s || true` + "\n", "logical operator requires bool"},
		{`x = nope_func()` + "\n", "unknown function"},
		{`x = [for v in n : v]` + "\n", "requires a tuple or object"},
		{`x = {for v in n : v => v}` + "\n", "requires a tuple or object"},
		{`x = [for v in tup : v if v]` + "\n", "must be a bool"},
		{`x = {for k, v in {a=1,b=1} : "same" => v}` + "\n", "duplicate object key"},
		{`x = {(tup) = 1}` + "\n", "object key must be a string"},
		{`x = "${nope}"` + "\n", "unknown variable"},
		{`x = "${null}"` + "\n", "cannot interpolate null"},
		{`x = "${tup}"` + "\n", "cannot interpolate a collection"},
		{`x = "${1 +}"` + "\n", "expected expression"},
		{`x = "${1 1}"` + "\n", "unexpected trailing tokens"},
		{`x = "${"` + "\n", "unterminated"},
		{`x = "bad\q"` + "\n", "invalid escape sequence"},
		{`x = "trunc\u12"` + "\n", "truncated unicode escape"},
		{`x = "bad\uZZZZ"` + "\n", "invalid unicode escape"},
		{`x = "sur\uD800"` + "\n", "invalid unicode code point"},
	}
	for _, c := range cases {
		err := wantErr(t, c.src, ctx)
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q => %q, want contains %q", c.src, err.Error(), c.want)
		}
	}
}

func TestDirectiveErrors(t *testing.T) {
	ctx := NewContext()
	ctx.Variables["n"] = int64(1)
	ctx.Variables["t"] = []Value{int64(1)}
	cases := []struct{ src, want string }{
		{`x = "%{ if n }a%{ endif }"` + "\n", "must be a bool"},
		{`x = "%{ for n }a%{ endfor }"` + "\n", "requires 'in'"},
		{`x = "%{ for , in t }a%{ endfor }"` + "\n", "requires a loop variable"},
		{`x = "%{ nope }"` + "\n", "unknown template directive"},
		{`x = "%{ if true }a"` + "\n", "unterminated"},
		{`x = "%{ for x in t }a"` + "\n", "unterminated"},
	}
	for _, c := range cases {
		err := wantErr(t, c.src, ctx)
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q => %q, want contains %q", c.src, err.Error(), c.want)
		}
	}
}

func TestFunctionErrors(t *testing.T) {
	cases := []struct{ src, want string }{
		{`length(1)`, "string, tuple, or object"},
		{`length()`, "wrong number"},
		{`upper(1)`, "expected a string"},
		{`min()`, "at least one"},
		{`min("a")`, "requires numbers"},
		{`min(1, "a")`, "requires numbers"},
		{`max("a")`, "requires numbers"},
		{`abs("a")`, "requires a number"},
		{`floor("a")`, "requires a number"},
		{`concat(1)`, "tuple arguments"},
		{`join()`, "requires a separator"},
		{`join(1, [])`, "separator must be a string"},
		{`join(",", 1)`, "tuple arguments"},
		{`join(",", [1])`, "elements must be strings"},
		{`split(1, "x")`, "two strings"},
		{`keys(1)`, "requires an object"},
		{`values(1)`, "requires an object"},
		{`lookup(1)`, "2 or 3"},
		{`lookup(1, "k")`, "requires an object"},
		{`lookup({}, 1)`, "key must be a string"},
		{`lookup({}, "k")`, "not found"},
		{`contains(1, 2)`, "requires a tuple"},
		{`coalesce(null)`, "no non-null"},
		{`reverse(1)`, "requires a tuple"},
		{`tostring([])`, "primitive value"},
		{`tonumber("zz")`, "could not parse"},
		{`tonumber([])`, "number or numeric string"},
		{`tobool("zz")`, "bool or"},
		{`tobool([])`, "bool or"},
		{`format()`, "requires a format string"},
		{`format(1)`, "spec must be a string"},
		{`format("%")`, "dangling"},
		{`format("%d")`, "not enough arguments"},
		{`format("%d", "x")`, "requires an integer"},
		{`format("%d", 1.5)`, "requires an integer"},
		{`format("%f", "x")`, "requires a number"},
		{`format("%g", "x")`, "requires a number"},
		{`format("%t", 1)`, "requires a bool"},
		{`format("%z", 1)`, "unknown verb"},
		{`format("%s", [])`, "cannot render a collection"},
		{`jsonencode([1, {a=null}])`, ""}, // success path, no error
		{`try()`, "at least one argument"},
		{`can(1, 2)`, "exactly one"},
		{`max(1, "x"...)`, "must be a tuple"},
	}
	for _, c := range cases {
		_, err := Eval("x = "+c.src+"\n", nil)
		if c.want == "" {
			if err != nil {
				t.Errorf("%q unexpected error %v", c.src, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q => %v, want contains %q", c.src, err, c.want)
		}
	}
}

func TestJSONDecodeErrors(t *testing.T) {
	cases := []struct{ src, want string }{
		{`""`, "unexpected end"},
		{`"  "`, "unexpected end"},
		{`"@"`, "unexpected character"},
		{`"{"`, "expected object key"},
		{`"{\"a\"}"`, "expected ':'"},
		{`"{\"a\":1"`, "unterminated object"},
		{`"{\"a\":1 2}"`, "expected ',' or '}'"},
		{`"[1"`, "unterminated array"},
		{`"[1 2]"`, "expected ',' or ']'"},
		{`"tru"`, "invalid literal"},
		{`"nul"`, "invalid literal"},
		{`"\"abc"`, "unterminated string"},
		{`"\"\\x\""`, "invalid escape"},
		{`"\"\\u12\""`, "truncated"},
		{`"\"\\uZZZZ\""`, "invalid \\u"},
		{`"1 2"`, "trailing data"},
		{`"[1,]"`, "unexpected character"},
	}
	for _, c := range cases {
		_, err := Eval("x = jsondecode("+c.src+")\n", nil)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("jsondecode(%s) => %v, want contains %q", c.src, err, c.want)
		}
	}
}
