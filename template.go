// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

import (
	"strconv"
	"strings"
)

// evalTemplate interprets a string/heredoc template against ctx and returns the
// rendered string. For a quoted string (heredoc=false) backslash escapes are
// expanded; for a heredoc they are kept literal (matching HCL). `${ }`
// interpolations are re-parsed and evaluated; `%{ }` directives provide control
// flow.
func (ev *evaluator) evalTemplate(raw string, heredoc bool, pos Pos) (string, *Diagnostic) {
	toks, d := tokenizeTemplate(raw, heredoc, pos)
	if d != nil {
		return "", d
	}
	parts, _, d := ev.renderTemplate(toks, 0, pos, nil)
	if d != nil {
		return "", d
	}
	return parts, nil
}

// tplTok is a lexical chunk of a template.
type tplTok struct {
	kind tplKind
	text string // literal text, or the raw expr/directive body
	pos  Pos
}

type tplKind int

const (
	tplLit tplKind = iota
	tplInterp
	tplDirective
)

// tokenizeTemplate splits a raw template into literal / `${ }` / `%{ }` chunks,
// expanding `\`-escapes for quoted strings.
func tokenizeTemplate(raw string, heredoc bool, pos Pos) ([]tplTok, *Diagnostic) {
	var toks []tplTok
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			toks = append(toks, tplTok{kind: tplLit, text: lit.String(), pos: pos})
			lit.Reset()
		}
	}
	i := 0
	for i < len(raw) {
		c := raw[i]
		if c == '\\' && !heredoc {
			s, n, d := decodeEscape(raw[i:], pos)
			if d != nil {
				return nil, d
			}
			lit.WriteString(s)
			i += n
			continue
		}
		// `$${` / `%%{` literal escapes.
		if (c == '$' || c == '%') && i+2 < len(raw) && raw[i+1] == c && raw[i+2] == '{' {
			lit.WriteByte(c)
			lit.WriteByte('{')
			i += 3
			continue
		}
		if (c == '$' || c == '%') && i+1 < len(raw) && raw[i+1] == '{' {
			flush()
			body, n, d := scanBraced(raw[i+2:], pos)
			if d != nil {
				return nil, d
			}
			kind := tplInterp
			if c == '%' {
				kind = tplDirective
			}
			toks = append(toks, tplTok{kind: kind, text: body, pos: pos})
			i += 2 + n
			continue
		}
		lit.WriteByte(c)
		i++
	}
	flush()
	return toks, nil
}

// scanBraced reads up to the matching `}` (brace-depth aware) and returns the
// inner body and the number of bytes consumed including the closing `}`.
func scanBraced(s string, pos Pos) (string, int, *Diagnostic) {
	depth := 1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[:i], i + 1, nil
			}
		}
	}
	return "", 0, &Diagnostic{Summary: "unterminated template interpolation", Pos: pos}
}

// decodeEscape decodes one backslash escape at the start of s, returning the
// decoded text and the number of source bytes consumed.
func decodeEscape(s string, pos Pos) (string, int, *Diagnostic) {
	if len(s) < 2 {
		return "", 0, &Diagnostic{Summary: "dangling backslash in string", Pos: pos}
	}
	switch s[1] {
	case 'n':
		return "\n", 2, nil
	case 't':
		return "\t", 2, nil
	case 'r':
		return "\r", 2, nil
	case '"':
		return "\"", 2, nil
	case '\\':
		return "\\", 2, nil
	case 'u':
		return decodeHexEscape(s, 4, pos)
	case 'U':
		return decodeHexEscape(s, 8, pos)
	}
	return "", 0, &Diagnostic{Summary: "invalid escape sequence \\" + string(s[1]), Pos: pos}
}

func decodeHexEscape(s string, n int, pos Pos) (string, int, *Diagnostic) {
	if len(s) < 2+n {
		return "", 0, &Diagnostic{Summary: "truncated unicode escape", Pos: pos}
	}
	v, err := strconv.ParseUint(s[2:2+n], 16, 32)
	if err != nil {
		return "", 0, &Diagnostic{Summary: "invalid unicode escape", Pos: pos}
	}
	r := rune(v)
	if r > 0x10FFFF || (r >= 0xD800 && r <= 0xDFFF) {
		return "", 0, &Diagnostic{Summary: "invalid unicode code point", Pos: pos}
	}
	return string(r), 2 + n, nil
}

// renderTemplate renders a token slice starting at index start. When stop is
// non-nil it returns at the first directive keyword in stop, leaving the index
// at that directive; otherwise it consumes to the end. The returned int is the
// next unconsumed index.
func (ev *evaluator) renderTemplate(toks []tplTok, start int, pos Pos, stop []string) (string, int, *Diagnostic) {
	var b strings.Builder
	i := start
	for i < len(toks) {
		t := toks[i]
		switch t.kind {
		case tplLit:
			b.WriteString(t.text)
			i++
		case tplInterp:
			s, d := ev.evalInterp(t.text, pos)
			if d != nil {
				return "", 0, d
			}
			b.WriteString(s)
			i++
		case tplDirective:
			kw := directiveKeyword(t.text)
			if stop != nil && containsStr(stop, kw) {
				return b.String(), i, nil
			}
			s, ni, d := ev.evalDirective(toks, i, pos)
			if d != nil {
				return "", 0, d
			}
			b.WriteString(s)
			i = ni
		}
	}
	if stop != nil {
		return "", 0, &Diagnostic{Summary: "unterminated template directive", Pos: pos}
	}
	return b.String(), i, nil
}

