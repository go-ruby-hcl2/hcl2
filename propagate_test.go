// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import "testing"

// TestErrorPropagation drives a parse or lexical error deep inside every nesting
// construct so each error-propagation branch is exercised. A bare `&` is a
// guaranteed lexical error; an unterminated string serves the same purpose where
// a `&` would be ambiguous.
func TestErrorPropagation(t *testing.T) {
	bad := []string{
		// nested expression error inside each container
		`x = [&]`,                           // tuple element
		`x = [1, &]`,                        // tuple later element
		`x = {a = &}`,                       // object value
		`x = {(&) = 1}`,                     // object key expression
		`x = f(&)`,                          // call argument
		`x = f(1, &)`,                       // call later argument
		`x = f(1, 2...&)`,                   // junk after spread expr
		`x = -&`,                            // unary operand
		`x = 1 + &`,                         // binary right operand
		`x = & + 1`,                         // binary left operand
		`x = & ? 1 : 2`,                     // conditional predicate
		`x = true ? & : 2`,                  // conditional then
		`x = true ? 1 : &`,                  // conditional else
		`x = (&)`,                           // grouped expression
		`x = a.&`,                           // postfix attr after error-ish (lex)
		`x = a[&]`,                          // index expression
		`x = a.*[&]`,                        // splat trailing index
		`x = [for v in & : v]`,              // for collection
		`x = [for v in xs : &]`,             // for body
		`x = [for v in xs : v if &]`,        // for if-cond
		`x = {for k,v in & : k=>v}`,         // object-for collection
		`x = {for k,v in xs : & => v}`,      // object-for key
		`x = {for k,v in xs : k => &}`,      // object-for value
		`x = {for k,v in xs : k => v if &}`, // object-for if
		// structural errors inside containers
		`x = f(1`,                     // unterminated call
		`x = [1`,                      // unterminated tuple
		`x = a.*[1`,                   // unterminated splat index
		`x = [for v in xs : v`,        // unterminated for-tuple
		`x = {for k,v in xs : k => v`, // unterminated for-object
		// body-level
		`b "lbl {` + "\n" + `}`,          // unterminated label string (lex)
		`b {` + "\n" + `  c = &` + "\n}", // error in nested block body
		`a = 1 b`,                        // junk where newline expected? (block start)
		`b { } x`,                        // trailing garbage after block on same line
	}
	for _, src := range bad {
		if _, err := Parse(src); err == nil {
			t.Errorf("Parse(%q) expected error", src)
		}
	}
}

// TestCallTrailingComma covers the call-arg trailing-comma-then-close branch and
// the empty-after-comma close.
func TestCallEdgeCases(t *testing.T) {
	// f(1,) trailing comma
	if got := attr(t, `max(1, 2,)`, nil); got != int64(2) {
		t.Errorf("trailing comma call => %v", got)
	}
	// f() empty
	ctx := NewContext()
	ctx.Functions["now"] = func(a []Value) (Value, error) { return "T", nil }
	if got := attr(t, `now()`, ctx); got != "T" {
		t.Errorf("empty call => %v", got)
	}
}

// TestValueEqualForeignType reaches valueEqual's fallthrough by comparing a value
// of a type outside the model (produced by a custom function).
func TestValueEqualForeignType(t *testing.T) {
	ctx := NewContext()
	ctx.Functions["weird"] = func(a []Value) (Value, error) { return struct{}{}, nil }
	// weird() == 1  -> the struct{} is not a known type, equality is false.
	if got := attr(t, `weird() == weird()`, ctx); got != false {
		t.Errorf("foreign-type equality => %v", got)
	}
}

// TestParseDocTrailing covers parseDocument's trailing-token rejection: a closing
// brace with no matching block.
func TestParseDocTrailing(t *testing.T) {
	if _, err := Parse("a = 1\n}\n"); err == nil {
		t.Error("expected trailing '}' error")
	}
}
