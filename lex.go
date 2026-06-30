// Copyright (c) the go-ruby-hcl2/hcl2 authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hcl2

// tokKind enumerates the lexical token kinds. The lexer is a single-byte
// look-ahead scanner; it skips ASCII whitespace and `#` / `//` line comments and
// `/* ... */` block comments between tokens.
type tokKind int

const (
	tEOF tokKind = iota
	tError
	tNumber
	tString // raw inner bytes of a "..." template (escapes kept verbatim)
	tHeredoc
	tIdent

	tLParen
	tRParen
	tLBrack
	tRBrack
	tLBrace
	tRBrace
	tComma
	tDot
	tColon
	tQuest
	tAssign
	tFatArrow // =>
	tEllipsis // ...

	tEq // ==
	tNe // !=
	tLt
	tLe
	tGt
	tGe
	tPlus
	tMinus
	tStar
	tSlash
	tPercent
	tBang // !
	tAnd  // &&
	tOr   // ||
	tNewline
)

// token is one lexical token with its source position.
type token struct {
	kind tokKind
	// text holds the lexeme: the identifier/number text, the raw inner bytes of a
	// string template, or for a heredoc the assembled raw body.
	text string
	// stripIndent is true for a `<<-` heredoc.
	stripIndent bool
	pos         Pos
}

// lexer turns a source string into tokens on demand.
type lexer struct {
	src  string
	pos  int // byte offset of the next unread byte
	line int
	col  int
}

func newLexer(src string) *lexer {
	return &lexer{src: src, line: 1, col: 1}
}

func (l *lexer) here() Pos { return Pos{Line: l.line, Col: l.col, Offset: l.pos} }

// advance consumes one byte, tracking line/col.
func (l *lexer) advance() byte {
	c := l.src[l.pos]
	l.pos++
	if c == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return c
}

func (l *lexer) peek() byte {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *lexer) peek2() byte {
	if l.pos+1 >= len(l.src) {
		return 0
	}
	return l.src[l.pos+1]
}

func isSpaceNoNL(c byte) bool { return c == ' ' || c == '\t' || c == '\r' }
func isDigit(c byte) bool     { return c >= '0' && c <= '9' }
func isIDStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
func isIDChar(c byte) bool { return isIDStart(c) || isDigit(c) || c == '-' }

// skipTrivia consumes whitespace (except newlines, which are tokens) and
// comments. It returns a diagnostic only for an unterminated block comment.
func (l *lexer) skipTrivia() *Diagnostic {
	for l.pos < len(l.src) {
		c := l.peek()
		switch {
		case isSpaceNoNL(c):
			l.advance()
		case c == '#':
			l.lineComment()
		case c == '/' && l.peek2() == '/':
			l.lineComment()
		case c == '/' && l.peek2() == '*':
			start := l.here()
			l.advance()
			l.advance()
			closed := false
			for l.pos < len(l.src) {
				if l.peek() == '*' && l.peek2() == '/' {
					l.advance()
					l.advance()
					closed = true
					break
				}
				l.advance()
			}
			if !closed {
				return &Diagnostic{Summary: "unterminated block comment", Pos: start}
			}
		default:
			return nil
		}
	}
	return nil
}

func (l *lexer) lineComment() {
	for l.pos < len(l.src) && l.peek() != '\n' {
		l.advance()
	}
}

// next returns the next token. On a lexical error it returns a token with kind
// tEOF and a non-nil diagnostic.
func (l *lexer) next() (token, *Diagnostic) {
	if d := l.skipTrivia(); d != nil {
		return token{kind: tEOF, pos: l.here()}, d
	}
	pos := l.here()
	if l.pos >= len(l.src) {
		return token{kind: tEOF, pos: pos}, nil
	}
	c := l.peek()

	if c == '\n' {
		l.advance()
		return token{kind: tNewline, pos: pos}, nil
	}
	if isDigit(c) {
		return l.lexNumber(pos), nil
	}
	if c == '"' {
		return l.lexString(pos)
	}
	if c == '<' && l.peek2() == '<' {
		return l.lexHeredoc(pos)
	}
	if isIDStart(c) {
		return l.lexIdent(pos), nil
	}
	return l.lexPunct(pos)
}

