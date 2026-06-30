// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import (
	"math"
	"strconv"
)

// evalBinary evaluates a binary operator. `&&` and `||` short-circuit.
func (ev *evaluator) evalBinary(n *binaryExpr) (Value, *Diagnostic) {
	switch n.op {
	case tAnd, tOr:
		return ev.evalLogical(n)
	}
	l, d := ev.eval(n.l)
	if d != nil {
		return nil, d
	}
	r, d := ev.eval(n.r)
	if d != nil {
		return nil, d
	}
	switch n.op {
	case tPlus, tMinus, tStar, tSlash, tPercent:
		return arith(n.op, l, r, n.pos)
	case tLt, tLe, tGt, tGe:
		return compare(n.op, l, r, n.pos)
	case tEq:
		return valueEqual(l, r), nil
	default: // tNe
		return !valueEqual(l, r), nil
	}
}

func (ev *evaluator) evalLogical(n *binaryExpr) (Value, *Diagnostic) {
	lv, d := ev.eval(n.l)
	if d != nil {
		return nil, d
	}
	lb, ok := lv.(bool)
	if !ok {
		return nil, &Diagnostic{Summary: "logical operator requires bool operands", Pos: n.pos}
	}
	if n.op == tAnd && !lb {
		return false, nil
	}
	if n.op == tOr && lb {
		return true, nil
	}
	rv, d := ev.eval(n.r)
	if d != nil {
		return nil, d
	}
	rb, ok := rv.(bool)
	if !ok {
		return nil, &Diagnostic{Summary: "logical operator requires bool operands", Pos: n.pos}
	}
	return rb, nil
}

// toFloat coerces a numeric Ruby value to float64.
func toFloat(v Value) (float64, bool) {
	switch x := v.(type) {
	case int64:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}

// arith applies +,-,*,/,% with number coercion. `+` also concatenates strings.
func arith(op tokKind, l, r Value, pos Pos) (Value, *Diagnostic) {
	if op == tPlus {
		ls, lok := l.(string)
		rs, rok := r.(string)
		if lok && rok {
			return ls + rs, nil
		}
	}
	li, liok := l.(int64)
	ri, riok := r.(int64)
	bothInt := liok && riok
	lf, lok := toFloat(l)
	rf, rok := toFloat(r)
	if !lok || !rok {
		return nil, &Diagnostic{Summary: "arithmetic requires numbers", Pos: pos}
	}
	switch op {
	case tPlus:
		if bothInt {
			return li + ri, nil
		}
		return lf + rf, nil
	case tMinus:
		if bothInt {
			return li - ri, nil
		}
		return lf - rf, nil
	case tStar:
		if bothInt {
			return li * ri, nil
		}
		return lf * rf, nil
	case tSlash:
		if rf == 0 {
			return nil, &Diagnostic{Summary: "division by zero", Pos: pos}
		}
		if bothInt && li%ri == 0 {
			return li / ri, nil
		}
		return lf / rf, nil
	default: // tPercent
		if rf == 0 {
			return nil, &Diagnostic{Summary: "modulo by zero", Pos: pos}
		}
		if bothInt {
			return li % ri, nil
		}
		return math.Mod(lf, rf), nil
	}
}

// compare applies <,<=,>,>= over numbers.
func compare(op tokKind, l, r Value, pos Pos) (Value, *Diagnostic) {
	lf, lok := toFloat(l)
	rf, rok := toFloat(r)
	if !lok || !rok {
		return nil, &Diagnostic{Summary: "comparison requires numbers", Pos: pos}
	}
	switch op {
	case tLt:
		return lf < rf, nil
	case tLe:
		return lf <= rf, nil
	case tGt:
		return lf > rf, nil
	default: // tGe
		return lf >= rf, nil
	}
}

// valueEqual implements HCL value equality across the Ruby model. Numbers
// compare by value across int64/float64; tuples and objects compare deeply.
func valueEqual(a, b Value) bool {
	if af, aok := toFloat(a); aok {
		if bf, bok := toFloat(b); bok {
			return af == bf
		}
		return false
	}
	switch x := a.(type) {
	case string:
		y, ok := b.(string)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case nil:
		return b == nil
	case []Value:
		y, ok := b.([]Value)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !valueEqual(x[i], y[i]) {
				return false
			}
		}
		return true
	case *Map:
		y, ok := b.(*Map)
		if !ok || x.Len() != y.Len() {
			return false
		}
		for _, pr := range x.Pairs() {
			yv, ok := y.Get(pr.Key)
			if !ok || !valueEqual(pr.Val, yv) {
				return false
			}
		}
		return true
	}
	return false
}

// formatFloat renders a float64 the way HCL interpolation does: shortest round-
// trippable decimal.
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}
