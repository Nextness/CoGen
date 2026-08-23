// lexer.go tokenizes raw .something source text into a sequence of Token
// values. It recognises keywords, identifiers, string literals (with
// interpolation), number literals, multiline strings, punctuation, and
// comments. The lexer tracks line/column positions for error reporting.
package something

import (
	"strconv"
	"strings"
	"unicode"
)

// TokenKind identifies a lexical category in the SOMETHING language.
type TokenKind int

const (
	TkENUM TokenKind = iota
	TkSETUP
	TkSCOPE
	TkMAPPING
	TkFOR
	TkINSERT
	TkITERATION
	TkASLVALUE
	TkMACRO
	TkSET
	TkPRIV
	TkSTRING
	TkINTEGER
	TkBOOLEAN
	TkFLOAT
	TkTIMESTAMP
	TkINCLUDE
	TkNAMESPACE
	TkTRUE
	TkFALSE
	TkCOLON
	TkEQUALS
	TkARROW
	TkRARROW
	TkBANG
	TkCOMMA
	TkSEMICOLON
	TkDOT
	TkLBRACE
	TkRBRACE
	TkLPAREN
	TkRPAREN
	TkLBRACKET
	TkRBRACKET
	TkPIPE
	TkOPTIONAL
	TkSTRING_LITERAL
	TkINTEGER_LITERAL
	TkFLOAT_LITERAL
	TkBOOLEAN_LITERAL
	TkMULTILINE_STRING
	TkIDENTIFIER
	TkHASH
	TkEOF
	TkASSERT // "assert"
	TkIF     // "if"
	TkAND    // "and"
	TkOR     // "or"
	TkMATCH  // "match"
	TkLEN    // "len"
	TkNOT    // "not"
	TkERROR  // "error"
	TkEQ     // "=="
	TkNEQ    // "!="
	TkLE     // "<="
	TkGE     // ">="
	TkLT     // "<"
	TkGT     // ">"
)

var tokenKindNames = map[TokenKind]string{
	TkENUM:             "ENUM",
	TkSETUP:            "SETUP",
	TkSCOPE:            "SCOPE",
	TkMAPPING:          "MAPPING",
	TkFOR:              "FOR",
	TkINSERT:           "INSERT",
	TkITERATION:        "ITERATION",
	TkASLVALUE:         "ASLVALUE",
	TkMACRO:            "MACRO",
	TkSET:              "SET",
	TkPRIV:             "PRIV",
	TkSTRING:           "STRING",
	TkINTEGER:          "INTEGER",
	TkBOOLEAN:          "BOOLEAN",
	TkFLOAT:            "FLOAT",
	TkTIMESTAMP:        "TIMESTAMP",
	TkINCLUDE:          "INCLUDE",
	TkNAMESPACE:        "NAMESPACE",
	TkTRUE:             "TRUE",
	TkFALSE:            "FALSE",
	TkCOLON:            "COLON",
	TkEQUALS:           "EQUALS",
	TkARROW:            "ARROW",
	TkRARROW:           "RARROW",
	TkBANG:             "BANG",
	TkCOMMA:            "COMMA",
	TkSEMICOLON:        "SEMICOLON",
	TkDOT:              "DOT",
	TkLBRACE:           "LBRACE",
	TkRBRACE:           "RBRACE",
	TkLPAREN:           "LPAREN",
	TkRPAREN:           "RPAREN",
	TkLBRACKET:         "LBRACKET",
	TkRBRACKET:         "RBRACKET",
	TkPIPE:             "PIPE",
	TkOPTIONAL:         "OPTIONAL",
	TkSTRING_LITERAL:   "STRING_LITERAL",
	TkINTEGER_LITERAL:  "INTEGER_LITERAL",
	TkFLOAT_LITERAL:    "FLOAT_LITERAL",
	TkBOOLEAN_LITERAL:  "BOOLEAN_LITERAL",
	TkMULTILINE_STRING: "MULTILINE_STRING",
	TkIDENTIFIER:       "IDENTIFIER",
	TkHASH:             "HASH",
	TkEOF:              "EOF",
	TkASSERT:           "ASSERT",
	TkIF:               "IF",
	TkAND:              "AND",
	TkOR:               "OR",
	TkMATCH:            "MATCH",
	TkLEN:              "LEN",
	TkNOT:              "NOT",
	TkERROR:            "ERROR",
	TkEQ:               "EQ",
	TkNEQ:              "NEQ",
	TkLE:               "LE",
	TkGE:               "GE",
	TkLT:               "LT",
	TkGT:               "GT",
}

