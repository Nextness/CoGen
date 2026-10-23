// lexer_unit_test.go contains tokenizer tests for the SOMETHING config language.

package something_test

import subject "analysis/backend/something"
import testhelpers "analysis/tests/support/go/backend/something"

import (
	"testing"
)

// TestTokenizeEmpty verifies tokenize empty.
func TestTokenizeEmpty(t *testing.T) {
	ts := testhelpers.Tokenize(t, "")
	if len(ts) != 1 || ts[0].Kind != subject.TkEOF {
		t.Fatalf("expected only EOF token, got %v", ts)
	}
}

// TestTokenizeWhitespace verifies tokenize whitespace.
func TestTokenizeWhitespace(t *testing.T) {
	ts := testhelpers.Tokenize(t, "   \n\t  ")
	if len(ts) != 1 || ts[0].Kind != subject.TkEOF {
		t.Fatalf("expected only EOF token, got %v", ts)
	}
}

// TestTokenizeStringDouble verifies tokenize string double.
func TestTokenizeStringDouble(t *testing.T) {
	ts := testhelpers.Tokenize(t, `"hello world"`)
	if len(ts) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(ts))
	}
	testhelpers.AssertKind(t, ts[0], subject.TkSTRING_LITERAL)
	sl, ok := ts[0].Value.(*subject.StringLiteral)
	if !ok {
		t.Fatalf("expected *StringLiteral, got %T", ts[0].Value)
	}
	if len(sl.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(sl.Parts))
	}
	if sl.Parts[0].(subject.StringText) != "hello world" {
		t.Errorf("expected 'hello world', got %q", sl.Parts[0])
	}
}

// TestTokenizeStringSingle verifies tokenize string single.
func TestTokenizeStringSingle(t *testing.T) {
	ts := testhelpers.Tokenize(t, `'hello world'`)
	if len(ts) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(ts))
	}
	testhelpers.AssertKind(t, ts[0], subject.TkSTRING_LITERAL)
}

// TestTokenizeStringEscape verifies tokenize string escape.
func TestTokenizeStringEscape(t *testing.T) {
	ts := testhelpers.Tokenize(t, `"hello\nworld\t\"escaped\""`)
	sl := ts[0].Value.(*subject.StringLiteral)
	part := sl.Parts[0].(subject.StringText)
	if part != "hello\nworld\t\"escaped\"" {
		t.Errorf("expected 'hello\\nworld\\t\"escaped\"', got %q", part)
	}
}

// TestTokenizeStringInterpolation verifies tokenize string interpolation.
func TestTokenizeStringInterpolation(t *testing.T) {
	ts := testhelpers.Tokenize(t, `"hello {name} world"`)
	sl := ts[0].Value.(*subject.StringLiteral)
	if len(sl.Parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(sl.Parts))
	}
	if sl.Parts[0].(subject.StringText) != "hello " {
		t.Errorf("expected 'hello ', got %q", sl.Parts[0])
	}
	ref, ok := sl.Parts[1].(*subject.InterpolationRef)
	if !ok {
		t.Fatalf("expected *InterpolationRef, got %T", sl.Parts[1])
	}
	if ref.Name != "name" {
		t.Errorf("expected 'name', got %q", ref.Name)
	}
	if sl.Parts[2].(subject.StringText) != " world" {
		t.Errorf("expected ' world', got %q", sl.Parts[2])
	}
}

// TestTokenizeStringInterpolationDotPath verifies tokenize string interpolation dot path.
func TestTokenizeStringInterpolationDotPath(t *testing.T) {
	ts := testhelpers.Tokenize(t, `"{a.b.c}"`)
	sl := ts[0].Value.(*subject.StringLiteral)
	ref := sl.Parts[0].(*subject.InterpolationRef)
	if ref.Name != "a.b.c" {
		t.Errorf("expected 'a.b.c', got %q", ref.Name)
	}
}

// TestTokenizeUnterminatedString verifies tokenize unterminated string.
func TestTokenizeUnterminatedString(t *testing.T) {
	testhelpers.AssertPanic(t, func() {
		testhelpers.Tokenize(t, `"hello`)
	}, "Unterminated string literal")
}

