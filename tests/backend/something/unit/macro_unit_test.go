// macro_unit_test.go contains unit tests for macro lexing, parsing, and
// interface markers, tested directly without the full pipeline.

package something_test

import subject "analysis/backend/something"
import testhelpers "analysis/tests/support/go/backend/something"

import (
	"testing"
)

// TestTokenizeMacroAndSet verifies tokenize macro and set.
func TestTokenizeMacroAndSet(t *testing.T) {
	ts := testhelpers.Tokenize(t, "#macro")
	if len(ts) < 3 {
		t.Fatalf("expected at least 3 tokens, got %d", len(ts))
	}
	if ts[0].Kind != subject.TkHASH {
		t.Errorf("expected HASH, got %v", ts[0].Kind)
	}
	if ts[1].Kind != subject.TkMACRO {
		t.Errorf("expected MACRO, got %v(%q)", ts[1].Kind, ts[1].StrValue())
	}

	ts2 := testhelpers.Tokenize(t, "#set")
	if ts2[0].Kind != subject.TkHASH {
		t.Errorf("expected HASH, got %v", ts2[0].Kind)
	}
	if ts2[1].Kind != subject.TkSET {
		t.Errorf("expected SET, got %v(%q)", ts2[1].Kind, ts2[1].StrValue())
	}
}

// TestTokenizeRarrow verifies tokenize rarrow.
func TestTokenizeRarrow(t *testing.T) {
	kinds := testhelpers.TokenKinds(t, "->")
	if kinds[0] != subject.TkRARROW {
		t.Errorf("expected RARROW, got %v", kinds[0])
	}
}

// TestTokenizeBang verifies tokenize bang.
func TestTokenizeBang(t *testing.T) {
	kinds := testhelpers.TokenKinds(t, "!")
	if kinds[0] != subject.TkBANG {
		t.Errorf("expected BANG, got %v", kinds[0])
	}
}

// TestParseMacroDecl verifies parse macro decl.
func TestParseMacroDecl(t *testing.T) {
	text := `#macro greet := () -> string {
    #set "hello";
}`
	prog := testhelpers.ParseText(t, text)
	if len(prog.Macros) != 1 {
		t.Fatalf("expected 1 macro, got %d", len(prog.Macros))
	}
	md := prog.Macros[0]
	if md.Name != "greet" {
		t.Errorf("expected macro name 'greet', got %q", md.Name)
	}
	if len(md.Params) != 0 {
		t.Errorf("expected 0 params, got %d", len(md.Params))
	}
	if md.SetExpr == nil {
		t.Fatal("expected non-nil set expression")
	}
	if md.SetExpr.Kind != testhelpers.KindString {
		t.Errorf("expected KindString set expression, got %v", md.SetExpr.Kind)
	}
}

// TestParseMacroDeclWithParams verifies parse macro decl with params.
func TestParseMacroDeclWithParams(t *testing.T) {
	text := `#macro make_pair := (a: string, b: string) -> []string {
    #set []string { a, b };
}`
	prog := testhelpers.ParseText(t, text)
	if len(prog.Macros) != 1 {
		t.Fatalf("expected 1 macro, got %d", len(prog.Macros))
	}
	md := prog.Macros[0]
	if md.Name != "make_pair" {
		t.Errorf("expected macro name 'make_pair', got %q", md.Name)
	}
	if len(md.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(md.Params))
	}
	if md.Params[0].Name != "a" || md.Params[1].Name != "b" {
		t.Errorf("expected params [a, b], got %v", md.Params)
	}
}

// TestParseMacroCall verifies parse macro call.
func TestParseMacroCall(t *testing.T) {
	text := `x := greet!();`
	prog := testhelpers.ParseText(t, text)
	if len(prog.TopLevelVars) != 1 {
		t.Fatalf("expected 1 var, got %d", len(prog.TopLevelVars))
	}
	vn := prog.TopLevelVars[0].Value
	if vn.Kind != testhelpers.KindMacroCall {
		t.Errorf("expected KindMacroCall, got %v", vn.Kind)
	}
	if vn.Raw.(string) != "greet" {
		t.Errorf("expected macro name 'greet', got %q", vn.Raw.(string))
	}
}

// TestParseMacroCallWithArgs verifies parse macro call with args.
func TestParseMacroCallWithArgs(t *testing.T) {
	text := `x := make_pair!("a", "b");`
	prog := testhelpers.ParseText(t, text)
	if len(prog.TopLevelVars) != 1 {
		t.Fatalf("expected 1 var, got %d", len(prog.TopLevelVars))
	}
	vn := prog.TopLevelVars[0].Value
	if vn.Kind != testhelpers.KindMacroCall {
		t.Errorf("expected KindMacroCall, got %v", vn.Kind)
	}
	args := vn.Resolved.([]*testhelpers.ValueNode)
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

// TestKindNameMacroCall verifies kind name macro call.
func TestKindNameMacroCall(t *testing.T) {
	result := testhelpers.KindName(testhelpers.KindMacroCall)
	if result != "macro call" {
		t.Errorf("expected 'macro call', got %q", result)
	}
}