// String returns the receiver's textual representation.
func (k TokenKind) String() string {
	if s, ok := tokenKindNames[k]; ok {
		return s
	}
	return "UNKNOWN"
}

// Token represents a single lexeme.
type Token struct {
	Kind  TokenKind
	Value any
	Line  int
	Col   int
}

// StringPart is one literal or interpolated part of a string token.
type StringPart interface {
	stringPartMarker()
}

// StringText is literal text within a string.
type StringText string

// stringPartMarker marks StringText as a StringPart implementation.
func (StringText) stringPartMarker() {}

// stringPartMarker marks InterpolationRef as a StringPart implementation.
func (*InterpolationRef) stringPartMarker() {}

// InterpolationRef is a dotted reference inside a string literal.
type InterpolationRef struct {
	Name string
}

// StringLiteral is the lexer's structured representation of a string.
type StringLiteral struct {
	Parts []StringPart
}

// StrValue returns the token's string payload, or an empty string for another payload type.
func (t Token) StrValue() string {
	if s, ok := t.Value.(string); ok {
		return s
	}
	return ""
}

var builtinBooleans = map[string]TokenKind{
	"true":  TkTRUE,
	"false": TkFALSE,
}

var builtinTypes = map[string]TokenKind{
	"enum":      TkENUM,
	"setup":     TkSETUP,
	"scope":     TkSCOPE,
	"namespace": TkNAMESPACE,
	"string":    TkSTRING,
	"integer":   TkINTEGER,
	"boolean":   TkBOOLEAN,
	"float":     TkFLOAT,
	"timestamp": TkTIMESTAMP,
}

var keywords = map[string]TokenKind{
	"mapping":   TkMAPPING,
	"for":       TkFOR,
	"insert":    TkINSERT,
	"iteration": TkITERATION,
	"as_lvalue": TkASLVALUE,
	"include":   TkINCLUDE,
	"macro":     TkMACRO,
	"set":       TkSET,
	"priv":      TkPRIV,
	"assert":    TkASSERT,
	"if":        TkIF,
	"and":       TkAND,
	"or":        TkOR,
	"match":     TkMATCH,
	"len":       TkLEN,
	"not":       TkNOT,
	"error":     TkERROR,
}

// TypeTokens is the set of token kinds that represent primitive types.
var typeTokens = map[TokenKind]bool{
	TkSTRING:    true,
	TkINTEGER:   true,
	TkBOOLEAN:   true,
	TkFLOAT:     true,
	TkTIMESTAMP: true,
	TkSCOPE:     true,
	TkNAMESPACE: true,
}

var arrayIndexTypeTokens = map[TokenKind]bool{
	TkINTEGER: true,
	TkSTRING:  true,
}

// Lexer tracks source text, coordinates, and file identity while producing tokens.
type Lexer struct {
	text     string
	pos      int
	line     int
	col      int
	length   int
	filepath string
}

// NewLexer constructs lexer.
func NewLexer(text string, filepath string) *Lexer {
	return &Lexer{
		text:     text,
		pos:      0,
		line:     1,
		col:      1,
		length:   len(text),
		filepath: filepath,
	}
}

// err panics with a SomethingError at the lexer's current source position.
func (l *Lexer) err(msg string) {
	panic(&SomethingError{Message: msg, Line: l.line, Col: l.col, Filepath: l.filepath})
}

