// intrinsics_functional_test.go contains functional tests for the built-in
// intrinsic functions (the '@' prefix), primarily @split_by, through the full
// SOMETHING pipeline.
//go:build functional

package something

import (
	"reflect"
	"testing"
)

// TestEvalSplitByBasic verifies @split_by splits on a delimiter.
func TestEvalSplitByBasic(t *testing.T) {
	r := evalText(t, `parts := @split_by("a,b,c", ",");`)
	expected := []any{"a", "b", "c"}
	if !reflect.DeepEqual(r["parts"], expected) {
		t.Errorf("expected %v, got %v", expected, r["parts"])
	}
}

// TestEvalSplitByKeepsEmpties verifies @split_by preserves empty segments.
func TestEvalSplitByKeepsEmpties(t *testing.T) {
	r := evalText(t, `parts := @split_by("a,,b", ",");`)
	expected := []any{"a", "", "b"}
	if !reflect.DeepEqual(r["parts"], expected) {
		t.Errorf("expected %v, got %v", expected, r["parts"])
	}
}

// TestEvalSplitByEmptyDelimiter verifies @split_by with an empty delimiter
// splits into individual characters, matching Go strings.Split.
func TestEvalSplitByEmptyDelimiter(t *testing.T) {
	r := evalText(t, `parts := @split_by("abc", "");`)
	expected := []any{"a", "b", "c"}
	if !reflect.DeepEqual(r["parts"], expected) {
		t.Errorf("expected %v, got %v", expected, r["parts"])
	}
}

// TestEvalSplitByVariableInput verifies @split_by accepts a string variable.
func TestEvalSplitByVariableInput(t *testing.T) {
	r := evalText(t, `input := "x;y;z"; parts := @split_by(input, ";");`)
	expected := []any{"x", "y", "z"}
	if !reflect.DeepEqual(r["parts"], expected) {
		t.Errorf("expected %v, got %v", expected, r["parts"])
	}
}

// TestEvalSplitByTypedAssignment verifies @split_by assigns to a typed []string.
func TestEvalSplitByTypedAssignment(t *testing.T) {
	r := evalText(t, `parts: []string = @split_by("a,b", ",");`)
	expected := []any{"a", "b"}
	if !reflect.DeepEqual(r["parts"], expected) {
		t.Errorf("expected %v, got %v", expected, r["parts"])
	}
}

// TestEvalSplitByInFor verifies @split_by is usable as a #for source.
func TestEvalSplitByInFor(t *testing.T) {
	r := evalText(t, `
#for part: @split_by("a,b,c", ",") {
    #iteration("_part") := part;
}
`)
	expected := []any{"a", "b", "c"}
	for index, want := range expected {
		key := "iteration_000000000" + string(rune('0'+index)) + "_part"
		if r[key] != want {
			t.Errorf("expected %q at %s, got %v", want, key, r[key])
		}
	}
}

// TestEvalSplitByInArray verifies @split_by is usable inside an array literal.
func TestEvalSplitByInArray(t *testing.T) {
	r := evalText(t, `nested := [][]string{@split_by("a,b", ","), @split_by("c,d", ",")};`)
	expected := []any{[]any{"a", "b"}, []any{"c", "d"}}
	if !reflect.DeepEqual(r["nested"], expected) {
		t.Errorf("expected %v, got %v", expected, r["nested"])
	}
}

// TestEvalSplitByLen verifies the result of @split_by works with #len.
func TestEvalSplitByLen(t *testing.T) {
	r := evalText(t, `count := #len(@split_by("a,b,c,d", ","));`)
	if r["count"] != 4 {
		t.Errorf("expected 4, got %v", r["count"])
	}
}

// TestEvalSplitByResultIndex verifies member access on an intrinsic result.
func TestEvalSplitByResultIndex(t *testing.T) {
	r := evalText(t, `first := @split_by("a,b,c", ",")[0]; last := @split_by("a,b,c", ",")[2];`)
	if r["first"] != "a" {
		t.Errorf("expected 'a', got %v", r["first"])
	}
	if r["last"] != "c" {
		t.Errorf("expected 'c', got %v", r["last"])
	}
}

// TestEvalSplitByArgumentIndex verifies member access on an intrinsic argument.
func TestEvalSplitByArgumentIndex(t *testing.T) {
	r := evalText(t, `a := []string{"hi how are you doing ?", "hello"}; b := @split_by(a[0], " ");`)
	expected := []any{"hi", "how", "are", "you", "doing", "?"}
	if !reflect.DeepEqual(r["b"], expected) {
		t.Errorf("expected %v, got %v", expected, r["b"])
	}
}

// TestEvalSplitBySetupMemberArgument verifies member access on a setup member argument.
func TestEvalSplitBySetupMemberArgument(t *testing.T) {
	r := evalText(t, `S: setup = { text: string; } s := S { text = "hi there" }; b := @split_by(s.text, " ");`)
	expected := []any{"hi", "there"}
	if !reflect.DeepEqual(r["b"], expected) {
		t.Errorf("expected %v, got %v", expected, r["b"])
	}
}

// TestEvalSplitByResultIndexOutOfBounds verifies an out-of-bounds result index is an error.
func TestEvalSplitByResultIndexOutOfBounds(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x := @split_by("a,b", ",")[5];`)
	}, "Index 5 out of bounds for array of length 2")
}

// TestEvalSplitByResultIndexOnString verifies indexing a string result is a type error.
func TestEvalSplitByResultIndexOnString(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x := @split_by("a,b", ",")[0][0];`)
	}, "Cannot index string")
}

