// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

// parser is a hand-written recursive-descent + Pratt parser. The whole input is
// tokenized up front into a slice (with any single lexical error captured as a
// terminal tError token), so token advancement is infallible: bump never errors,
// and a lexical error surfaces wherever the parser next consumes the tError
// token. This keeps the grammar code free of per-advance error plumbing.
type parser struct {
	toks []token
	i    int   // index of the current token
	tok  token // toks[i], the current look-ahead token
	lerr *Diagnostic
}

func newParser(src string) *parser {
	lx := newLexer(src)
	var toks []token
	var lerr *Diagnostic
	for {
		t, d := lx.next()
		if d != nil {
			lerr = d
			toks = append(toks, token{kind: tError, pos: d.Pos})
			break
		}
		toks = append(toks, t)
		if t.kind == tEOF {
			break
		}
	}
	p := &parser{toks: toks, lerr: lerr}
	p.tok = toks[0]
	return p
}

// bump advances to the next token. Advancing never moves past the final token
// (tEOF or tError), so repeated bumps at end-of-input are stable.
func (p *parser) bump() {
	if p.i < len(p.toks)-1 {
		p.i++
	}
	p.tok = p.toks[p.i]
}

// skipNewlines consumes any run of newline tokens (used where newlines are not
// significant, e.g. inside brackets and around operators).
func (p *parser) skipNewlines() {
	for p.tok.kind == tNewline {
		p.bump()
	}
}

// errf builds a diagnostic at pos. When the parser has stopped on the terminal
// tError sentinel, the original lexical error is surfaced instead of the
// grammar-level "expected X" message, so a lexer failure is always reported with
// its true cause.
func (p *parser) errf(pos Pos, msg string) *Diagnostic {
	if p.tok.kind == tError {
		return p.lerr
	}
	return &Diagnostic{Summary: msg, Pos: pos}
}

// expect consumes a token of kind k, or returns a diagnostic.
func (p *parser) expect(k tokKind, what string) *Diagnostic {
	if p.tok.kind != k {
		return p.errf(p.tok.pos, "expected "+what)
	}
	p.bump()
	return nil
}

// ---- body grammar -------------------------------------------------------

// parseDocument parses a whole document body. The top-level body parser only
// returns successfully once it reaches EOF, so any stray token (such as an
// unmatched `}`) is already reported by parseBody as "expected attribute or
// block name".
func (p *parser) parseDocument() (*Body, *Diagnostic) {
	return p.parseBody(true)
}

// parseBody parses a body. When top is false it ends at a `}` (a nested block
// body); when top is true it ends at EOF.
func (p *parser) parseBody(top bool) (*Body, *Diagnostic) {
	b := &Body{}
	for {
		p.skipNewlines()
		if p.tok.kind == tEOF {
			return b, nil
		}
		if !top && p.tok.kind == tRBrace {
			return b, nil
		}
		if p.tok.kind != tIdent {
			return nil, p.errf(p.tok.pos, "expected attribute or block name")
		}
		name := p.tok.text
		namePos := p.tok.pos
		p.bump()
		if p.tok.kind == tAssign {
			// attribute
			p.bump()
			e, d := p.parseExpr()
			if d != nil {
				return nil, d
			}
			b.Attributes = append(b.Attributes, &Attribute{Name: name, expr: e, Pos: namePos})
			if d := p.endOfLine(); d != nil {
				return nil, d
			}
			continue
		}
		// block: name labels... { body }
		blk := &Block{Type: name, Pos: namePos}
		for p.tok.kind == tString || p.tok.kind == tIdent {
			if p.tok.kind == tString {
				s, d := interpretStringKey(p.tok.text, p.tok.pos)
				if d != nil {
					return nil, d
				}
				blk.Labels = append(blk.Labels, s)
			} else {
				blk.Labels = append(blk.Labels, p.tok.text)
			}
			p.bump()
		}
		if d := p.expect(tLBrace, "'{' to open block body"); d != nil {
			return nil, d
		}
		inner, d := p.parseBody(false)
		if d != nil {
			return nil, d
		}
		if d := p.expect(tRBrace, "'}' to close block body"); d != nil {
			return nil, d
		}
		blk.Body = inner
		b.Blocks = append(b.Blocks, blk)
		if d := p.endOfLine(); d != nil {
			return nil, d
		}
	}
}

