// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import (
	"errors"
	"strconv"
	"strings"
)

// biJSONEncode renders a Ruby value to a compact JSON string, mirroring HCL's
// jsonencode (objects in insertion order, no trailing whitespace).
func biJSONEncode(args []Value) (Value, error) {
	if err := argN(args, 1); err != nil {
		return nil, err
	}
	var b strings.Builder
	if err := jsonEncode(&b, args[0]); err != nil {
		return nil, err
	}
	return b.String(), nil
}

func jsonEncode(b *strings.Builder, v Value) error {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case float64:
		b.WriteString(formatFloat(x))
	case string:
		jsonString(b, x)
	case []Value:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := jsonEncode(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case *Map:
		b.WriteByte('{')
		for i, pr := range x.Pairs() {
			if i > 0 {
				b.WriteByte(',')
			}
			jsonString(b, pr.Key)
			b.WriteByte(':')
			if err := jsonEncode(b, pr.Val); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return errors.New("jsonencode() cannot encode this value")
	}
	return nil
}

func jsonString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\t':
			b.WriteString("\\t")
		case '\r':
			b.WriteString("\\r")
		default:
			if r < 0x20 {
				b.WriteString("\\u")
				const hex = "0123456789abcdef"
				b.WriteByte('0')
				b.WriteByte('0')
				b.WriteByte(hex[(r>>4)&0xf])
				b.WriteByte(hex[r&0xf])
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// biJSONDecode parses a JSON string into the Ruby value model.
func biJSONDecode(args []Value) (Value, error) {
	s, err := arg1String(args)
	if err != nil {
		return nil, err
	}
	d := &jsonDec{s: s}
	d.ws()
	v, err := d.value()
	if err != nil {
		return nil, err
	}
	d.ws()
	if d.i != len(d.s) {
		return nil, errors.New("jsondecode() trailing data after JSON value")
	}
	return v, nil
}

// jsonDec is a minimal recursive-descent JSON decoder producing *Map/[]Value/
// string/int64/float64/bool/nil.
type jsonDec struct {
	s string
	i int
}

func (d *jsonDec) ws() {
	for d.i < len(d.s) {
		switch d.s[d.i] {
		case ' ', '\t', '\n', '\r':
			d.i++
		default:
			return
		}
	}
}

func (d *jsonDec) value() (Value, error) {
	if d.i >= len(d.s) {
		return nil, errors.New("jsondecode() unexpected end of input")
	}
	switch c := d.s[d.i]; {
	case c == '{':
		return d.object()
	case c == '[':
		return d.array()
	case c == '"':
		return d.str()
	case c == 't', c == 'f':
		return d.boolean()
	case c == 'n':
		return d.null()
	case c == '-' || (c >= '0' && c <= '9'):
		return d.number()
	}
	return nil, errors.New("jsondecode() unexpected character")
}

func (d *jsonDec) object() (Value, error) {
	d.i++ // '{'
	m := NewMap()
	d.ws()
	if d.i < len(d.s) && d.s[d.i] == '}' {
		d.i++
		return m, nil
	}
	for {
		d.ws()
		if d.i >= len(d.s) || d.s[d.i] != '"' {
			return nil, errors.New("jsondecode() expected object key")
		}
		key, err := d.str()
		if err != nil {
			return nil, err
		}
		d.ws()
		if d.i >= len(d.s) || d.s[d.i] != ':' {
			return nil, errors.New("jsondecode() expected ':'")
		}
		d.i++
		d.ws()
		v, err := d.value()
		if err != nil {
			return nil, err
		}
		m.Set(key.(string), v)
		d.ws()
		if d.i >= len(d.s) {
			return nil, errors.New("jsondecode() unterminated object")
		}
		if d.s[d.i] == ',' {
			d.i++
			continue
		}
		if d.s[d.i] == '}' {
			d.i++
			return m, nil
		}
		return nil, errors.New("jsondecode() expected ',' or '}'")
	}
}

func (d *jsonDec) array() (Value, error) {
	d.i++ // '['
	out := []Value{}
	d.ws()
	if d.i < len(d.s) && d.s[d.i] == ']' {
		d.i++
		return out, nil
	}
	for {
		d.ws()
		v, err := d.value()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		d.ws()
		if d.i >= len(d.s) {
			return nil, errors.New("jsondecode() unterminated array")
		}
		if d.s[d.i] == ',' {
			d.i++
			continue
		}
		if d.s[d.i] == ']' {
			d.i++
			return out, nil
		}
		return nil, errors.New("jsondecode() expected ',' or ']'")
	}
}

func (d *jsonDec) str() (Value, error) {
	d.i++ // opening quote
	var b strings.Builder
	for d.i < len(d.s) {
		c := d.s[d.i]
		if c == '"' {
			d.i++
			return b.String(), nil
		}
		if c == '\\' {
			d.i++
			if d.i >= len(d.s) {
				break
			}
			switch d.s[d.i] {
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case '/':
				b.WriteByte('/')
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'u':
				if d.i+4 >= len(d.s) {
					return nil, errors.New("jsondecode() truncated \\u escape")
				}
				v, err := strconv.ParseUint(d.s[d.i+1:d.i+5], 16, 32)
				if err != nil {
					return nil, errors.New("jsondecode() invalid \\u escape")
				}
				b.WriteRune(rune(v))
				d.i += 4
			default:
				return nil, errors.New("jsondecode() invalid escape")
			}
			d.i++
			continue
		}
		b.WriteByte(c)
		d.i++
	}
	return nil, errors.New("jsondecode() unterminated string")
}

func (d *jsonDec) boolean() (Value, error) {
	if strings.HasPrefix(d.s[d.i:], "true") {
		d.i += 4
		return true, nil
	}
	if strings.HasPrefix(d.s[d.i:], "false") {
		d.i += 5
		return false, nil
	}
	return nil, errors.New("jsondecode() invalid literal")
}

func (d *jsonDec) null() (Value, error) {
	if strings.HasPrefix(d.s[d.i:], "null") {
		d.i += 4
		return nil, nil
	}
	return nil, errors.New("jsondecode() invalid literal")
}

func (d *jsonDec) number() (Value, error) {
	start := d.i
	for d.i < len(d.s) {
		c := d.s[d.i]
		if (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E' {
			d.i++
			continue
		}
		break
	}
	lit := d.s[start:d.i]
	v, ok := parseNumber(lit)
	if !ok {
		return nil, errors.New("jsondecode() invalid number")
	}
	return v, nil
}
