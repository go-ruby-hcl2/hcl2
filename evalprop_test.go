// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import (
	"reflect"
	"testing"
)

// TestEvalErrorPropagation drives an evaluation error (an unknown variable
// `undef`) through every sub-evaluation site so each error-propagation branch in
// the evaluator is exercised.
func TestEvalErrorPropagation(t *testing.T) {
	ctx := NewContext()
	ctx.Variables["obj"] = func() *Map { m := NewMap(); m.Set("k", int64(1)); return m }()
	ctx.Variables["tup"] = []Value{int64(1), int64(2)}
	ctx.Variables["m"] = func() *Map { m := NewMap(); m.Set("a", int64(1)); return m }()

	bad := []string{
		`x = undef.k`,                          // evalAttr: object eval error
		`x = undef[0]`,                         // evalIndex: coll eval error
		`x = tup[undef]`,                       // evalIndex: index eval error
		`x = -undef`,                           // evalUnary operand error
		`x = undef ? 1 : 2`,                    // evalCond predicate error
		`x = [undef]`,                          // evalTuple element error
		`x = {a = undef}`,                      // evalObject value error
		`x = {(undef) = 1}`,                    // evalObject key error
		`x = [for v in undef : v]`,             // evalForTuple coll error
		`x = [for v in tup : undef]`,           // evalForTuple body error
		`x = [for v in tup : v if undef]`,      // evalForTuple cond error
		`x = {for k,v in undef : k=>v}`,        // evalForObject coll error
		`x = {for k,v in m : undef => v}`,      // evalForObject key error
		`x = {for k,v in m : k => undef}`,      // evalForObject value error
		`x = {for k,v in m : k => v if undef}`, // evalForObject cond error
		`x = 1 + undef`,                        // binary right error
		`x = undef + 1`,                        // binary left error
		`x = undef && true`,                    // logical left error
		`x = true && undef`,                    // logical right error
		`x = undef(1)`,                         // (custom path skipped) unknown function via call args eval
		`x = upper(undef)`,                     // call argument eval error
		`x = max(1, undef...)`,                 // spread argument eval error
		`x = "${undef}"`,                       // template interp eval error
		`x = jsonencode(undef)`,                // function arg eval error
	}
	for _, src := range bad {
		if _, err := Eval(src+"\n", ctx); err == nil {
			t.Errorf("Eval(%q) expected eval error", src)
		}
	}
}

// TestIndexFloatWhole covers indexInt's float-with-integral-value branch.
func TestIndexFloatWhole(t *testing.T) {
	ctx := NewContext()
	ctx.Variables["tup"] = []Value{"a", "b", "c"}
	ctx.Variables["f"] = 1.0
	if got := attr(t, `tup[f]`, ctx); got != "b" {
		t.Errorf("float-whole index => %v", got)
	}
}

// TestThreeBlocksCollect covers mergeBlock's []Value accumulation branch (a third
// repeated block appends to the existing tuple).
func TestThreeBlocksCollect(t *testing.T) {
	src := `
r { n = 1 }
r { n = 2 }
r { n = 3 }
`
	m := evalOK(t, src, nil)
	v, _ := m.Get("r")
	tup, ok := v.([]Value)
	if !ok || len(tup) != 3 {
		t.Fatalf("three blocks => %#v", v)
	}
	if n, _ := tup[2].(*Map).Get("n"); n != int64(3) {
		t.Errorf("r[2].n => %v", n)
	}
}

// TestJSONNestedSuccess covers the jsonencode tuple/object recursion success
// branches and jsondecode nested success branches.
func TestJSONRoundTrip(t *testing.T) {
	enc := attr(t, `jsonencode([{a=1}, [true], "s", 2.5, null, false])`, nil)
	want := `[{"a":1},[true],"s",2.5,null,false]`
	if enc != want {
		t.Errorf("jsonencode => %q want %q", enc, want)
	}
	dec := attr(t, `jsondecode("[{\"a\": 1}, [true], \"s\"]")`, nil)
	exp := []Value{
		func() Value { m := NewMap(); m.Set("a", int64(1)); return m }(),
		[]Value{true},
		"s",
	}
	if !reflect.DeepEqual(dec, exp) {
		t.Errorf("jsondecode => %#v", dec)
	}
}

// TestTryReturnsLastError covers evalTry's path where every argument errors and
// the last diagnostic is returned.
func TestTryAllFail(t *testing.T) {
	if _, err := Eval("x = try(a, b, c)\n", nil); err == nil {
		t.Error("try over all-failing args should error")
	}
}