// endOfLine requires a newline or EOF (or a closing brace for a nested body)
// after a body item, enforcing newline-terminated attributes/blocks.
func (p *parser) endOfLine() *Diagnostic {
	switch p.tok.kind {
	case tNewline, tEOF, tRBrace:
		return nil
	default:
		return p.errf(p.tok.pos, "expected newline after body item")
	}
}

// ---- expression grammar (Pratt) ----------------------------------------

// parseExpr parses a full expression (the loosest production: conditional).
func (p *parser) parseExpr() (expr, *Diagnostic) {
	cond, d := p.parseBinary(0)
	if d != nil {
		return nil, d
	}
	if p.tok.kind == tQuest {
		qpos := p.tok.pos
		p.bump()
		then, d := p.parseExpr()
		if d != nil {
			return nil, d
		}
		if d := p.expect(tColon, "':' in conditional"); d != nil {
			return nil, d
		}
		els, d := p.parseExpr()
		if d != nil {
			return nil, d
		}
		return &condExpr{cond: cond, then: then, els: els, posMarker: posMarker{pos: qpos}}, nil
	}
	return cond, nil
}

// binBP returns the binding power of a binary operator, or 0 if not one.
func binBP(k tokKind) int {
	switch k {
	case tOr:
		return 1
	case tAnd:
		return 2
	case tEq, tNe:
		return 3
	case tLt, tLe, tGt, tGe:
		return 4
	case tPlus, tMinus:
		return 5
	case tStar, tSlash, tPercent:
		return 6
	}
	return 0
}

// parseBinary is precedence-climbing; all binary operators are left-associative.
func (p *parser) parseBinary(minBP int) (expr, *Diagnostic) {
	left, d := p.parseUnary()
	if d != nil {
		return nil, d
	}
	for {
		bp := binBP(p.tok.kind)
		if bp == 0 || bp < minBP {
			return left, nil
		}
		op := p.tok.kind
		opPos := p.tok.pos
		p.bump()
		right, d := p.parseBinary(bp + 1)
		if d != nil {
			return nil, d
		}
		left = &binaryExpr{op: op, l: left, r: right, posMarker: posMarker{pos: opPos}}
	}
}

// parseUnary handles prefix `-` and `!`.
func (p *parser) parseUnary() (expr, *Diagnostic) {
	if p.tok.kind == tMinus || p.tok.kind == tBang {
		op := p.tok.kind
		opPos := p.tok.pos
		p.bump()
		operand, d := p.parseUnary()
		if d != nil {
			return nil, d
		}
		return &unaryExpr{op: op, operand: operand, posMarker: posMarker{pos: opPos}}, nil
	}
	return p.parsePostfix()
}

