// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import "strconv"

// Pos is a 1-based source position. Line and Col are 1-based; Offset is the
// 0-based byte index in the document.
type Pos struct {
	Line   int
	Col    int
	Offset int
}

// Diagnostic is a single error reported by the parser or evaluator, carrying the
// source position of the offending token — mirroring HCL's hcl.Diagnostic.
type Diagnostic struct {
	Summary string
	Pos     Pos
}

// Error renders the message with its `line:col` position.
func (d *Diagnostic) Error() string {
	return strconv.Itoa(d.Pos.Line) + ":" + strconv.Itoa(d.Pos.Col) + ": " + d.Summary
}

// Diagnostics is a non-empty collection of [Diagnostic], itself an error so it
// can flow through the standard error channel — mirroring hcl.Diagnostics.
type Diagnostics []*Diagnostic

// Error renders the first diagnostic, plus a count of any further ones, matching
// the way HCL surfaces the leading error to a CLI.
func (ds Diagnostics) Error() string {
	switch len(ds) {
	case 0:
		return "no diagnostics"
	case 1:
		return ds[0].Error()
	default:
		return ds[0].Error() + " (and " + strconv.Itoa(len(ds)-1) + " more diagnostic" + plural(len(ds)-1) + ")"
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
