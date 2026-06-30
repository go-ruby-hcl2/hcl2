// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import "testing"

// TestArgCountErrors calls each fixed-arity builtin with the wrong number of
// arguments so every argN / arg1* guard is exercised.
func TestArgCountErrors(t *testing.T) {
	wrong := []string{
		`upper()`, `upper("a", "b")`,
		`lower()`,
		`trimspace()`,
		`abs()`,
		`floor()`, `floor(1, 2)`,
		`ceil()`,
		`split("a")`, `split("a", "b", "c")`,
		`keys()`, `keys({}, {})`,
		`values()`,
		`contains([1])`, `contains([1], 2, 3)`,
		`reverse()`,
		`tostring()`, `tostring(1, 2)`,
		`tonumber()`,
		`tobool()`,
		`length()`, `length("a", "b")`,
		`jsonencode()`, `jsonencode(1, 2)`,
		`jsondecode()`, `jsondecode("1", "2")`,
		`format()`,
	}
	for _, src := range wrong {
		if _, err := Eval("x = "+src+"\n", nil); err == nil {
			t.Errorf("%s expected arg-count error", src)
		}
	}
}

// TestFalseRenderPaths covers the false-branch of every bool renderer.
func TestFalseRenderPaths(t *testing.T) {
	if got := attr(t, `tostring(false)`, nil); got != "false" {
		t.Errorf("tostring(false) => %v", got)
	}
	if got := attr(t, `format("%t", false)`, nil); got != "false" {
		t.Errorf("format %%t false => %v", got)
	}
	if got := attr(t, `format("%s", false)`, nil); got != "false" {
		t.Errorf("format %%s false => %v", got)
	}
	ctx := NewContext()
	ctx.Variables["b"] = false
	if got := attr(t, `"v=${b}"`, ctx); got != "v=false" {
		t.Errorf("interp false => %v", got)
	}
}

// TestFloatArithBranches covers the float (non-both-int) arms of each operator.
func TestFloatArithBranches(t *testing.T) {
	cases := []struct {
		src  string
		want Value
	}{
		{`1.5 - 0.5`, float64(1)},
		{`1.5 * 2.0`, float64(3)},
		{`7 / 2.0`, 3.5},
		{`1.5 < 2.0`, true},
		{`2.5 <= 2.5`, true},
		{`3.5 > 2.0`, true},
		{`3.5 >= 3.5`, true},
		{`5 - 3`, int64(2)},
	}
	for _, c := range cases {
		if got := attr(t, c.src, nil); got != c.want {
			t.Errorf("%s => %v, want %v", c.src, got, c.want)
		}
	}
}