// parsePostfix parses a primary then any chain of `.attr`, `[idx]`, and splats.
func (p *parser) parsePostfix() (expr, *Diagnostic) {
	e, d := p.parsePrimary()
	if d != nil {
		return nil, d
	}
	for {
		switch p.tok.kind {
		case tDot:
			pos := p.tok.pos
			p.bump()
			if p.tok.kind == tStar {
				// attribute splat: e.*.rest
				p.bump()
				se, d := p.parseSplat(e, pos)
				if d != nil {
					return nil, d
				}
				e = se
				continue
			}
			if p.tok.kind != tIdent && p.tok.kind != tNumber {
				return nil, p.errf(p.tok.pos, "expected attribute name after '.'")
			}
			// `a.0` is legal traversal sugar for `a[0]`.
			if p.tok.kind == tNumber {
				ne := &litExpr{}
				v, ok := parseNumber(p.tok.text)
				if !ok {
					return nil, p.errf(p.tok.pos, "invalid number index")
				}
				ne.val = v
				ne.posMarker = posMarker{pos: p.tok.pos}
				e = &indexExpr{coll: e, idx: ne, posMarker: posMarker{pos: pos}}
				p.bump()
				continue
			}
			e = &attrExpr{obj: e, name: p.tok.text, posMarker: posMarker{pos: pos}}
			p.bump()
		case tLBrack:
			pos := p.tok.pos
			p.bump()
			if p.tok.kind == tStar {
				// full splat: e[*].rest
				p.bump()
				if d := p.expect(tRBrack, "']' after splat '*'"); d != nil {
					return nil, d
				}
				se, d := p.parseSplat(e, pos)
				if d != nil {
					return nil, d
				}
				e = se
				continue
			}
			idx, d := p.parseExpr()
			if d != nil {
				return nil, d
			}
			if d := p.expect(tRBrack, "']' after index"); d != nil {
				return nil, d
			}
			e = &indexExpr{coll: e, idx: idx, posMarker: posMarker{pos: pos}}
		default:
			return e, nil
		}
	}
}

// splatVar is the internal loop variable a splat desugars to; it cannot collide
// because identifiers may not contain '$'.
const splatVar = "$splat"

// parseSplat desugars `coll[*]<rel>` / `coll.*<rel>` into a tuple for-expression
// `[for $splat in coll : $splat<rel>]`, capturing the following relative
// traversal (a chain of `.attr` and `[idx]`).
func (p *parser) parseSplat(coll expr, pos Pos) (expr, *Diagnostic) {
	var body expr = &varExpr{name: splatVar, posMarker: posMarker{pos: pos}}
	for {
		switch p.tok.kind {
		case tDot:
			dpos := p.tok.pos
			p.bump()
			if p.tok.kind == tStar {
				return nil, p.errf(p.tok.pos, "chained splats are not supported")
			}
			if p.tok.kind != tIdent {
				return nil, p.errf(p.tok.pos, "expected attribute name after '.'")
			}
			body = &attrExpr{obj: body, name: p.tok.text, posMarker: posMarker{pos: dpos}}
			p.bump()
		case tLBrack:
			ipos := p.tok.pos
			p.bump()
			if p.tok.kind == tStar {
				return nil, p.errf(p.tok.pos, "chained splats are not supported")
			}
			idx, d := p.parseExpr()
			if d != nil {
				return nil, d
			}
			if d := p.expect(tRBrack, "']' after index"); d != nil {
				return nil, d
			}
			body = &indexExpr{coll: body, idx: idx, posMarker: posMarker{pos: ipos}}
		default:
			return &forTupleExpr{valVar: splatVar, coll: coll, body: body, posMarker: posMarker{pos: pos}}, nil
		}
	}
}

// parsePrimary parses the tightest expression forms.
func (p *parser) parsePrimary() (expr, *Diagnostic) {
	t := p.tok
	switch t.kind {
	case tNumber:
		v, ok := parseNumber(t.text)
		if !ok {
			return nil, p.errf(t.pos, "invalid number literal")
		}
		p.bump()
		return &litExpr{val: v, posMarker: posMarker{pos: t.pos}}, nil
	case tString:
		p.bump()
		return &tmplExpr{raw: t.text, posMarker: posMarker{pos: t.pos}}, nil
	case tHeredoc:
		p.bump()
		return &tmplExpr{raw: t.text, heredoc: true, posMarker: posMarker{pos: t.pos}}, nil
	case tIdent:
		switch t.text {
		case "true":
			p.bump()
			return &litExpr{val: true, posMarker: posMarker{pos: t.pos}}, nil
		case "false":
			p.bump()
			return &litExpr{val: false, posMarker: posMarker{pos: t.pos}}, nil
		case "null":
			p.bump()
			return &litExpr{val: nil, posMarker: posMarker{pos: t.pos}}, nil
		}
		p.bump()
		if p.tok.kind == tLParen {
			return p.parseCall(t.text, t.pos)
		}
		return &varExpr{name: t.text, posMarker: posMarker{pos: t.pos}}, nil
	case tLParen:
		p.bump()
		p.skipNewlines()
		e, d := p.parseExpr()
		if d != nil {
			return nil, d
		}
		p.skipNewlines()
		if d := p.expect(tRParen, "')'"); d != nil {
			return nil, d
		}
		return e, nil
	case tLBrack:
		return p.parseTupleOrFor()
	case tLBrace:
		return p.parseObjectOrFor()
	}
	return nil, p.errf(t.pos, "expected expression")
}