// TestTokenizeBadInterpolation verifies tokenize bad interpolation.
func TestTokenizeBadInterpolation(t *testing.T) {
	testhelpers.AssertPanic(t, func() {
		testhelpers.Tokenize(t, `"hello {} world"`)
	}, "Expected variable name")
}

// TestTokenizeUnclosedInterpolation verifies tokenize unclosed interpolation.
func TestTokenizeUnclosedInterpolation(t *testing.T) {
	testhelpers.AssertPanic(t, func() {
		testhelpers.Tokenize(t, `"hello {var`)
	}, "Expected '}'")
}

// TestTokenizeInteger verifies tokenize integer.
func TestTokenizeInteger(t *testing.T) {
	ts := testhelpers.Tokenize(t, "42")
	testhelpers.AssertKind(t, ts[0], subject.TkINTEGER_LITERAL)
	if ts[0].StrValue() != "42" {
		t.Errorf("expected '42', got %q", ts[0].StrValue())
	}
}

// TestTokenizeIntegerNegative verifies tokenize integer negative.
func TestTokenizeIntegerNegative(t *testing.T) {
	ts := testhelpers.Tokenize(t, "-42")
	testhelpers.AssertKind(t, ts[0], subject.TkINTEGER_LITERAL)
	if ts[0].StrValue() != "-42" {
		t.Errorf("expected '-42', got %q", ts[0].StrValue())
	}
}

// TestTokenizeIntegerUnderscore verifies tokenize integer underscore.
func TestTokenizeIntegerUnderscore(t *testing.T) {
	ts := testhelpers.Tokenize(t, "1_000_000")
	testhelpers.AssertKind(t, ts[0], subject.TkINTEGER_LITERAL)
	if ts[0].StrValue() != "1_000_000" {
		t.Errorf("expected '1_000_000', got %q", ts[0].StrValue())
	}
}

// TestTokenizeFloat verifies tokenize float.
func TestTokenizeFloat(t *testing.T) {
	ts := testhelpers.Tokenize(t, "3.14")
	testhelpers.AssertKind(t, ts[0], subject.TkFLOAT_LITERAL)
	if ts[0].StrValue() != "3.14" {
		t.Errorf("expected '3.14', got %q", ts[0].StrValue())
	}
}

// TestTokenizeFloatExponent verifies tokenize float exponent.
func TestTokenizeFloatExponent(t *testing.T) {
	ts := testhelpers.Tokenize(t, "1E10")
	testhelpers.AssertKind(t, ts[0], subject.TkFLOAT_LITERAL)
	if ts[0].StrValue() != "1E10" {
		t.Errorf("expected '1E10', got %q", ts[0].StrValue())
	}
}

// TestTokenizeFloatNegativeExponent verifies tokenize float negative exponent.
func TestTokenizeFloatNegativeExponent(t *testing.T) {
	ts := testhelpers.Tokenize(t, "1E-10")
	testhelpers.AssertKind(t, ts[0], subject.TkFLOAT_LITERAL)
	if ts[0].StrValue() != "1E-10" {
		t.Errorf("expected '1E-10', got %q", ts[0].StrValue())
	}
}

// TestTokenizeFloatNegative verifies tokenize float negative.
func TestTokenizeFloatNegative(t *testing.T) {
	ts := testhelpers.Tokenize(t, "-0.5")
	testhelpers.AssertKind(t, ts[0], subject.TkFLOAT_LITERAL)
	if ts[0].StrValue() != "-0.5" {
		t.Errorf("expected '-0.5', got %q", ts[0].StrValue())
	}
}

// TestTokenizeFloatUnderscore verifies tokenize float underscore.
func TestTokenizeFloatUnderscore(t *testing.T) {
	ts := testhelpers.Tokenize(t, "0.11_00")
	testhelpers.AssertKind(t, ts[0], subject.TkFLOAT_LITERAL)
}