// advance consumes one byte and updates line and column coordinates.
func (l *Lexer) advance() {
	if l.pos >= l.length {
		return
	}
	ch := l.text[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
}

// peek returns a byte relative to the current position, or zero outside the source.
func (l *Lexer) peek(offset int) byte {
	idx := l.pos + offset
	if idx >= 0 && idx < l.length {
		return l.text[idx]
	}
	return 0
}

// skipWhitespaceAndComments consumes whitespace and nested line or block comments.
func (l *Lexer) skipWhitespaceAndComments() {
	for l.pos < l.length {
		ch := l.peek(0)
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			l.advance()
			continue
		}
		if ch == '/' && l.peek(1) == '/' {
			for l.pos < l.length && l.peek(0) != '\n' {
				l.advance()
			}
			continue
		}
		if ch == '/' && l.peek(1) == '*' {
			l.advance()
			l.advance()
			depth := 1
			for l.pos < l.length && depth > 0 {
				if l.peek(0) == '*' && l.peek(1) == '/' {
					l.advance()
					l.advance()
					depth--
				} else if l.peek(0) == '/' && l.peek(1) == '*' {
					l.advance()
					l.advance()
					depth++
				} else {
					l.advance()
				}
			}
			if depth != 0 {
				l.err("Unterminated block comment")
			}
			continue
		}
		break
	}
}

// readWord reads word from the supplied source.
func (l *Lexer) readWord() string {
	start := l.pos
	ch := rune(l.peek(0))
	if ch == 0 || !(unicode.IsLetter(ch) || ch == '_') {
		return ""
	}
	l.advance()
	for l.pos < l.length {
		ch := rune(l.peek(0))
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			l.advance()
		} else {
			break
		}
	}
	return l.text[start:l.pos]
}

// readDigitsWithUnderscore reads digits with underscore from the supplied source.
func (l *Lexer) readDigitsWithUnderscore() string {
	start := l.pos
	for l.pos < l.length {
		ch := l.peek(0)
		if ch >= '0' && ch <= '9' {
			l.advance()
		} else if ch == '_' && l.peek(1) != 0 && l.peek(1) >= '0' && l.peek(1) <= '9' {
			l.advance()
		} else {
			break
		}
	}
	return l.text[start:l.pos]
}

// readExponent reads exponent from the supplied source.
func (l *Lexer) readExponent() string {
	var b strings.Builder
	b.WriteByte(l.peek(0))
	l.advance()
	if l.peek(0) == '-' {
		b.WriteByte('-')
		l.advance()
	}
	expDigits := l.readDigitsWithUnderscore()
	if expDigits == "" {
		l.err("Expected exponent digits after 'E'")
	}
	b.WriteString(expDigits)
	return b.String()
}

// readNumber reads number from the supplied source.
func (l *Lexer) readNumber() Token {
	startLine, startCol := l.line, l.col
	negative := false
	if l.peek(0) == '-' {
		negative = true
		l.advance()
	}

	intPart := l.readDigitsWithUnderscore()
	if intPart == "" {
		if negative {
			l.pos--
			l.col--
			return l.fallbackChar()
		}
		l.err("Expected digits after '-'")
	}

	ch := l.peek(0)
	if ch == '.' && l.peek(1) != 0 && (l.peek(1) >= '0' && l.peek(1) <= '9' || l.peek(1) == '_') {
		l.advance()
		frac := l.readDigitsWithUnderscore()
		raw := intPart + "." + frac
		if l.peek(0) == 'E' || l.peek(0) == 'e' {
			raw += l.readExponent()
		}
		if negative {
			raw = "-" + raw
		}
		return Token{TkFLOAT_LITERAL, raw, startLine, startCol}
	}

	if ch == 'E' || ch == 'e' {
		raw := intPart + l.readExponent()
		if negative {
			raw = "-" + raw
		}
		return Token{TkFLOAT_LITERAL, raw, startLine, startCol}
	}

	raw := intPart
	if negative {
		raw = "-" + raw
	}
	return Token{TkINTEGER_LITERAL, raw, startLine, startCol}
}

