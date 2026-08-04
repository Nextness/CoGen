// lexer_unit_test.go contains tokenizer tests for the SOMETHING config language.
//go:build unit

package something

import (
	"testing"
)

// TestTokenizeEmpty verifies tokenize empty.
func TestTokenizeEmpty(t *testing.T) {
	ts := tokenize(t, "")
	if len(ts) != 1 || ts[0].Kind != TkEOF {
		t.Fatalf("expected only EOF token, got %v", ts)
	}
}

// TestTokenizeWhitespace verifies tokenize whitespace.
func TestTokenizeWhitespace(t *testing.T) {
	ts := tokenize(t, "   \n\t  ")
	if len(ts) != 1 || ts[0].Kind != TkEOF {
		t.Fatalf("expected only EOF token, got %v", ts)
	}
}

// TestTokenizeStringDouble verifies tokenize string double.
func TestTokenizeStringDouble(t *testing.T) {
	ts := tokenize(t, `"hello world"`)
	if len(ts) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(ts))
	}
	assertKind(t, ts[0], TkSTRING_LITERAL)
	sl, ok := ts[0].Value.(*StringLiteral)
	if !ok {
		t.Fatalf("expected *StringLiteral, got %T", ts[0].Value)
	}
	if len(sl.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(sl.Parts))
	}
	if sl.Parts[0].(StringText) != "hello world" {
		t.Errorf("expected 'hello world', got %q", sl.Parts[0])
	}
}

// TestTokenizeStringSingle verifies tokenize string single.
func TestTokenizeStringSingle(t *testing.T) {
	ts := tokenize(t, `'hello world'`)
	if len(ts) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(ts))
	}
	assertKind(t, ts[0], TkSTRING_LITERAL)
}

// TestTokenizeStringEscape verifies tokenize string escape.
func TestTokenizeStringEscape(t *testing.T) {
	ts := tokenize(t, `"hello\nworld\t\"escaped\""`)
	sl := ts[0].Value.(*StringLiteral)
	part := sl.Parts[0].(StringText)
	if part != "hello\nworld\t\"escaped\"" {
		t.Errorf("expected 'hello\\nworld\\t\"escaped\"', got %q", part)
	}
}

// TestTokenizeStringInterpolation verifies tokenize string interpolation.
func TestTokenizeStringInterpolation(t *testing.T) {
	ts := tokenize(t, `"hello {name} world"`)
	sl := ts[0].Value.(*StringLiteral)
	if len(sl.Parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(sl.Parts))
	}
	if sl.Parts[0].(StringText) != "hello " {
		t.Errorf("expected 'hello ', got %q", sl.Parts[0])
	}
	ref, ok := sl.Parts[1].(*InterpolationRef)
	if !ok {
		t.Fatalf("expected *InterpolationRef, got %T", sl.Parts[1])
	}
	if ref.Name != "name" {
		t.Errorf("expected 'name', got %q", ref.Name)
	}
	if sl.Parts[2].(StringText) != " world" {
		t.Errorf("expected ' world', got %q", sl.Parts[2])
	}
}

// TestTokenizeStringInterpolationDotPath verifies tokenize string interpolation dot path.
func TestTokenizeStringInterpolationDotPath(t *testing.T) {
	ts := tokenize(t, `"{a.b.c}"`)
	sl := ts[0].Value.(*StringLiteral)
	ref := sl.Parts[0].(*InterpolationRef)
	if ref.Name != "a.b.c" {
		t.Errorf("expected 'a.b.c', got %q", ref.Name)
	}
}

// TestTokenizeUnterminatedString verifies tokenize unterminated string.
func TestTokenizeUnterminatedString(t *testing.T) {
	assertPanic(t, func() {
		tokenize(t, `"hello`)
	}, "Unterminated string literal")
}

// TestTokenizeBadInterpolation verifies tokenize bad interpolation.
func TestTokenizeBadInterpolation(t *testing.T) {
	assertPanic(t, func() {
		tokenize(t, `"hello {} world"`)
	}, "Expected variable name")
}

// TestTokenizeUnclosedInterpolation verifies tokenize unclosed interpolation.
func TestTokenizeUnclosedInterpolation(t *testing.T) {
	assertPanic(t, func() {
		tokenize(t, `"hello {var`)
	}, "Expected '}'")
}

// TestTokenizeInteger verifies tokenize integer.
func TestTokenizeInteger(t *testing.T) {
	ts := tokenize(t, "42")
	assertKind(t, ts[0], TkINTEGER_LITERAL)
	if ts[0].StrValue() != "42" {
		t.Errorf("expected '42', got %q", ts[0].StrValue())
	}
}

// TestTokenizeIntegerNegative verifies tokenize integer negative.
func TestTokenizeIntegerNegative(t *testing.T) {
	ts := tokenize(t, "-42")
	assertKind(t, ts[0], TkINTEGER_LITERAL)
	if ts[0].StrValue() != "-42" {
		t.Errorf("expected '-42', got %q", ts[0].StrValue())
	}
}