// TestTokenizeKeywords verifies tokenize keywords.
func TestTokenizeKeywords(t *testing.T) {
	tests := []struct {
		input string
		kind  subject.TokenKind
	}{
		{"enum", subject.TkENUM},
		{"setup", subject.TkSETUP},
		{"scope", subject.TkSCOPE},
		{"mapping", subject.TkMAPPING},
		{"for", subject.TkFOR},
		{"insert", subject.TkINSERT},
		{"iteration", subject.TkITERATION},
		{"as_lvalue", subject.TkASLVALUE},
		{"include", subject.TkINCLUDE},
		{"namespace", subject.TkNAMESPACE},
		{"string", subject.TkSTRING},
		{"integer", subject.TkINTEGER},
		{"boolean", subject.TkBOOLEAN},
		{"float", subject.TkFLOAT},
		{"timestamp", subject.TkTIMESTAMP},
		{"true", subject.TkTRUE},
		{"false", subject.TkFALSE},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ts := testhelpers.Tokenize(t, tt.input)
			testhelpers.AssertKind(t, ts[0], tt.kind)
		})
	}
}

// TestTokenizeIdentifier verifies tokenize identifier.
func TestTokenizeIdentifier(t *testing.T) {
	ts := testhelpers.Tokenize(t, "myVariable")
	testhelpers.AssertKind(t, ts[0], subject.TkIDENTIFIER)
	if ts[0].StrValue() != "myVariable" {
		t.Errorf("expected 'myVariable', got %q", ts[0].StrValue())
	}
}

// TestTokenizeIdentifierWithUnderscore verifies tokenize identifier with underscore.
func TestTokenizeIdentifierWithUnderscore(t *testing.T) {
	ts := testhelpers.Tokenize(t, "my_var_name")
	testhelpers.AssertKind(t, ts[0], subject.TkIDENTIFIER)
}

// TestTokenizePunctuation verifies tokenize punctuation.
func TestTokenizePunctuation(t *testing.T) {
	tests := []struct {
		input string
		kind  subject.TokenKind
	}{
		{":", subject.TkCOLON},
		{"=", subject.TkEQUALS},
		{",", subject.TkCOMMA},
		{";", subject.TkSEMICOLON},
		{".", subject.TkDOT},
		{"{", subject.TkLBRACE},
		{"}", subject.TkRBRACE},
		{"(", subject.TkLPAREN},
		{")", subject.TkRPAREN},
		{"[", subject.TkLBRACKET},
		{"]", subject.TkRBRACKET},
		{"|", subject.TkPIPE},
		{"?", subject.TkOPTIONAL},
		{"=>", subject.TkARROW},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ts := testhelpers.Tokenize(t, tt.input)
			testhelpers.AssertKind(t, ts[0], tt.kind)
		})
	}
}

// TestTokenizeHash verifies tokenize hash.
func TestTokenizeHash(t *testing.T) {
	ts := testhelpers.Tokenize(t, "#")
	testhelpers.AssertKind(t, ts[0], subject.TkHASH)
}

// TestTokenizeLineComment verifies tokenize line comment.
func TestTokenizeLineComment(t *testing.T) {
	ts := testhelpers.Tokenize(t, "// this is a comment\n42")
	if len(ts) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(ts))
	}
	testhelpers.AssertKind(t, ts[0], subject.TkINTEGER_LITERAL)
}

// TestTokenizeBlockComment verifies tokenize block comment.
func TestTokenizeBlockComment(t *testing.T) {
	ts := testhelpers.Tokenize(t, "/* comment */ 42")
	testhelpers.AssertKind(t, ts[0], subject.TkINTEGER_LITERAL)
}

// TestTokenizeNestedBlockComment verifies tokenize nested block comment.
func TestTokenizeNestedBlockComment(t *testing.T) {
	ts := testhelpers.Tokenize(t, "/* outer /* inner */ */ 42")
	testhelpers.AssertKind(t, ts[0], subject.TkINTEGER_LITERAL)
}