// fallbackChar tokenizes punctuation that is not handled by a longer lexical form.
func (l *Lexer) fallbackChar() Token {
	startLine, startCol := l.line, l.col
	ch := l.peek(0)
	l.advance()
	switch ch {
	case ':':
		return Token{TkCOLON, ":", startLine, startCol}
	case '=':
		return Token{TkEQUALS, "=", startLine, startCol}
	case ',':
		return Token{TkCOMMA, ",", startLine, startCol}
	case ';':
		return Token{TkSEMICOLON, ";", startLine, startCol}
	case '.':
		return Token{TkDOT, ".", startLine, startCol}
	case '{':
		return Token{TkLBRACE, "{", startLine, startCol}
	case '}':
		return Token{TkRBRACE, "}", startLine, startCol}
	case '(':
		return Token{TkLPAREN, "(", startLine, startCol}
	case ')':
		return Token{TkRPAREN, ")", startLine, startCol}
	case '[':
		return Token{TkLBRACKET, "[", startLine, startCol}
	case ']':
		return Token{TkRBRACKET, "]", startLine, startCol}
	case '|':
		return Token{TkPIPE, "|", startLine, startCol}
	case '?':
		return Token{TkOPTIONAL, "?", startLine, startCol}
	case '-':
		return Token{TkOPTIONAL, "-", startLine, startCol}
	default:
		return Token{TkOPTIONAL, string(ch), startLine, startCol}
	}
}

// readString reads string from the supplied source.
func (l *Lexer) readString(quote byte, tokLine, tokCol int) Token {
	parts := []StringPart{}
	buf := strings.Builder{}

	for l.pos < l.length {
		ch := l.peek(0)
		if ch == quote {
			l.advance()
			if buf.Len() > 0 {
				parts = append(parts, StringText(buf.String()))
				buf.Reset()
			}
			return Token{TkSTRING_LITERAL, &StringLiteral{Parts: parts}, tokLine, tokCol}
		}
		if ch == '\\' {
			l.advance()
			esc := l.peek(0)
			switch esc {
			case 'n':
				buf.WriteByte('\n')
			case 't':
				buf.WriteByte('\t')
			case 'r':
				buf.WriteByte('\r')
			case '"':
				buf.WriteByte('"')
			case '\'':
				buf.WriteByte('\'')
			case '\\':
				buf.WriteByte('\\')
			default:
				buf.WriteByte(esc)
			}
			l.advance()
			continue
		}
		if ch == '{' {
			if l.peek(1) == '{' {
				// Escaped {{ produces literal {
				buf.WriteByte('{')
				l.advance()
				l.advance()
				continue
			}
			if buf.Len() > 0 {
				parts = append(parts, StringText(buf.String()))
				buf.Reset()
			}
			l.advance()
			varStart := l.pos
			for l.pos < l.length {
				c := l.peek(0)
				if unicode.IsLetter(rune(c)) || unicode.IsDigit(rune(c)) || c == '_' || c == '.' {
					l.advance()
				} else {
					break
				}
			}
			varName := l.text[varStart:l.pos]
			if varName == "" {
				l.err("Expected variable name after '{' in string interpolation")
			}
			parts = append(parts, &InterpolationRef{Name: varName})
			if l.peek(0) != '}' {
				l.err("Expected '}' closing interpolation reference")
			}
			l.advance()
			continue
		}
		if ch == '}' && l.peek(1) == '}' {
			// Escaped }} produces literal }
			buf.WriteByte('}')
			l.advance()
			l.advance()
			continue
		}
		buf.WriteByte(ch)
		l.advance()
	}
	l.err("Unterminated string literal")
	return Token{}
}