// parseCall parses `(args...)` after an identifier known to be a call.
func (p *parser) parseCall(name string, pos Pos) (expr, *Diagnostic) {
	p.bump() // consume '('
	c := &callExpr{name: name, posMarker: posMarker{pos: pos}}
	p.skipNewlines()
	if p.tok.kind == tRParen {
		p.bump()
		return c, nil
	}
	for {
		p.skipNewlines()
		arg, d := p.parseExpr()
		if d != nil {
			return nil, d
		}
		c.args = append(c.args, arg)
		p.skipNewlines()
		if p.tok.kind == tEllipsis {
			c.expand = true
			p.bump()
			p.skipNewlines()
			if p.tok.kind == tComma {
				p.bump()
				p.skipNewlines()
			}
			if d := p.expect(tRParen, "')' after '...'"); d != nil {
				return nil, d
			}
			return c, nil
		}
		if p.tok.kind == tComma {
			p.bump()
			p.skipNewlines()
			if p.tok.kind == tRParen {
				p.bump()
				return c, nil
			}
			continue
		}
		if p.tok.kind == tRParen {
			p.bump()
			return c, nil
		}
		return nil, p.errf(p.tok.pos, "expected ',' or ')' in call arguments")
	}
}

// parseTupleOrFor parses `[ ... ]` as a tuple or a tuple for-expression.
func (p *parser) parseTupleOrFor() (expr, *Diagnostic) {
	pos := p.tok.pos
	p.bump() // consume '['
	p.skipNewlines()
	if p.tok.kind == tIdent && p.tok.text == "for" {
		return p.parseForTuple(pos)
	}
	t := &tupleExpr{posMarker: posMarker{pos: pos}}
	if p.tok.kind == tRBrack {
		p.bump()
		return t, nil
	}
	for {
		p.skipNewlines()
		e, d := p.parseExpr()
		if d != nil {
			return nil, d
		}
		t.items = append(t.items, e)
		p.skipNewlines()
		switch p.tok.kind {
		case tComma:
			p.bump()
			p.skipNewlines()
			if p.tok.kind == tRBrack {
				p.bump()
				return t, nil
			}
		case tRBrack:
			p.bump()
			return t, nil
		default:
			return nil, p.errf(p.tok.pos, "expected ',' or ']' in tuple")
		}
	}
}

// parseForTuple parses `for v[, v2] in coll : body [if cond]]` (the leading
// `[`/`for` already consumed except `for`).
func (p *parser) parseForTuple(pos Pos) (expr, *Diagnostic) {
	v1, v2, coll, d := p.parseForIntro()
	if d != nil {
		return nil, d
	}
	body, d := p.parseExpr()
	if d != nil {
		return nil, d
	}
	f := &forTupleExpr{coll: coll, body: body, posMarker: posMarker{pos: pos}}
	if v2 == "" {
		f.valVar = v1
	} else {
		f.keyVar = v1
		f.valVar = v2
	}
	p.skipNewlines()
	if p.tok.kind == tIdent && p.tok.text == "if" {
		p.bump()
		f.cond, d = p.parseExpr()
		if d != nil {
			return nil, d
		}
		p.skipNewlines()
	}
	if d := p.expect(tRBrack, "']' to close for-expression"); d != nil {
		return nil, d
	}
	return f, nil
}