// evalInterp parses and evaluates a `${ expr }` body, rendering the result.
func (ev *evaluator) evalInterp(body string, pos Pos) (string, *Diagnostic) {
	v, d := ev.evalSubExpr(body, pos)
	if d != nil {
		return "", d
	}
	return renderInterpValue(v, pos)
}

// evalSubExpr parses and evaluates a standalone expression string.
func (ev *evaluator) evalSubExpr(src string, pos Pos) (Value, *Diagnostic) {
	p := newParser(src)
	e, d := p.parseExpr()
	if d != nil {
		return nil, d
	}
	if p.tok.kind != tEOF {
		return nil, &Diagnostic{Summary: "unexpected trailing tokens in interpolation", Pos: pos}
	}
	return ev.eval(e)
}

func renderInterpValue(v Value, pos Pos) (string, *Diagnostic) {
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
		return "", &Diagnostic{Summary: "cannot interpolate null into a string", Pos: pos}
	}
	return "", &Diagnostic{Summary: "cannot interpolate a collection into a string", Pos: pos}
}

// directiveKeyword returns the leading keyword of a `%{ ... }` directive body.
func directiveKeyword(body string) string {
	f := strings.Fields(body)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

// evalDirective handles `%{ if }`, `%{ for }`, and reports a clear error for an
// unknown directive. It returns the rendered text and the next index.
func (ev *evaluator) evalDirective(toks []tplTok, i int, pos Pos) (string, int, *Diagnostic) {
	kw := directiveKeyword(toks[i].text)
	switch kw {
	case "if":
		return ev.evalIfDirective(toks, i, pos)
	case "for":
		return ev.evalForDirective(toks, i, pos)
	}
	return "", 0, &Diagnostic{Summary: "unknown template directive %{" + kw + "}", Pos: pos}
}

// evalIfDirective renders `%{ if c }...%{ else }...%{ endif }`.
func (ev *evaluator) evalIfDirective(toks []tplTok, i int, pos Pos) (string, int, *Diagnostic) {
	condSrc := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(toks[i].text), "if"))
	cv, d := ev.evalSubExpr(condSrc, pos)
	if d != nil {
		return "", 0, d
	}
	cond, ok := cv.(bool)
	if !ok {
		return "", 0, &Diagnostic{Summary: "if directive condition must be a bool", Pos: pos}
	}
	thenText, j, d := ev.renderTemplate(toks, i+1, pos, []string{"else", "endif"})
	if d != nil {
		return "", 0, d
	}
	var elseText string
	if directiveKeyword(toks[j].text) == "else" {
		elseText, j, d = ev.renderTemplate(toks, j+1, pos, []string{"endif"})
		if d != nil {
			return "", 0, d
		}
	}
	// toks[j] is endif.
	if cond {
		return thenText, j + 1, nil
	}
	return elseText, j + 1, nil
}

// evalForDirective renders `%{ for v in coll }...%{ endfor }` (and the k,v form).
func (ev *evaluator) evalForDirective(toks []tplTok, i int, pos Pos) (string, int, *Diagnostic) {
	keyVar, valVar, coll, d := ev.parseForDirectiveIntro(toks[i].text, pos)
	if d != nil {
		return "", 0, d
	}
	// Locate the matching endfor by structural scan (no evaluation), so the body
	// can be re-rendered once per element with the loop variables bound.
	end, d := scanDirectiveEnd(toks, i+1, pos, "for", "endfor")
	if d != nil {
		return "", 0, d
	}
	body := toks[i+1 : end]
	var b strings.Builder
	d = ev.iterateCollection(coll, keyVar, valVar, pos, func() *Diagnostic {
		s, _, d := ev.renderTemplate(body, 0, pos, nil)
		if d != nil {
			return d
		}
		b.WriteString(s)
		return nil
	})
	if d != nil {
		return "", 0, d
	}
	return b.String(), end + 1, nil
}

// scanDirectiveEnd returns the index of the directive closing `closeKW`,
// accounting for nested `openKW` directives, without evaluating anything.
func scanDirectiveEnd(toks []tplTok, start int, pos Pos, openKW, closeKW string) (int, *Diagnostic) {
	depth := 0
	for i := start; i < len(toks); i++ {
		if toks[i].kind != tplDirective {
			continue
		}
		switch directiveKeyword(toks[i].text) {
		case openKW:
			depth++
		case closeKW:
			if depth == 0 {
				return i, nil
			}
			depth--
		}
	}
	return 0, &Diagnostic{Summary: "unterminated %{" + openKW + "} directive", Pos: pos}
}

// parseForDirectiveIntro parses the `for v[, v2] in <expr>` of a for directive.
func (ev *evaluator) parseForDirectiveIntro(body string, pos Pos) (keyVar, valVar string, coll Value, d *Diagnostic) {
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(body), "for"))
	inIdx := strings.Index(rest, " in ")
	if inIdx < 0 {
		return "", "", nil, &Diagnostic{Summary: "for directive requires 'in'", Pos: pos}
	}
	vars := strings.TrimSpace(rest[:inIdx])
	collSrc := strings.TrimSpace(rest[inIdx+4:])
	if comma := strings.Index(vars, ","); comma >= 0 {
		keyVar = strings.TrimSpace(vars[:comma])
		valVar = strings.TrimSpace(vars[comma+1:])
	} else {
		valVar = vars
	}
	if valVar == "" {
		return "", "", nil, &Diagnostic{Summary: "for directive requires a loop variable", Pos: pos}
	}
	coll, d = ev.evalSubExpr(collSrc, pos)
	return
}

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
