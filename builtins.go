// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// builtins is the pure standard-library subset shipped by this package. Each is
// a [Func]; a host may shadow any of them via [Context.Functions]. See the
// README for the full list and semantics.
var builtins = map[string]Func{
	"length":     biLength,
	"upper":      biUpper,
	"lower":      biLower,
	"min":        biMin,
	"max":        biMax,
	"abs":        biAbs,
	"floor":      biFloor,
	"ceil":       biCeil,
	"concat":     biConcat,
	"join":       biJoin,
	"split":      biSplit,
	"keys":       biKeys,
	"values":     biValues,
	"lookup":     biLookup,
	"contains":   biContains,
	"coalesce":   biCoalesce,
	"format":     biFormat,
	"jsonencode": biJSONEncode,
	"jsondecode": biJSONDecode,
	"tostring":   biToString,
	"tonumber":   biToNumber,
	"tobool":     biToBool,
	"reverse":    biReverse,
	"trimspace":  biTrimSpace,
}

var errArgCount = errors.New("wrong number of arguments")

func argN(args []Value, n int) error {
	if len(args) != n {
		return errArgCount
	}
	return nil
}

func biLength(args []Value) (Value, error) {
	if err := argN(args, 1); err != nil {
		return nil, err
	}
	switch x := args[0].(type) {
	case string:
		return int64(utf8.RuneCountInString(x)), nil
	case []Value:
		return int64(len(x)), nil
	case *Map:
		return int64(x.Len()), nil
	}
	return nil, errors.New("length() requires a string, tuple, or object")
}

func biUpper(args []Value) (Value, error) {
	s, err := arg1String(args)
	if err != nil {
		return nil, err
	}
	return strings.ToUpper(s), nil
}

func biLower(args []Value) (Value, error) {
	s, err := arg1String(args)
	if err != nil {
		return nil, err
	}
	return strings.ToLower(s), nil
}

func biTrimSpace(args []Value) (Value, error) {
	s, err := arg1String(args)
	if err != nil {
		return nil, err
	}
	return strings.TrimSpace(s), nil
}

func arg1String(args []Value) (string, error) {
	if err := argN(args, 1); err != nil {
		return "", err
	}
	s, ok := args[0].(string)
	if !ok {
		return "", errors.New("expected a string argument")
	}
	return s, nil
}

// numericFold reduces a non-empty numeric argument list with pick, preserving
// int64-ness when all inputs are int64.
func numericFold(args []Value, name string, pick func(a, b float64) float64) (Value, error) {
	if len(args) == 0 {
		return nil, errors.New(name + "() requires at least one argument")
	}
	allInt := true
	acc, ok := toFloat(args[0])
	if !ok {
		return nil, errors.New(name + "() requires numbers")
	}
	if _, isInt := args[0].(int64); !isInt {
		allInt = false
	}
	for _, a := range args[1:] {
		f, ok := toFloat(a)
		if !ok {
			return nil, errors.New(name + "() requires numbers")
		}
		if _, isInt := a.(int64); !isInt {
			allInt = false
		}
		acc = pick(acc, f)
	}
	if allInt {
		return int64(acc), nil
	}
	return acc, nil
}

func biMin(args []Value) (Value, error) {
	return numericFold(args, "min", math.Min)
}

func biMax(args []Value) (Value, error) {
	return numericFold(args, "max", math.Max)
}

func biAbs(args []Value) (Value, error) {
	if err := argN(args, 1); err != nil {
		return nil, err
	}
	switch x := args[0].(type) {
	case int64:
		if x < 0 {
			return -x, nil
		}
		return x, nil
	case float64:
		return math.Abs(x), nil
	}
	return nil, errors.New("abs() requires a number")
}

func biFloor(args []Value) (Value, error) {
	f, err := arg1Float(args, "floor")
	if err != nil {
		return nil, err
	}
	return int64(math.Floor(f)), nil
}

func biCeil(args []Value) (Value, error) {
	f, err := arg1Float(args, "ceil")
	if err != nil {
		return nil, err
	}
	return int64(math.Ceil(f)), nil
}

func arg1Float(args []Value, name string) (float64, error) {
	if err := argN(args, 1); err != nil {
		return 0, err
	}
	f, ok := toFloat(args[0])
	if !ok {
		return 0, errors.New(name + "() requires a number")
	}
	return f, nil
}

func biConcat(args []Value) (Value, error) {
	out := []Value{}
	for _, a := range args {
		t, ok := a.([]Value)
		if !ok {
			return nil, errors.New("concat() requires tuple arguments")
		}
		out = append(out, t...)
	}
	return out, nil
}