func (l *lexer) lexNumber(pos Pos) token {
	start := l.pos
	for l.pos < len(l.src) {
		c := l.peek()
		if isDigit(c) || c == '.' {
			l.advance()
			continue
		}
		if c == 'e' || c == 'E' {
			l.advance()
			if l.peek() == '+' || l.peek() == '-' {
				l.advance()
			}
			continue
		}
		break
	}
	return token{kind: tNumber, text: l.src[start:l.pos], pos: pos}
}

func (l *lexer) lexIdent(pos Pos) token {
	start := l.pos
	for l.pos < len(l.src) && isIDChar(l.peek()) {
		l.advance()
	}
	return token{kind: tIdent, text: l.src[start:l.pos], pos: pos}
}

// lexString scans a "..." string, capturing the raw inner bytes. The scan is
// interpolation-aware: a `"` only closes the string when not inside a `${ }` /
// `%{ }` template, so nested strings inside interpolations work.
func (l *lexer) lexString(pos Pos) (token, *Diagnostic) {
	l.advance() // opening quote
	start := l.pos
	depth := 0 // template brace depth
	for l.pos < len(l.src) {
		c := l.peek()
		if c == '\\' && depth == 0 {
			l.advance()
			if l.pos < len(l.src) {
				l.advance()
			}
			continue
		}
		// `$${` and `%%{` are escaped literals, not interpolation openers; consume
		// the doubled marker so its trailing `{` is not counted.
		if (c == '$' && l.peek2() == '$') || (c == '%' && l.peek2() == '%') {
			if l.pos+2 < len(l.src) && l.src[l.pos+2] == '{' {
				l.advance()
				l.advance()
				continue
			}
		}
		if (c == '$' || c == '%') && l.peek2() == '{' {
			l.advance() // $ or %
			l.advance() // {
			depth++
			continue
		}
		if c == '{' && depth > 0 {
			depth++
			l.advance()
			continue
		}
		if c == '}' && depth > 0 {
			depth--
			l.advance()
			continue
		}
		if c == '"' && depth == 0 {
			raw := l.src[start:l.pos]
			l.advance() // closing quote
			return token{kind: tString, text: raw, pos: pos}, nil
		}
		l.advance()
	}
	return token{kind: tEOF, pos: pos}, &Diagnostic{Summary: "unterminated string", Pos: pos}
}

// lexHeredoc scans a `<<DELIM` / `<<-DELIM` heredoc and returns the raw body.
func (l *lexer) lexHeredoc(pos Pos) (token, *Diagnostic) {
	l.advance() // <
	l.advance() // <
	strip := false
	if l.peek() == '-' {
		strip = true
		l.advance()
	}
	if !isIDStart(l.peek()) {
		return token{kind: tEOF, pos: pos}, &Diagnostic{Summary: "heredoc delimiter must be an identifier", Pos: l.here()}
	}
	ds := l.pos
	for l.pos < len(l.src) && isIDChar(l.peek()) {
		l.advance()
	}
	delim := l.src[ds:l.pos]
	// Skip the remainder of the introducer line up to and including the newline.
	for l.pos < len(l.src) && l.peek() != '\n' {
		if !isSpaceNoNL(l.peek()) {
			return token{kind: tEOF, pos: pos}, &Diagnostic{Summary: "unexpected characters after heredoc introducer", Pos: l.here()}
		}
		l.advance()
	}
	if l.pos >= len(l.src) {
		return token{kind: tEOF, pos: pos}, &Diagnostic{Summary: "unterminated heredoc", Pos: pos}
	}
	l.advance() // newline after introducer
	bodyStart := l.pos
	for {
		lineStart := l.pos
		// Read one line.
		for l.pos < len(l.src) && l.peek() != '\n' {
			l.advance()
		}
		lineEnd := l.pos
		line := l.src[lineStart:lineEnd]
		if isDelimLine(line, delim, strip) {
			body := l.src[bodyStart:lineStart]
			// Leave the trailing newline of the delimiter line unconsumed so it
			// lexes as a tNewline and terminates the enclosing body item.
			if strip {
				body = stripHeredocIndent(body)
			}
			return token{kind: tHeredoc, text: body, stripIndent: strip, pos: pos}, nil
		}
		if l.pos >= len(l.src) {
			return token{kind: tEOF, pos: pos}, &Diagnostic{Summary: "unterminated heredoc", Pos: pos}
		}
		l.advance() // newline
	}
}