// parseForIntro parses `for v[, v2] in coll :` and returns the vars and coll.
func (p *parser) parseForIntro() (v1, v2 string, coll expr, d *Diagnostic) {
	p.bump() // consume 'for'
	if p.tok.kind != tIdent {
		d = p.errf(p.tok.pos, "expected loop variable")
		return
	}
	v1 = p.tok.text
	p.bump()
	if p.tok.kind == tComma {
		p.bump()
		if p.tok.kind != tIdent {
			d = p.errf(p.tok.pos, "expected second loop variable")
			return
		}
		v2 = p.tok.text
		p.bump()
	}
	if p.tok.kind != tIdent || p.tok.text != "in" {
		d = p.errf(p.tok.pos, "expected 'in' in for-expression")
		return
	}
	p.bump()
	coll, d = p.parseExpr()
	if d != nil {
		return
	}
	if d = p.expect(tColon, "':' in for-expression"); d != nil {
		return
	}
	p.skipNewlines()
	return
}

// parseObjectOrFor parses `{ ... }` as an object literal or object
// for-expression.
func (p *parser) parseObjectOrFor() (expr, *Diagnostic) {
	pos := p.tok.pos
	p.bump() // consume '{'
	p.skipNewlines()
	if p.tok.kind == tIdent && p.tok.text == "for" {
		return p.parseForObject(pos)
	}
	o := &objectExpr{posMarker: posMarker{pos: pos}}
	if p.tok.kind == tRBrace {
		p.bump()
		return o, nil
	}
	for {
		p.skipNewlines()
		if p.tok.kind == tRBrace {
			p.bump()
			return o, nil
		}
		key, d := p.parseObjectKey()
		if d != nil {
			return nil, d
		}
		if p.tok.kind != tAssign && p.tok.kind != tColon {
			return nil, p.errf(p.tok.pos, "expected '=' or ':' after object key")
		}
		p.bump()
		val, d := p.parseExpr()
		if d != nil {
			return nil, d
		}
		o.keys = append(o.keys, key)
		o.vals = append(o.vals, val)
		// Items are separated by ',' or a newline.
		switch p.tok.kind {
		case tComma:
			p.bump()
		case tNewline:
			p.skipNewlines()
		case tRBrace:
			p.bump()
			return o, nil
		default:
			return nil, p.errf(p.tok.pos, "expected ',' or newline between object items")
		}
	}
}

// parseObjectKey parses an object key: a bare identifier (kept literal) or any
// expression (parenthesised / quoted, evaluated as the key).
func (p *parser) parseObjectKey() (expr, *Diagnostic) {
	if p.tok.kind == tIdent {
		// A bare identifier key is taken literally as a string.
		name := p.tok.text
		pos := p.tok.pos
		p.bump()
		return &litExpr{val: name, posMarker: posMarker{pos: pos}}, nil
	}
	return p.parseExpr()
}

// parseForObject parses `for k[, v] in coll : keyE => valE [...] [if cond]}`.
func (p *parser) parseForObject(pos Pos) (expr, *Diagnostic) {
	v1, v2, coll, d := p.parseForIntro()
	if d != nil {
		return nil, d
	}
	keyE, d := p.parseExpr()
	if d != nil {
		return nil, d
	}
	if d := p.expect(tFatArrow, "'=>' in object for-expression"); d != nil {
		return nil, d
	}
	valE, d := p.parseExpr()
	if d != nil {
		return nil, d
	}
	f := &forObjectExpr{coll: coll, keyE: keyE, valE: valE, posMarker: posMarker{pos: pos}}
	if v2 == "" {
		f.valVar = v1
	} else {
		f.keyVar = v1
		f.valVar = v2
	}
	if p.tok.kind == tEllipsis {
		f.group = true
		p.bump()
	}
	p.skipNewlines()
	if p.tok.kind == tIdent && p.tok.text == "if" {
		p.bump()
		f.cond, d = p.parseExpr()
		if d != nil {
			return nil, d
		}
		p.skipNewlines()
	}
	if d := p.expect(tRBrace, "'}' to close for-expression"); d != nil {
		return nil, d
	}
	return f, nil
}