// TestTokenizeMultilineNestedBlockComment verifies a multiline /** **/ block
// comment with nested comments is skipped and content after it still tokenizes.
func TestTokenizeMultilineNestedBlockComment(t *testing.T) {
	ts := testhelpers.Tokenize(t, `/** This is a 
multiline comment /** with nested comments also **/ that work anywhere in the configuration **/ 42`)
	if len(ts) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(ts))
	}
	testhelpers.AssertKind(t, ts[0], subject.TkINTEGER_LITERAL)
	testhelpers.AssertKind(t, ts[1], subject.TkEOF)
}

// TestTokenizeBlockCommentBetweenTokens verifies a block comment is skipped
// anywhere in the configuration, not only at the start of the source.
func TestTokenizeBlockCommentBetweenTokens(t *testing.T) {
	kinds := testhelpers.TokenKinds(t, "a /* comment */ := /* another */ 1;")
	expected := []subject.TokenKind{subject.TkIDENTIFIER, subject.TkCOLON, subject.TkEQUALS, subject.TkINTEGER_LITERAL, subject.TkSEMICOLON, subject.TkEOF}
	if len(kinds) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(kinds), kinds)
	}
	for i, kind := range expected {
		if kinds[i] != kind {
			t.Errorf("expected token %d to be %v, got %v", i, kind, kinds[i])
		}
	}
}

// TestTokenizeCRLFWhitespace verifies tokenize crlf whitespace.
func TestTokenizeCRLFWhitespace(t *testing.T) {
	ts := testhelpers.Tokenize(t, "x := 1;\r\ny := 2;\r\n")
	if len(ts) == 0 || ts[len(ts)-1].Kind != subject.TkEOF {
		t.Fatalf("expected CRLF input to tokenize, got %v", ts)
	}
}

// TestTokenizeUnterminatedBlockComment verifies tokenize unterminated block comment.
func TestTokenizeUnterminatedBlockComment(t *testing.T) {
	testhelpers.AssertPanic(t, func() {
		testhelpers.Tokenize(t, "/* never closed")
	}, "Unterminated block comment")
}

// TestTokenizeMultiline verifies tokenize multiline.
func TestTokenizeMultiline(t *testing.T) {
	ts := testhelpers.Tokenize(t, "#multiline EOF\nhello world\nEOF")
	testhelpers.AssertKind(t, ts[0], subject.TkMULTILINE_STRING)
	pair := ts[0].Value.([2]string)
	if pair[0] != "hello world" {
		t.Errorf("expected 'hello world', got %q", pair[0])
	}
}

// TestTokenizeMultilineWithParams verifies tokenize multiline with params.
func TestTokenizeMultilineWithParams(t *testing.T) {
	ts := testhelpers.Tokenize(t, "#multiline (no_newline) EOF\nhello\nworld\nEOF")
	testhelpers.AssertKind(t, ts[0], subject.TkMULTILINE_STRING)
	pair := ts[0].Value.([2]string)
	if pair[1] != "no_newline" {
		t.Errorf("expected params 'no_newline', got %q", pair[1])
	}
}

// TestTokenizeMultilineWithMultipleParams verifies tokenize multiline with multiple params.
func TestTokenizeMultilineWithMultipleParams(t *testing.T) {
	ts := testhelpers.Tokenize(t, "#multiline (no_newline|no_indent) EOF\nhello\nEOF")
	testhelpers.AssertKind(t, ts[0], subject.TkMULTILINE_STRING)
	pair := ts[0].Value.([2]string)
	if pair[1] != "no_newline|no_indent" {
		t.Errorf("expected params 'no_newline|no_indent', got %q", pair[1])
	}
}

// TestTokenizeMultilineStripSpaces verifies tokenize multiline strip spaces.
func TestTokenizeMultilineStripSpaces(t *testing.T) {
	ts := testhelpers.Tokenize(t, "#multiline (strip_spaces) EOF\nhello   world\nEOF")
	testhelpers.AssertKind(t, ts[0], subject.TkMULTILINE_STRING)
}