// isDelimLine reports whether a line is the closing delimiter. For `<<` the
// delimiter must be at column 0; for `<<-` leading whitespace is allowed.
func isDelimLine(line, delim string, strip bool) bool {
	if strip {
		i := 0
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		line = line[i:]
	}
	return line == delim
}

// stripHeredocIndent removes the smallest run of leading spaces/tabs common to
// all non-blank lines of the body.
func stripHeredocIndent(body string) string {
	lines := splitLines(body)
	min := -1
	for _, ln := range lines {
		if blankLine(ln) {
			continue
		}
		n := 0
		for n < len(ln) && (ln[n] == ' ' || ln[n] == '\t') {
			n++
		}
		if min < 0 || n < min {
			min = n
		}
	}
	if min <= 0 {
		return body
	}
	out := make([]byte, 0, len(body))
	for i, ln := range lines {
		if i > 0 {
			out = append(out, '\n')
		}
		if blankLine(ln) {
			out = append(out, ln...)
			continue
		}
		out = append(out, ln[min:]...)
	}
	return string(out)
}

// splitLines splits on '\n' keeping the final (possibly empty) trailing segment.
func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func blankLine(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' && s[i] != '\r' {
			return false
		}
	}
	return true
}

func (l *lexer) lexPunct(pos Pos) (token, *Diagnostic) {
	c := l.advance()
	mk := func(k tokKind) (token, *Diagnostic) { return token{kind: k, pos: pos}, nil }
	switch c {
	case '(':
		return mk(tLParen)
	case ')':
		return mk(tRParen)
	case '[':
		return mk(tLBrack)
	case ']':
		return mk(tRBrack)
	case '{':
		return mk(tLBrace)
	case '}':
		return mk(tRBrace)
	case ',':
		return mk(tComma)
	case ':':
		return mk(tColon)
	case '?':
		return mk(tQuest)
	case '+':
		return mk(tPlus)
	case '-':
		return mk(tMinus)
	case '*':
		return mk(tStar)
	case '/':
		return mk(tSlash)
	case '%':
		return mk(tPercent)
	case '.':
		if l.peek() == '.' && l.peek2() == '.' {
			l.advance()
			l.advance()
			return mk(tEllipsis)
		}
		return mk(tDot)
	case '=':
		if l.peek() == '=' {
			l.advance()
			return mk(tEq)
		}
		if l.peek() == '>' {
			l.advance()
			return mk(tFatArrow)
		}
		return mk(tAssign)
	case '!':
		if l.peek() == '=' {
			l.advance()
			return mk(tNe)
		}
		return mk(tBang)
	case '<':
		if l.peek() == '=' {
			l.advance()
			return mk(tLe)
		}
		return mk(tLt)
	case '>':
		if l.peek() == '=' {
			l.advance()
			return mk(tGe)
		}
		return mk(tGt)
	case '&':
		if l.peek() == '&' {
			l.advance()
			return mk(tAnd)
		}
		return token{kind: tEOF, pos: pos}, &Diagnostic{Summary: "unexpected '&'; did you mean '&&'?", Pos: pos}
	case '|':
		if l.peek() == '|' {
			l.advance()
			return mk(tOr)
		}
		return token{kind: tEOF, pos: pos}, &Diagnostic{Summary: "unexpected '|'; did you mean '||'?", Pos: pos}
	}
	return token{kind: tEOF, pos: pos}, &Diagnostic{Summary: "invalid character " + strconvQuoteByte(c), Pos: pos}
}

func strconvQuoteByte(c byte) string {
	return "'" + string([]byte{c}) + "'"
}
