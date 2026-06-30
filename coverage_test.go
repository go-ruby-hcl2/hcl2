// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestComments exercises every comment style and the lineComment helper.
func TestComments(t *testing.T) {
	src := `
# hash comment
// slash comment
a = 1 // trailing
/* block
   comment */
b = 2
`
	m := evalOK(t, src, nil)
	if v, _ := m.Get("a"); v != int64(1) {
		t.Errorf("a %v", v)
	}
	if v, _ := m.Get("b"); v != int64(2) {
		t.Errorf("b %v", v)
	}
	// comment at very end of file with no newline.
	m = evalOK(t, "a = 1 # end", nil)
	if v, _ := m.Get("a"); v != int64(1) {
		t.Errorf("end-comment a %v", v)
	}
}

// TestNumberForms covers integer/float/exponent and the int-out-of-range fall
// back to float64.
func TestNumberForms(t *testing.T) {
	if v := attr(t, `100`, nil); v != int64(100) {
		t.Errorf("int %v", v)
	}
	// A magnitude beyond int64 falls back to float64.
	big := attr(t, `99999999999999999999`, nil)
	if _, ok := big.(float64); !ok {
		t.Errorf("big number type %T", big)
	}
	if v := attr(t, `0`, nil); v != int64(0) {
		t.Errorf("zero %v", v)
	}
}

// TestEvalFile covers EvalFile and its read-error path.
func TestEvalFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.hcl")
	if err := os.WriteFile(p, []byte("a = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := EvalFile(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := m.Get("a"); v != int64(1) {
		t.Errorf("a %v", v)
	}
	if _, err := EvalFile(filepath.Join(dir, "nope.hcl"), nil); err == nil {
		t.Error("expected read error")
	}
	// a parse error inside a file surfaces too.
	bad := filepath.Join(dir, "bad.hcl")
	os.WriteFile(bad, []byte("a = "), 0o644)
	if _, err := EvalFile(bad, nil); err == nil {
		t.Error("expected parse error")
	}
}

// TestEvalExprErrors covers EvalExpr parse / trailing / eval error paths.
func TestEvalExprErrors(t *testing.T) {
	if _, err := EvalExpr("1 +", nil); err == nil {
		t.Error("expected parse error")
	}
	if _, err := EvalExpr("1 2", nil); err == nil {
		t.Error("expected trailing-token error")
	}
	if _, err := EvalExpr("undefined", nil); err == nil {
		t.Error("expected eval error")
	}
	if _, err := EvalExpr("& bad", nil); err == nil {
		t.Error("expected lex error")
	}
}

// TestAttrError covers the Attr lazy-decode error path.
func TestAttrError(t *testing.T) {
	body, err := Parse("a = undefined_ref\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := body.Attr("a", nil); !ok || err == nil {
		t.Errorf("Attr error path: ok=%v err=%v", ok, err)
	}
}

// TestParseTopErrors covers the document-level "expected attribute or block"
// trailing-token branch and an empty body.
func TestParseTopErrors(t *testing.T) {
	if _, err := Parse(""); err != nil {
		t.Errorf("empty doc should parse: %v", err)
	}
	if _, err := Parse("   \n  \n"); err != nil {
		t.Errorf("blank doc should parse: %v", err)
	}
	// a stray closing brace at top level.
	if _, err := Parse("}"); err == nil {
		t.Error("expected error on stray '}'")
	}
}

// TestDiagnostics covers the Diagnostics rendering branches.
func TestDiagnostics(t *testing.T) {
	d1 := &Diagnostic{Summary: "first", Pos: Pos{Line: 2, Col: 3, Offset: 7}}
	d2 := &Diagnostic{Summary: "second", Pos: Pos{Line: 4, Col: 1}}
	if got := (Diagnostics{}).Error(); got != "no diagnostics" {
		t.Errorf("empty diags %q", got)
	}
	if got := (Diagnostics{d1}).Error(); got != "2:3: first" {
		t.Errorf("single %q", got)
	}
	multi := Diagnostics{d1, d2}.Error()
	if !strings.Contains(multi, "2:3: first") || !strings.Contains(multi, "and 1 more diagnostic)") {
		t.Errorf("multi %q", multi)
	}
	if got := plural(0); got != "s" {
		t.Errorf("plural(0) %q", got)
	}
	if got := plural(1); got != "" {
		t.Errorf("plural(1) %q", got)
	}
	if got := plural(3); got != "s" {
		t.Errorf("plural(3) %q", got)
	}
	// three diagnostics → "2 more diagnostics" (plural)
	three := Diagnostics{d1, d2, d1}.Error()
	if !strings.Contains(three, "2 more diagnostics)") {
		t.Errorf("three %q", three)
	}
}

// TestMapAPI covers Map.Set replace, Get on a nil-index map, and zero value.
func TestMapAPI(t *testing.T) {
	var m Map // zero value, nil index
	if _, ok := m.Get("x"); ok {
		t.Error("zero map Get should miss")
	}
	m.Set("a", 1) // initialises index
	m.Set("a", 2) // replace path
	if v, _ := m.Get("a"); v != 2 {
		t.Errorf("replace %v", v)
	}
	if m.Len() != 1 {
		t.Errorf("len %d", m.Len())
	}
	if len(m.Pairs()) != 1 {
		t.Errorf("pairs %d", len(m.Pairs()))
	}
}

// TestValueEqualNumberMix covers number-vs-non-number equality and nested deep
// equality across the model.
func TestValueEqualNumberMix(t *testing.T) {
	if attr(t, `1 == "x"`, nil) != false {
		t.Error("num vs string")
	}
	if attr(t, `[{a=1}] == [{a=1}]`, nil) != true {
		t.Error("nested deep eq")
	}
	if attr(t, `[{a=1}] == [{a=2}]`, nil) != false {
		t.Error("nested deep neq")
	}
}

// TestStringKeyTraversal exercises an integer index against an object whose key
// came from integer coercion, plus object key from quoted string with escape.
func TestQuotedObjectKey(t *testing.T) {
	m := attr(t, `{ "a\tb" = 1 }`, nil).(*Map)
	if _, ok := m.Get("a\tb"); !ok {
		t.Errorf("escaped quoted key missing: %#v", m.Pairs())
	}
}

// TestBlockTypeBothWays covers a type used as both a labelled and unlabelled
// block (accumulates into a tuple) and the labelled-merge branch.
func TestBlockMergeBranches(t *testing.T) {
	// labelled then unlabelled of the same type -> tuple accumulation.
	src := `
t "x" {
  a = 1
}
t {
  b = 2
}
`
	m := evalOK(t, src, nil)
	v, _ := m.Get("t")
	if _, ok := v.([]Value); !ok {
		t.Errorf("mixed block kinds should accumulate: %#v", v)
	}
}

// TestForTupleKeyVarObject covers iterating an object with an explicit key var
// inside a for-object grouping where order is preserved.
func TestForObjectOrder(t *testing.T) {
	g := attr(t, `{for k, v in {z=1, a=2} : k => v}`, nil).(*Map)
	ps := g.Pairs()
	if len(ps) != 2 || ps[0].Key != "z" || ps[1].Key != "a" {
		t.Errorf("order not preserved: %#v", ps)
	}
}