// TestTokenizeIntegerUnderscore verifies tokenize integer underscore.
func TestTokenizeIntegerUnderscore(t *testing.T) {
	ts := tokenize(t, "1_000_000")
	assertKind(t, ts[0], TkINTEGER_LITERAL)
	if ts[0].StrValue() != "1_000_000" {
		t.Errorf("expected '1_000_000', got %q", ts[0].StrValue())
	}
}

// TestTokenizeFloat verifies tokenize float.
func TestTokenizeFloat(t *testing.T) {
	ts := tokenize(t, "3.14")
	assertKind(t, ts[0], TkFLOAT_LITERAL)
	if ts[0].StrValue() != "3.14" {
		t.Errorf("expected '3.14', got %q", ts[0].StrValue())
	}
}

// TestTokenizeFloatExponent verifies tokenize float exponent.
func TestTokenizeFloatExponent(t *testing.T) {
	ts := tokenize(t, "1E10")
	assertKind(t, ts[0], TkFLOAT_LITERAL)
	if ts[0].StrValue() != "1E10" {
		t.Errorf("expected '1E10', got %q", ts[0].StrValue())
	}
}

// TestTokenizeFloatNegativeExponent verifies tokenize float negative exponent.
func TestTokenizeFloatNegativeExponent(t *testing.T) {
	ts := tokenize(t, "1E-10")
	assertKind(t, ts[0], TkFLOAT_LITERAL)
	if ts[0].StrValue() != "1E-10" {
		t.Errorf("expected '1E-10', got %q", ts[0].StrValue())
	}
}

// TestTokenizeFloatNegative verifies tokenize float negative.
func TestTokenizeFloatNegative(t *testing.T) {
	ts := tokenize(t, "-0.5")
	assertKind(t, ts[0], TkFLOAT_LITERAL)
	if ts[0].StrValue() != "-0.5" {
		t.Errorf("expected '-0.5', got %q", ts[0].StrValue())
	}
}

// TestTokenizeFloatUnderscore verifies tokenize float underscore.
func TestTokenizeFloatUnderscore(t *testing.T) {
	ts := tokenize(t, "0.11_00")
	assertKind(t, ts[0], TkFLOAT_LITERAL)
}

// TestTokenizeKeywords verifies tokenize keywords.
func TestTokenizeKeywords(t *testing.T) {
	tests := []struct {
		input string
		kind  TokenKind
	}{
		{"enum", TkENUM},
		{"setup", TkSETUP},
		{"scope", TkSCOPE},
		{"mapping", TkMAPPING},
		{"for", TkFOR},
		{"insert", TkINSERT},
		{"iteration", TkITERATION},
		{"as_lvalue", TkASLVALUE},
		{"include", TkINCLUDE},
		{"namespace", TkNAMESPACE},
		{"string", TkSTRING},
		{"integer", TkINTEGER},
		{"boolean", TkBOOLEAN},
		{"float", TkFLOAT},
		{"timestamp", TkTIMESTAMP},
		{"true", TkTRUE},
		{"false", TkFALSE},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ts := tokenize(t, tt.input)
			assertKind(t, ts[0], tt.kind)
		})
	}
}

// TestTokenizeIdentifier verifies tokenize identifier.
func TestTokenizeIdentifier(t *testing.T) {
	ts := tokenize(t, "myVariable")
	assertKind(t, ts[0], TkIDENTIFIER)
	if ts[0].StrValue() != "myVariable" {
		t.Errorf("expected 'myVariable', got %q", ts[0].StrValue())
	}
}

// TestTokenizeIdentifierWithUnderscore verifies tokenize identifier with underscore.
func TestTokenizeIdentifierWithUnderscore(t *testing.T) {
	ts := tokenize(t, "my_var_name")
	assertKind(t, ts[0], TkIDENTIFIER)
}

// TestTokenizePunctuation verifies tokenize punctuation.
func TestTokenizePunctuation(t *testing.T) {
	tests := []struct {
		input string
		kind  TokenKind
	}{
		{":", TkCOLON},
		{"=", TkEQUALS},
		{",", TkCOMMA},
		{";", TkSEMICOLON},
		{".", TkDOT},
		{"{", TkLBRACE},
		{"}", TkRBRACE},
		{"(", TkLPAREN},
		{")", TkRPAREN},
		{"[", TkLBRACKET},
		{"]", TkRBRACKET},
		{"|", TkPIPE},
		{"?", TkOPTIONAL},
		{"=>", TkARROW},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ts := tokenize(t, tt.input)
			assertKind(t, ts[0], tt.kind)
		})
	}
}

// TestTokenizeHash verifies tokenize hash.
func TestTokenizeHash(t *testing.T) {
	ts := tokenize(t, "#")
	assertKind(t, ts[0], TkHASH)
}

// TestTokenizeLineComment verifies tokenize line comment.
func TestTokenizeLineComment(t *testing.T) {
	ts := tokenize(t, "// this is a comment\n42")
	if len(ts) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(ts))
	}
	assertKind(t, ts[0], TkINTEGER_LITERAL)
}