// TestTokenizeMultilineComment verifies a // comment is removed from multiline content.
func TestTokenizeMultilineComment(t *testing.T) {
	ts := testhelpers.Tokenize(t, "#multiline EOF\nhello // comment\nworld\nEOF")
	pair := ts[0].Value.([2]string)
	if pair[0] != "hello\nworld" {
		t.Errorf("expected comment stripped, got %q", pair[0])
	}
}

// TestTokenizeMultilineEscapedSlash verifies \/\/ produces a literal //.
func TestTokenizeMultilineEscapedSlash(t *testing.T) {
	ts := testhelpers.Tokenize(t, "#multiline EOF\nhello \\/\\/ world\nEOF")
	pair := ts[0].Value.([2]string)
	if pair[0] != "hello // world" {
		t.Errorf("expected escaped slash, got %q", pair[0])
	}
}

// TestTokenizeMultilineClosingTagComment verifies a closing tag with a trailing comment closes.
func TestTokenizeMultilineClosingTagComment(t *testing.T) {
	ts := testhelpers.Tokenize(t, "#multiline EOF\nhello\nEOF; // done")
	if pair := ts[0].Value.([2]string); pair[0] != "hello" {
		t.Errorf("expected 'hello', got %q", pair[0])
	}
	testhelpers.AssertKind(t, ts[1], subject.TkSEMICOLON)
}

// TestTokenizeMultilineCommentOnlyLine verifies a comment-only line becomes empty.
func TestTokenizeMultilineCommentOnlyLine(t *testing.T) {
	ts := testhelpers.Tokenize(t, "#multiline EOF\n// only a comment\nhello\nEOF")
	pair := ts[0].Value.([2]string)
	if pair[0] != "\nhello" {
		t.Errorf("expected empty first line, got %q", pair[0])
	}
}

// TestTokenizeUnexpectedChar verifies tokenize unexpected char.
func TestTokenizeUnexpectedChar(t *testing.T) {
	testhelpers.AssertPanic(t, func() {
		testhelpers.Tokenize(t, "$")
	}, "Unexpected character")
}

// TestTokenizeAt verifies the '@' intrinsic prefix tokenizes as TkAT.
func TestTokenizeAt(t *testing.T) {
	ts := testhelpers.Tokenize(t, "@")
	testhelpers.AssertKind(t, ts[0], subject.TkAT)
}

// TestParseFallbackChar verifies parse fallback char.
func TestParseFallbackChar(t *testing.T) {
	ts := testhelpers.Tokenize(t, "-")
	testhelpers.AssertKind(t, ts[0], subject.TkOPTIONAL)
}

// TestEvalStringLiteralRef verifies eval string literal ref.
func TestEvalStringLiteralRef(t *testing.T) {
	// Test that a string literal token produces the correct kind
	lex := subject.NewLexer(`"hello world"`, "")
	ts := lex.Tokenize()
	if ts[0].Kind != subject.TkSTRING_LITERAL {
		t.Errorf("expected STRING_LITERAL, got %v", ts[0].Kind)
	}
}

// TestLexerFallbackChar verifies lexer fallback char.
func TestLexerFallbackChar(t *testing.T) {
	// Direct test of the fallbackChar function
	// Test with various characters
	tests := []struct {
		ch   byte
		kind subject.TokenKind
	}{
		{':', subject.TkCOLON},
		{'=', subject.TkEQUALS},
		{',', subject.TkCOMMA},
		{';', subject.TkSEMICOLON},
		{'.', subject.TkDOT},
		{'{', subject.TkLBRACE},
		{'}', subject.TkRBRACE},
		{'(', subject.TkLPAREN},
		{')', subject.TkRPAREN},
		{'[', subject.TkLBRACKET},
		{']', subject.TkRBRACKET},
		{'|', subject.TkPIPE},
		{'?', subject.TkOPTIONAL},
	}
	for _, tt := range tests {
		t.Run(string(tt.ch), func(t *testing.T) {
			lex := subject.NewLexer(string(tt.ch), "")
			tok := lex.Tokenize()[0]
			if tok.Kind != tt.kind {
				t.Errorf("expected %v, got %v", tt.kind, tok.Kind)
			}
		})
	}
}