// readMultiline reads multiline from the supplied source.
func (l *Lexer) readMultiline() Token {
	tokLine, tokCol := l.line, l.col
	params := ""
	l.skipWhitespaceAndComments()

	if l.peek(0) == '(' {
		l.advance()
		depth := 1
		var b strings.Builder
		for l.pos < l.length && depth > 0 {
			ch := l.peek(0)
			if ch == '(' {
				depth++
			} else if ch == ')' {
				depth--
				if depth == 0 {
					break
				}
			}
			b.WriteByte(ch)
			l.advance()
		}
		if depth != 0 {
			l.err("Unterminated '(' in multiline parameters")
		}
		l.advance()
		params = strings.TrimSpace(b.String())
	}

	l.skipWhitespaceAndComments()
	tag := l.readWord()
	if tag == "" {
		l.err("Expected tag after #multiline")
	}

	for l.pos < l.length && (l.peek(0) == ' ' || l.peek(0) == '\t') {
		l.advance()
	}
	if l.peek(0) == '\n' {
		l.advance()
	}

	contentLines := []string{}
	found := false
	for l.pos < l.length {
		lineStart := l.pos
		eol := strings.IndexByte(l.text[l.pos:], '\n')
		if eol == -1 {
			eol = l.length - l.pos
		}
		line := strings.TrimRight(StripMultilineComment(l.text[l.pos:l.pos+eol]), " \t")
		withoutIndent := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(withoutIndent, tag) {
			suffix := strings.TrimSpace(withoutIndent[len(tag):])
			if suffix != "" && suffix != "," && suffix != ";" {
				contentLines = append(contentLines, line)
				l.pos = l.pos + eol
				if l.peek(0) == '\n' {
					l.advance()
				}
				continue
			}
			closingTagEnd := lineStart + len(line) - len(withoutIndent) + len(tag)
			for l.pos < closingTagEnd {
				l.advance()
			}
			found = true
			break
		}
		contentLines = append(contentLines, line)
		l.pos = l.pos + eol
		if l.peek(0) == '\n' {
			l.advance()
		}
	}
	if !found {
		l.err("Unterminated multiline string (tag '" + tag + "' never closed)")
	}

	raw := strings.Join(contentLines, "\n")
	return Token{TkMULTILINE_STRING, [2]string{raw, params}, tokLine, tokCol}
}

// StripMultilineComment removes a trailing // comment from one multiline
// string line and resolves the \/ escape to a literal slash. It is exported so
// tools that scan raw SOMETHING source (for example prepare-osf) apply the
// same comment and escape semantics as the lexer.
func StripMultilineComment(line string) string {
	var b strings.Builder
	for i := 0; i < len(line); i++ {
		if line[i] == '\\' && i+1 < len(line) && line[i+1] == '/' {
			b.WriteByte('/')
			i++
			continue
		}
		if line[i] == '/' && i+1 < len(line) && line[i+1] == '/' {
			break
		}
		b.WriteByte(line[i])
	}
	return b.String()
}