// TestTokenizeBlockComment verifies tokenize block comment.
func TestTokenizeBlockComment(t *testing.T) {
	ts := tokenize(t, "/* comment */ 42")
	assertKind(t, ts[0], TkINTEGER_LITERAL)
}

// TestTokenizeNestedBlockComment verifies tokenize nested block comment.
func TestTokenizeNestedBlockComment(t *testing.T) {
	ts := tokenize(t, "/* outer /* inner */ */ 42")
	assertKind(t, ts[0], TkINTEGER_LITERAL)
}

// TestTokenizeCRLFWhitespace verifies tokenize crlf whitespace.
func TestTokenizeCRLFWhitespace(t *testing.T) {
	ts := tokenize(t, "x := 1;\r\ny := 2;\r\n")
	if len(ts) == 0 || ts[len(ts)-1].Kind != TkEOF {
		t.Fatalf("expected CRLF input to tokenize, got %v", ts)
	}
}

// TestTokenizeUnterminatedBlockComment verifies tokenize unterminated block comment.
func TestTokenizeUnterminatedBlockComment(t *testing.T) {
	assertPanic(t, func() {
		tokenize(t, "/* never closed")
	}, "Unterminated block comment")
}

// TestTokenizeMultiline verifies tokenize multiline.
func TestTokenizeMultiline(t *testing.T) {
	ts := tokenize(t, "#multiline EOF\nhello world\nEOF")
	assertKind(t, ts[0], TkMULTILINE_STRING)
	pair := ts[0].Value.([2]string)
	if pair[0] != "hello world" {
		t.Errorf("expected 'hello world', got %q", pair[0])
	}
}

// TestTokenizeMultilineWithParams verifies tokenize multiline with params.
func TestTokenizeMultilineWithParams(t *testing.T) {
	ts := tokenize(t, "#multiline (no_newline) EOF\nhello\nworld\nEOF")
	assertKind(t, ts[0], TkMULTILINE_STRING)
	pair := ts[0].Value.([2]string)
	if pair[1] != "no_newline" {
		t.Errorf("expected params 'no_newline', got %q", pair[1])
	}
}

// TestTokenizeMultilineWithMultipleParams verifies tokenize multiline with multiple params.
func TestTokenizeMultilineWithMultipleParams(t *testing.T) {
	ts := tokenize(t, "#multiline (no_newline|no_indent) EOF\nhello\nEOF")
	assertKind(t, ts[0], TkMULTILINE_STRING)
	pair := ts[0].Value.([2]string)
	if pair[1] != "no_newline|no_indent" {
		t.Errorf("expected params 'no_newline|no_indent', got %q", pair[1])
	}
}

// TestTokenizeMultilineStripSpaces verifies tokenize multiline strip spaces.
func TestTokenizeMultilineStripSpaces(t *testing.T) {
	ts := tokenize(t, "#multiline (strip_spaces) EOF\nhello   world\nEOF")
	assertKind(t, ts[0], TkMULTILINE_STRING)
}

// TestTokenizeUnexpectedChar verifies tokenize unexpected char.
func TestTokenizeUnexpectedChar(t *testing.T) {
	assertPanic(t, func() {
		tokenize(t, "@")
	}, "Unexpected character")
}

// TestParseFallbackChar verifies parse fallback char.
func TestParseFallbackChar(t *testing.T) {
	ts := tokenize(t, "-")
	assertKind(t, ts[0], TkOPTIONAL)
}

// TestEvalStringLiteralRef verifies eval string literal ref.
func TestEvalStringLiteralRef(t *testing.T) {
	// Test that a string literal token produces the correct kind
	lex := NewLexer(`"hello world"`, "")
	ts := lex.Tokenize()
	if ts[0].Kind != TkSTRING_LITERAL {
		t.Errorf("expected STRING_LITERAL, got %v", ts[0].Kind)
	}
}

// TestLexerFallbackChar verifies lexer fallback char.
func TestLexerFallbackChar(t *testing.T) {
	// Direct test of the fallbackChar function
	// Test with various characters
	tests := []struct {
		ch   byte
		kind TokenKind
	}{
		{':', TkCOLON},
		{'=', TkEQUALS},
		{',', TkCOMMA},
		{';', TkSEMICOLON},
		{'.', TkDOT},
		{'{', TkLBRACE},
		{'}', TkRBRACE},
		{'(', TkLPAREN},
		{')', TkRPAREN},
		{'[', TkLBRACKET},
		{']', TkRBRACKET},
		{'|', TkPIPE},
		{'?', TkOPTIONAL},
		{'@', TkOPTIONAL}, // default case
	}
	for _, tt := range tests {
		t.Run(string(tt.ch), func(t *testing.T) {
			lex := NewLexer(string(tt.ch), "")
			tok := lex.fallbackChar()
			if tok.Kind != tt.kind {
				t.Errorf("expected %v, got %v", tt.kind, tok.Kind)
			}
		})
	}
}
