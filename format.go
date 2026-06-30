// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import (
	"errors"
	"strconv"
	"strings"
)

// biFormat implements HCL's format(): a printf-like string with the verbs
// %s, %d, %f, %g, %t, %q, %v, and %% (literal percent). Arguments are consumed
// left to right.
func biFormat(args []Value) (Value, error) {
	if len(args) < 1 {
		return nil, errors.New("format() requires a format string")
	}
	spec, ok := args[0].(string)
	if !ok {
		return nil, errors.New("format() spec must be a string")
	}
	rest := args[1:]
	var b strings.Builder
	ai := 0
	for i := 0; i < len(spec); i++ {
		c := spec[i]
		if c != '%' {
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(spec) {
			return nil, errors.New("format() dangling '%'")
		}
		verb := spec[i]
		if verb == '%' {
			b.WriteByte('%')
			continue
		}
		if ai >= len(rest) {
			return nil, errors.New("format() not enough arguments")
		}
		s, err := formatVerb(verb, rest[ai])
		if err != nil {
			return nil, err
		}
		b.WriteString(s)
		ai++
	}
	return b.String(), nil
}

func formatVerb(verb byte, v Value) (string, error) {
	switch verb {
	case 's':
		return formatAsString(v)
	case 'd':
		i, ok := v.(int64)
		if !ok {
			f, fok := toFloat(v)
			if !fok || f != float64(int64(f)) {
				return "", errors.New("format() %d requires an integer")
			}
			i = int64(f)
		}
		return strconv.FormatInt(i, 10), nil
	case 'f':
		f, ok := toFloat(v)
		if !ok {
			return "", errors.New("format() %f requires a number")
		}
		return strconv.FormatFloat(f, 'f', 6, 64), nil
	case 'g':
		f, ok := toFloat(v)
		if !ok {
			return "", errors.New("format() %g requires a number")
		}
		return formatFloat(f), nil
	case 't':
		bl, ok := v.(bool)
		if !ok {
			return "", errors.New("format() %t requires a bool")
		}
		if bl {
			return "true", nil
		}
		return "false", nil
	case 'q':
		s, err := formatAsString(v)
		if err != nil {
			return "", err
		}
		return strconv.Quote(s), nil
	case 'v':
		return formatAsString(v)
	}
	return "", errors.New("format() unknown verb %" + string(verb))
}

// formatAsString renders a primitive value as text (used by %s and %v).
func formatAsString(v Value) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case float64:
		return formatFloat(x), nil
	case bool:
		if x {
			return "true", nil
		}
		return "false", nil
	case nil:
		return "null", nil
	}
	return "", errors.New("format() cannot render a collection")
}
