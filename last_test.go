// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import "testing"

// TestBlockBodyEvalError covers docToMap's nested-block error branch.
func TestBlockBodyEvalError(t *testing.T) {
	if _, err := Eval("b {\n  x = undef\n}\n", nil); err == nil {
		t.Error("expected nested block eval error")
	}
}

// TestFormatQError covers the %q verb's error path on a non-renderable value.
func TestFormatQError(t *testing.T) {
	if _, err := Eval(`x = format("%q", [1])`+"\n", nil); err == nil {
		t.Error("expected percent-q collection error")
	}
}

// TestIdentBlockLabel covers the bare-identifier label branch.
func TestIdentBlockLabel(t *testing.T) {
	m := evalOK(t, "svc foo {\n  on = true\n}\n", nil)
	svc, _ := m.Get("svc")
	foo, _ := svc.(*Map).Get("foo")
	if v, _ := foo.(*Map).Get("on"); v != true {
		t.Errorf("ident label => %v", v)
	}
}

// TestJSONKeyEscapeError covers the object-key str() error branch.
func TestJSONKeyEscapeError(t *testing.T) {
	// an invalid escape inside the object key string.
	if _, err := Eval(`x = jsondecode("{\"a\\x\": 1}")`+"\n", nil); err == nil {
		t.Error("expected object-key escape error")
	}
}

// TestJSONStringEscapeInValue covers the str() escaped-quote case.
func TestJSONStringEscapeInValue(t *testing.T) {
	if got := attr(t, `jsondecode("\"a\\\"b\"")`, nil); got != `a"b` {
		t.Errorf("escaped quote in value => %q", got)
	}
}

// TestLexPeek2AtEOF drives peek2 past end of input via a trailing one-byte token
// candidate (`<`) at the very end.
func TestLexPeek2AtEOF(t *testing.T) {
	// a lone '<' at EOF: peek2 reads past the buffer (returns 0), then '<' is
	// lexed as the less-than operator.
	if _, err := Parse("x = 1 <"); err == nil {
		// "1 <" is an incomplete comparison -> parse error is expected, but the
		// lexer must not panic reaching peek2 at EOF.
		t.Error("expected incomplete-comparison parse error")
	}
	// a trailing '=' at EOF likewise calls peek2 at the boundary.
	if _, err := Parse("x ="); err == nil {
		t.Error("expected trailing '=' error")
	}
}

// TestUnterminatedInterpInTokenize covers scanBraced's error path. The quoted-
// string lexer balances braces itself, so an unbalanced `${` is surfaced through
// a heredoc body (which the lexer scans line-by-line without brace tracking).
func TestUnterminatedInterpTokenize(t *testing.T) {
	if _, err := Eval("x = <<EOT\n${ unclosed\nEOT\n", nil); err == nil {
		t.Error("expected unterminated interpolation in heredoc")
	}
}

// TestDirectiveNestedDifferentKind covers containsStr's false return: an `if`
// whose then-branch contains a `for` directive (a directive keyword that is not
// the if's stop word, so the stop check returns false and the for is rendered).
func TestDirectiveNestedDifferentKind(t *testing.T) {
	ctx := NewContext()
	ctx.Variables["flag"] = true
	ctx.Variables["items"] = []Value{"a", "b"}
	if got := attr(t, `"%{ if flag }<%{ for i in items }${i}%{ endfor }>%{ endif }"`, ctx); got != "<ab>" {
		t.Errorf("nested-different-kind directive => %q", got)
	}
}

// TestDanglingBackslashDirect drives decodeEscape's len<2 guard: a string whose
// only content is a backslash that the lexer keeps (it consumes the following
// closing quote as the escaped char, leaving the string unterminated — so we use
// a value where the backslash is the final raw byte).
func TestDanglingBackslashDirect(t *testing.T) {
	// `"\` then newline: the lexer treats \" as an escaped quote, so the string is
	// unterminated -> a lexical error (covers the lexer escape handling). To hit
	// decodeEscape's guard we feed a raw template via a heredoc-free path with a
	// trailing backslash before EOS is impossible through normal lexing; instead
	// rely on a string with a backslash followed by end handled by tokenizer.
	s, n, d := decodeEscape("\\", Pos{})
	if d == nil || s != "" || n != 0 {
		t.Errorf("dangling backslash guard: %q %d %v", s, n, d)
	}
}

// TestDirectiveErrorBranches covers the if-condition error, else-branch render
// error, and for-body render error inside directives.
func TestDirectiveErrorBranches(t *testing.T) {
	ctx := NewContext()
	ctx.Variables["t"] = []Value{int64(1)}
	bad := []string{
		`x = "%{ if undef }a%{ endif }"`,                  // if condition eval error
		`x = "%{ if true }${undef}%{ endif }"`,            // then-branch render error
		`x = "%{ if false }a%{ else }${undef}%{ endif }"`, // else-branch render error
		`x = "%{ for v in t }${undef}%{ endfor }"`,        // for-body render error
		`x = "%{ for v in undef }a%{ endfor }"`,           // for collection error
	}
	for _, src := range bad {
		if _, err := Eval(src+"\n", ctx); err == nil {
			t.Errorf("%q expected directive error", src)
		}
	}
}