// Tokenize returns the token list for the lexer's text.
func (l *Lexer) Tokenize() []Token {
	tokens := []Token{}
	for l.pos < l.length {
		l.skipWhitespaceAndComments()
		if l.pos >= l.length {
			break
		}
		ch := l.peek(0)
		sl, sc := l.line, l.col

		if ch == '"' || ch == '\'' {
			l.advance()
			tokens = append(tokens, l.readString(ch, sl, sc))
			continue
		}
		if ch == '#' {
			if l.length-l.pos >= 10 && l.text[l.pos:l.pos+10] == "#multiline" {
				l.pos += 10
				l.col += 10
				tokens = append(tokens, l.readMultiline())
				continue
			}
			l.advance()
			tokens = append(tokens, Token{TkHASH, "#", sl, sc})
			continue
		}
		if ch == '=' && l.peek(1) == '>' {
			l.advance()
			l.advance()
			tokens = append(tokens, Token{TkARROW, "=>", sl, sc})
			continue
		}
		if ch == '-' && l.peek(1) == '>' {
			l.advance()
			l.advance()
			tokens = append(tokens, Token{TkRARROW, "->", sl, sc})
			continue
		}
		if ch == '=' && l.peek(1) == '=' {
			l.advance()
			l.advance()
			tokens = append(tokens, Token{TkEQ, "==", sl, sc})
			continue
		}
		if ch == '!' && l.peek(1) == '=' {
			l.advance()
			l.advance()
			tokens = append(tokens, Token{TkNEQ, "!=", sl, sc})
			continue
		}
		if ch == '<' && l.peek(1) == '=' {
			l.advance()
			l.advance()
			tokens = append(tokens, Token{TkLE, "<=", sl, sc})
			continue
		}
		if ch == '>' && l.peek(1) == '=' {
			l.advance()
			l.advance()
			tokens = append(tokens, Token{TkGE, ">=", sl, sc})
			continue
		}

		switch ch {
		case ':':
			l.advance()
			tokens = append(tokens, Token{TkCOLON, ":", sl, sc})
			continue
		case '=':
			l.advance()
			tokens = append(tokens, Token{TkEQUALS, "=", sl, sc})
			continue
		case ',':
			l.advance()
			tokens = append(tokens, Token{TkCOMMA, ",", sl, sc})
			continue
		case ';':
			l.advance()
			tokens = append(tokens, Token{TkSEMICOLON, ";", sl, sc})
			continue
		case '.':
			l.advance()
			tokens = append(tokens, Token{TkDOT, ".", sl, sc})
			continue
		case '{':
			l.advance()
			tokens = append(tokens, Token{TkLBRACE, "{", sl, sc})
			continue
		case '}':
			l.advance()
			tokens = append(tokens, Token{TkRBRACE, "}", sl, sc})
			continue
		case '(':
			l.advance()
			tokens = append(tokens, Token{TkLPAREN, "(", sl, sc})
			continue
		case ')':
			l.advance()
			tokens = append(tokens, Token{TkRPAREN, ")", sl, sc})
			continue
		case '[':
			l.advance()
			tokens = append(tokens, Token{TkLBRACKET, "[", sl, sc})
			continue
		case ']':
			l.advance()
			tokens = append(tokens, Token{TkRBRACKET, "]", sl, sc})
			continue
		case '|':
			l.advance()
			tokens = append(tokens, Token{TkPIPE, "|", sl, sc})
			continue
		case '?':
			l.advance()
			tokens = append(tokens, Token{TkOPTIONAL, "?", sl, sc})
			continue
		case '!':
			l.advance()
			tokens = append(tokens, Token{TkBANG, "!", sl, sc})
			continue
		case '<':
			l.advance()
			tokens = append(tokens, Token{TkLT, "<", sl, sc})
			continue
		case '>':
			l.advance()
			tokens = append(tokens, Token{TkGT, ">", sl, sc})
			continue
		}

		if ch >= '0' && ch <= '9' || (ch == '-' && l.peek(1) != 0 && l.peek(1) >= '0' && l.peek(1) <= '9') {
			tokens = append(tokens, l.readNumber())
			continue
		}
		if ch == '-' {
			l.advance()
			tokens = append(tokens, Token{TkOPTIONAL, "-", sl, sc})
			continue
		}
		if unicode.IsLetter(rune(ch)) || ch == '_' {
			word := l.readWord()
			kind, ok := keywords[word]
			if !ok {
				kind, ok = builtinTypes[word]
			}
			if !ok {
				kind, ok = builtinBooleans[word]
			}
			if !ok {
				kind = TkIDENTIFIER
			}
			tokens = append(tokens, Token{kind, word, sl, sc})
			continue
		}
		l.err("Unexpected character '" + string(ch) + "'")
	}
	tokens = append(tokens, Token{TkEOF, nil, l.line, l.col})
	return tokens
}

// ParseIntLiteral removes digit separators and parses a validated integer literal.
func ParseIntLiteral(s string) int {
	s = strings.ReplaceAll(s, "_", "")
	n, _ := strconv.Atoi(s)
	return n
}

// ParseFloatLiteral removes digit separators and parses a validated floating-point literal.
func ParseFloatLiteral(s string) float64 {
	s = strings.ReplaceAll(s, "_", "")
	val, _ := strconv.ParseFloat(s, 64)
	return val
}

// IsInt reports whether a possibly underscore-separated string parses as an integer.
func IsInt(s string) bool {
	s = strings.ReplaceAll(s, "_", "")
	_, err := strconv.Atoi(s)
	return err == nil
}