// TestEvalLenResultIndexRejected verifies indexing an integer result is a type error.
func TestEvalLenResultIndexRejected(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x := #len([]string{"a", "b"})[0];`)
	}, "Cannot index integer")
}

// TestEvalSplitByNonStringArgument verifies a non-string argument is a type error.
func TestEvalSplitByNonStringArgument(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `parts := @split_by(42, ",");`)
	}, "Type mismatch in intrinsic '@split_by' argument 'input': expected string, got integer")
}

// TestEvalSplitByWrongArity verifies a wrong argument count is an error.
func TestEvalSplitByWrongArity(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `parts := @split_by("a,b");`)
	}, "Intrinsic '@split_by' expects 2 arguments, got 1")
}

// TestEvalUnknownIntrinsic verifies an unknown intrinsic name is an error.
func TestEvalUnknownIntrinsic(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `parts := @unknown_intrinsic("a", ",");`)
	}, "Unknown intrinsic '@unknown_intrinsic'")
}

// TestEvalConcatBasic verifies @concat joins a list with a delimiter.
func TestEvalConcatBasic(t *testing.T) {
	r := evalText(t, `a := []string{"a", "b", "c"}; b := @concat(a, "");`)
	if r["b"] != "abc" {
		t.Errorf("expected 'abc', got %v", r["b"])
	}
}

// TestEvalConcatWithDelimiter verifies @concat inserts the delimiter between elements.
func TestEvalConcatWithDelimiter(t *testing.T) {
	r := evalText(t, `a := []string{"a", "b", "c"}; b := @concat(a, ",");`)
	if r["b"] != "a,b,c" {
		t.Errorf("expected 'a,b,c', got %v", r["b"])
	}
}

// TestEvalConcatSingleElement verifies @concat with one element returns it unchanged.
func TestEvalConcatSingleElement(t *testing.T) {
	r := evalText(t, `a := []string{"only"}; b := @concat(a, ",");`)
	if r["b"] != "only" {
		t.Errorf("expected 'only', got %v", r["b"])
	}
}

// TestEvalConcatEmptyList verifies @concat with an empty list returns an empty string.
func TestEvalConcatEmptyList(t *testing.T) {
	r := evalText(t, `a := []string{}; b := @concat(a, ",");`)
	if r["b"] != "" {
		t.Errorf("expected '', got %v", r["b"])
	}
}

// TestEvalConcatLiteralList verifies @concat accepts an inline list literal.
func TestEvalConcatLiteralList(t *testing.T) {
	r := evalText(t, `b := @concat([]string{"x", "y"}, "-");`)
	if r["b"] != "x-y" {
		t.Errorf("expected 'x-y', got %v", r["b"])
	}
}

// TestEvalConcatResultIndexRejected verifies indexing a string result is a type error.
func TestEvalConcatResultIndexRejected(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x := @concat([]string{"a", "b"}, "")[0];`)
	}, "Cannot index string")
}

// TestEvalConcatNonStringList verifies a non-string list element is a type error.
func TestEvalConcatNonStringList(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x := @concat([]integer{1, 2}, ",");`)
	}, "Type mismatch in intrinsic '@concat' argument 'list': expected []string, got []integer")
}

// TestEvalAppendPrefixForEachBasic verifies @append_prefix_for_each prefixes each element.
func TestEvalAppendPrefixForEachBasic(t *testing.T) {
	r := evalText(t, `a := []string{"a", "b", "c"}; b := @append_prefix_for_each(a, "something");`)
	expected := []any{"somethinga", "somethingb", "somethingc"}
	if !reflect.DeepEqual(r["b"], expected) {
		t.Errorf("expected %v, got %v", expected, r["b"])
	}
}

// TestEvalAppendPrefixForEachEmptyPrefix verifies an empty prefix leaves elements unchanged.
func TestEvalAppendPrefixForEachEmptyPrefix(t *testing.T) {
	r := evalText(t, `a := []string{"a", "b"}; b := @append_prefix_for_each(a, "");`)
	expected := []any{"a", "b"}
	if !reflect.DeepEqual(r["b"], expected) {
		t.Errorf("expected %v, got %v", expected, r["b"])
	}
}

// TestEvalAppendPrefixForEachEmptyList verifies an empty list stays empty.
func TestEvalAppendPrefixForEachEmptyList(t *testing.T) {
	r := evalText(t, `a := []string{}; b := @append_prefix_for_each(a, "p");`)
	if len(r["b"].([]any)) != 0 {
		t.Errorf("expected empty list, got %v", r["b"])
	}
}

// TestEvalAppendPrefixForEachResultIndex verifies member access on the result.
func TestEvalAppendPrefixForEachResultIndex(t *testing.T) {
	r := evalText(t, `a := []string{"a", "b"}; first := @append_prefix_for_each(a, "p")[0];`)
	if r["first"] != "pa" {
		t.Errorf("expected 'pa', got %v", r["first"])
	}
}

// TestEvalAppendPrefixForEachNonStringList verifies a non-string list is a type error.
func TestEvalAppendPrefixForEachNonStringList(t *testing.T) {
	assertPanic(t, func() {
		evalText(t, `x := @append_prefix_for_each([]integer{1, 2}, "p");`)
	}, "Type mismatch in intrinsic '@append_prefix_for_each' argument 'list': expected []string, got []integer")
}