func biJoin(args []Value) (Value, error) {
	if len(args) < 1 {
		return nil, errors.New("join() requires a separator")
	}
	sep, ok := args[0].(string)
	if !ok {
		return nil, errors.New("join() separator must be a string")
	}
	var parts []string
	for _, a := range args[1:] {
		t, ok := a.([]Value)
		if !ok {
			return nil, errors.New("join() requires tuple arguments")
		}
		for _, e := range t {
			s, ok := e.(string)
			if !ok {
				return nil, errors.New("join() elements must be strings")
			}
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, sep), nil
}

func biSplit(args []Value) (Value, error) {
	if err := argN(args, 2); err != nil {
		return nil, err
	}
	sep, ok1 := args[0].(string)
	s, ok2 := args[1].(string)
	if !ok1 || !ok2 {
		return nil, errors.New("split() requires two strings")
	}
	parts := strings.Split(s, sep)
	out := make([]Value, len(parts))
	for i, p := range parts {
		out[i] = p
	}
	return out, nil
}

func biKeys(args []Value) (Value, error) {
	if err := argN(args, 1); err != nil {
		return nil, err
	}
	m, ok := args[0].(*Map)
	if !ok {
		return nil, errors.New("keys() requires an object")
	}
	keys := make([]string, 0, m.Len())
	for _, pr := range m.Pairs() {
		keys = append(keys, pr.Key)
	}
	sort.Strings(keys)
	out := make([]Value, len(keys))
	for i, k := range keys {
		out[i] = k
	}
	return out, nil
}

func biValues(args []Value) (Value, error) {
	if err := argN(args, 1); err != nil {
		return nil, err
	}
	m, ok := args[0].(*Map)
	if !ok {
		return nil, errors.New("values() requires an object")
	}
	keys := make([]string, 0, m.Len())
	for _, pr := range m.Pairs() {
		keys = append(keys, pr.Key)
	}
	sort.Strings(keys)
	out := make([]Value, len(keys))
	for i, k := range keys {
		v, _ := m.Get(k)
		out[i] = v
	}
	return out, nil
}

func biLookup(args []Value) (Value, error) {
	if len(args) != 2 && len(args) != 3 {
		return nil, errors.New("lookup() requires 2 or 3 arguments")
	}
	m, ok := args[0].(*Map)
	if !ok {
		return nil, errors.New("lookup() requires an object")
	}
	key, ok := args[1].(string)
	if !ok {
		return nil, errors.New("lookup() key must be a string")
	}
	if v, ok := m.Get(key); ok {
		return v, nil
	}
	if len(args) == 3 {
		return args[2], nil
	}
	return nil, errors.New("lookup() key not found and no default given")
}

func biContains(args []Value) (Value, error) {
	if err := argN(args, 2); err != nil {
		return nil, err
	}
	t, ok := args[0].([]Value)
	if !ok {
		return nil, errors.New("contains() requires a tuple")
	}
	for _, e := range t {
		if valueEqual(e, args[1]) {
			return true, nil
		}
	}
	return false, nil
}

func biCoalesce(args []Value) (Value, error) {
	for _, a := range args {
		if a != nil {
			return a, nil
		}
	}
	return nil, errors.New("coalesce() received no non-null arguments")
}

func biReverse(args []Value) (Value, error) {
	if err := argN(args, 1); err != nil {
		return nil, err
	}
	t, ok := args[0].([]Value)
	if !ok {
		return nil, errors.New("reverse() requires a tuple")
	}
	out := make([]Value, len(t))
	for i, e := range t {
		out[len(t)-1-i] = e
	}
	return out, nil
}

func biToString(args []Value) (Value, error) {
	if err := argN(args, 1); err != nil {
		return nil, err
	}
	switch x := args[0].(type) {
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
	}
	return nil, errors.New("tostring() requires a primitive value")
}

func biToNumber(args []Value) (Value, error) {
	if err := argN(args, 1); err != nil {
		return nil, err
	}
	switch x := args[0].(type) {
	case int64, float64:
		return x, nil
	case string:
		v, ok := parseNumber(x)
		if !ok {
			return nil, errors.New("tonumber() could not parse the string")
		}
		return v, nil
	}
	return nil, errors.New("tonumber() requires a number or numeric string")
}

func biToBool(args []Value) (Value, error) {
	if err := argN(args, 1); err != nil {
		return nil, err
	}
	switch x := args[0].(type) {
	case bool:
		return x, nil
	case string:
		switch x {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
	}
	return nil, errors.New("tobool() requires a bool or \"true\"/\"false\"")
}
