// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package hcl2 is a pure-Go (CGO-free) from-scratch parser and evaluator for the
// HCL2 native syntax (HashiCorp Configuration Language v2), exposing its result
// as a Ruby value model. [Parse] turns a document into a lazy [*Body] (attributes
// and blocks, expressions unevaluated); [Eval] parses and evaluates a document
// against a variables/functions [*Context], returning a Ruby Hash (an
// insertion-ordered [*Map]). It is the deterministic, interpreter-independent
// HCL2 backend a host such as go-embedded-ruby binds (HCL2.parse / HCL2.eval →
// Hash), with no Ruby runtime.
//
// # Faithfulness
//
// There is no canonical Ruby HCL gem to mirror, so this package is faithful to
// the HCL2 native-syntax specification (the hashicorp/hcl v2 grammar). It is a
// clean-room pure-Go implementation mirroring the structure and semantics of the
// org's from-scratch C reference, github.com/libhcl/c-hcl2; nothing from
// hashicorp/hcl is vendored.
//
// # Ruby value model
//
// An evaluated document is an [any] drawn from a small, fixed set of Go types so
// a host can map its own object graph to and from this package:
//
//	HCL2                       Go (Eval returns)         Ruby (rbgo maps to)
//	----                       -----------------         -------------------
//	string / template          string                    String
//	number (integral)          int64                     Integer
//	number (fractional)        float64                   Float
//	bool                       bool                      true / false
//	null                       nil                       nil
//	tuple                      []any                     Array
//	object                     *Map (insertion order)    Hash
//
// HCL has a single number type; this package narrows an integral number to int64
// and a fractional one to float64 so the host materialises Integer vs Float
// naturally, exactly as it does for the TOML and JSON backends.
package hcl2

// Value is the interface satisfied by every value this package handles. It is
// purely documentary — the public API uses any — but a host may use it to
// constrain its own adapters.
type Value = any

// Pair is one entry of an ordered mapping.
type Pair struct {
	Key string
	Val Value
}

// Map is an insertion-ordered Ruby Hash. [Eval] returns the document root and
// every object value as a *Map so key order round-trips into Ruby.
type Map struct {
	pairs []Pair
	index map[string]int
}

// NewMap returns an empty ordered Map.
func NewMap() *Map { return &Map{index: map[string]int{}} }

// Len reports the number of entries.
func (m *Map) Len() int { return len(m.pairs) }

// Pairs returns the entries in insertion order. The slice must not be mutated.
func (m *Map) Pairs() []Pair { return m.pairs }

// Set inserts or replaces the entry for key, preserving first-insertion order.
func (m *Map) Set(key string, val Value) {
	if m.index == nil {
		m.index = map[string]int{}
	}
	if i, ok := m.index[key]; ok {
		m.pairs[i].Val = val
		return
	}
	m.index[key] = len(m.pairs)
	m.pairs = append(m.pairs, Pair{Key: key, Val: val})
}

// Get returns the value for key and whether it was present.
func (m *Map) Get(key string) (Value, bool) {
	if m.index == nil {
		return nil, false
	}
	if i, ok := m.index[key]; ok {
		return m.pairs[i].Val, true
	}
	return nil, false
}
