// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import (
	"math"
	"strconv"
)

// parseNumber turns a numeric lexeme into a Ruby value: an int64 when the value
// is integral and fits, otherwise a float64. It returns false on a malformed
// lexeme.
func parseNumber(s string) (Value, bool) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, false
	}
	if isIntegral(s, f) {
		i, err := strconv.ParseInt(s, 10, 64)
		if err == nil {
			return i, true
		}
	}
	return f, true
}

// isIntegral reports whether the lexeme denotes a whole number representable as
// int64 (no '.', no exponent, and within range).
func isIntegral(s string, f float64) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' || c == 'e' || c == 'E' {
			return false
		}
	}
	return f == math.Trunc(f) && f >= math.MinInt64 && f <= math.MaxInt64
}
