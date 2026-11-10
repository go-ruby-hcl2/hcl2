// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import "testing"

// TestRubyValueModelInvariants pins the two evaluated behaviours that make this
// package a Ruby-value-model engine rather than a wrapper over
// github.com/hashicorp/hcl/v2 + zclconf/go-cty. Both were shown, by a control
// program run against hcl/v2 v2.24.0 and go-cty v1.19.0, to have NO cty
// equivalent:
//
//  1. String "+" concatenation ("a" + "b" -> "ab", Ruby String#+). cty's "+" is
//     numeric-only and rejects string operands.
//  2. Insertion-ordered objects ({z=1, a=2} -> keys [z, a], a Ruby Hash). A
//     cty.Object is unordered; its element iterator yields keys sorted ([a, z]),
//     and for a for-object the source order is discarded during evaluation and is
//     unrecoverable from the AST.
//
// These invariants are the reason this engine is from-scratch. Do not "refactor
// to wrap hashicorp/hcl": it is structurally infeasible without breaking them.
// See README.md "Relationship to hashicorp/hcl".
func TestRubyValueModelInvariants(t *testing.T) {
	// Invariant 1: string + concatenation (cty rejects this).
	got, err := EvalExpr(`"a" + "b"`, nil)
	if err != nil {
		t.Fatalf("string concat errored: %v", err)
	}
	if got != "ab" {
		t.Errorf("string concat = %#v, want \"ab\"", got)
	}

	// Invariant 2a: object-literal key order (cty sorts to [a z]).
	litOrder := attr(t, `{z = 1, a = 2}`, nil).(*Map).Pairs()
	if len(litOrder) != 2 || litOrder[0].Key != "z" || litOrder[1].Key != "a" {
		t.Errorf("object literal order = %#v, want [z a]", litOrder)
	}

	// Invariant 2b: for-object key order (cty sorts to [a z], unrecoverable).
	forOrder := attr(t, `{for k, v in {z = 1, a = 2} : k => v}`, nil).(*Map).Pairs()
	if len(forOrder) != 2 || forOrder[0].Key != "z" || forOrder[1].Key != "a" {
		t.Errorf("for-object order = %#v, want [z a]", forOrder)
	}
}
