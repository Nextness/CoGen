// utils_unit_test.go contains unit tests for utils.go: the Once, Index, and
// All families of accessors, plus ResolveMemberPath and error formatting.
//go:build unit

package something

import (
	"strings"
	"testing"
)

// TestMatchIterationKeysPlain verifies match iteration keys plain.
func TestMatchIterationKeysPlain(t *testing.T) {
	data := testData()
	keys := matchIterationKeys(data, "name")
	if len(keys) != 1 || keys[0] != "name" {
		t.Fatalf("expected ['name'], got %v", keys)
	}
}

// TestMatchIterationKeysPlainMissing verifies match iteration keys plain missing.
func TestMatchIterationKeysPlainMissing(t *testing.T) {
	data := testData()
	keys := matchIterationKeys(data, "nonexistent")
	if len(keys) != 0 {
		t.Fatalf("expected empty, got %v", keys)
	}
}

// TestMatchIterationKeysWildcard verifies match iteration keys wildcard.
func TestMatchIterationKeysWildcard(t *testing.T) {
	data := testData()
	keys := matchIterationKeys(data, "[iteration]")
	if len(keys) != 3 {
		t.Fatalf("expected 3 iteration keys, got %d: %v", len(keys), keys)
	}
	if keys[0] != "iteration_0000000000" {
		t.Errorf("expected first key 'iteration_0000000000', got %q", keys[0])
	}
	if keys[2] != "iteration_0000000002" {
		t.Errorf("expected third key 'iteration_0000000002', got %q", keys[2])
	}
}

// TestMatchIterationKeysWildcardWithSuffix verifies match iteration keys wildcard with suffix.
func TestMatchIterationKeysWildcardWithSuffix(t *testing.T) {
	data := testData()
	keys := matchIterationKeys(data, "[iteration]_label")
	if len(keys) != 2 {
		t.Fatalf("expected 2 iteration keys with '_label' suffix, got %d: %v", len(keys), keys)
	}
	if keys[0] != "iteration_0000000000_label" {
		t.Errorf("expected 'iteration_0000000000_label', got %q", keys[0])
	}
}

// TestMatchIterationKeysWildcardNoMatch verifies match iteration keys wildcard no match.
func TestMatchIterationKeysWildcardNoMatch(t *testing.T) {
	data := testData()
	keys := matchIterationKeys(data, "[iteration]_nonexistent")
	if len(keys) != 0 {
		t.Fatalf("expected empty, got %v", keys)
	}
}

// TestWalkOnceSimple verifies walk once simple.
func TestWalkOnceSimple(t *testing.T) {
	data := testData()
	val, err := walkOnce(data, []string{"name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "test" {
		t.Errorf("expected 'test', got %v", val)
	}
}

// TestWalkOnceNested verifies walk once nested.
func TestWalkOnceNested(t *testing.T) {
	data := testData()
	val, err := walkOnce(data, []string{"metadata", "version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 1 {
		t.Errorf("expected 1, got %v", val)
	}
}

// TestWalkOnceIteration verifies walk once iteration.
func TestWalkOnceIteration(t *testing.T) {
	data := testData()
	// Use a specific iteration key (exact match) to test walkOnce
	val, err := walkOnce(data, []string{"iteration_0000000000", "title"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "first" {
		t.Errorf("expected 'first', got %v", val)
	}
}

// TestWalkOnceIterationWithSuffix verifies walk once iteration with suffix.
func TestWalkOnceIterationWithSuffix(t *testing.T) {
	data := testData()
	val, err := walkOnce(data, []string{"iteration_0000000000_label", "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "labeled" {
		t.Errorf("expected 'labeled', got %v", val)
	}
}

// TestWalkOncePathNotValid verifies walk once path not valid.
func TestWalkOncePathNotValid(t *testing.T) {
	data := testData()
	_, err := walkOnce(data, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*PathNotValidError); !ok {
		t.Fatalf("expected *PathNotValidError, got %T: %v", err, err)
	}
}

// TestWalkOncePathCannotBeReached verifies walk once path cannot be reached.
func TestWalkOncePathCannotBeReached(t *testing.T) {
	data := testData()
	_, err := walkOnce(data, []string{"name", "inner"})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*PathCannotBeReachedError); !ok {
		t.Fatalf("expected *PathCannotBeReachedError, got %T: %v", err, err)
	}
}

// TestWalkOnceMultipleMatches verifies walk once multiple matches.
func TestWalkOnceMultipleMatches(t *testing.T) {
	data := testData()
	_, err := walkOnce(data, []string{"[iteration]"})
	if err == nil {
		t.Fatal("expected error for multiple matches")
	}
	if _, ok := err.(*PathNotValidError); !ok {
		t.Fatalf("expected *PathNotValidError, got %T: %v", err, err)
	}
}

// TestWalkIndexSimple verifies walk index simple.
func TestWalkIndexSimple(t *testing.T) {
	data := testData()
	val, err := walkIndex(data, 0, []string{"name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "test" {
		t.Errorf("expected 'test', got %v", val)
	}
}

// TestWalkIndexIteration verifies walk index iteration.
func TestWalkIndexIteration(t *testing.T) {
	data := testData()
	val, err := walkIndex(data, 1, []string{"[iteration]", "title"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "second" {
		t.Errorf("expected 'second', got %v", val)
	}
}

// TestWalkIndexIterationLast verifies walk index iteration last.
func TestWalkIndexIterationLast(t *testing.T) {
	data := testData()
	val, err := walkIndex(data, 2, []string{"[iteration]", "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 300 {
		t.Errorf("expected 300, got %v", val)
	}
}

// TestWalkIndexOutOfBounds verifies walk index out of bounds.
func TestWalkIndexOutOfBounds(t *testing.T) {
	data := testData()
	_, err := walkIndex(data, 5, []string{"[iteration]"})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*OutOfBoundsError); !ok {
		t.Fatalf("expected *OutOfBoundsError, got %T: %v", err, err)
	}
}

// TestWalkIndexNegative verifies walk index negative.
func TestWalkIndexNegative(t *testing.T) {
	data := testData()
	_, err := walkIndex(data, -1, []string{"[iteration]"})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*OutOfBoundsError); !ok {
		t.Fatalf("expected *OutOfBoundsError, got %T: %v", err, err)
	}
}

// TestWalkIndexPathNotValid verifies walk index path not valid.
func TestWalkIndexPathNotValid(t *testing.T) {
	data := testData()
	_, err := walkIndex(data, 0, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*PathNotValidError); !ok {
		t.Fatalf("expected *PathNotValidError, got %T: %v", err, err)
	}
}

// TestWalkAllSimple verifies walk all simple.
func TestWalkAllSimple(t *testing.T) {
	data := testData()
	vals, err := walkAll(data, []string{"name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vals) != 1 || vals[0] != "test" {
		t.Errorf("expected ['test'], got %v", vals)
	}
}

// TestWalkAllIteration verifies walk all iteration.
func TestWalkAllIteration(t *testing.T) {
	data := testData()
	vals, err := walkAll(data, []string{"[iteration]", "title"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vals) != 3 {
		t.Fatalf("expected 3 values, got %d: %v", len(vals), vals)
	}
	if vals[0] != "first" || vals[1] != "second" || vals[2] != "third" {
		t.Errorf("expected ['first', 'second', 'third'], got %v", vals)
	}
}

// TestWalkAllIterationWithSuffix verifies walk all iteration with suffix.
func TestWalkAllIterationWithSuffix(t *testing.T) {
	data := testData()
	vals, err := walkAll(data, []string{"[iteration]_label", "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vals) != 2 {
		t.Fatalf("expected 2 values, got %d: %v", len(vals), vals)
	}
}

// TestWalkAllEmptyResult verifies walk all empty result.
func TestWalkAllEmptyResult(t *testing.T) {
	data := testData()
	vals, err := walkAll(data, []string{"[iteration]_nonexistent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vals) != 0 {
		t.Fatalf("expected empty, got %v", vals)
	}
}

// TestWalkAllPathNotValid verifies walk all path not valid.
func TestWalkAllPathNotValid(t *testing.T) {
	data := testData()
	_, err := walkAll(data, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*PathNotValidError); !ok {
		t.Fatalf("expected *PathNotValidError, got %T: %v", err, err)
	}
}

// TestGetStringOnce verifies get string once.
func TestGetStringOnce(t *testing.T) {
	data := testData()
	v, err := GetStringOnce(data, "name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "test" {
		t.Errorf("expected 'test', got %q", v)
	}
}

// TestGetStringOnceTypeError verifies get string once type error.
func TestGetStringOnceTypeError(t *testing.T) {
	data := testData()
	_, err := GetStringOnce(data, "count")
	if err == nil {
		t.Fatal("expected type error")
	}
	if !strings.Contains(err.Error(), "expected string") {
		t.Errorf("expected 'expected string', got %v", err)
	}
}

// TestGetIntegerOnce verifies get integer once.
func TestGetIntegerOnce(t *testing.T) {
	data := testData()
	v, err := GetIntegerOnce(data, "count")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 42 {
		t.Errorf("expected 42, got %d", v)
	}
}

// TestGetFloatOnce verifies get float once.
func TestGetFloatOnce(t *testing.T) {
	data := testData()
	v, err := GetFloatOnce(data, "ratio")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 3.14 {
		t.Errorf("expected 3.14, got %f", v)
	}
}

// TestGetFloatOnceFromInt verifies get float once from int.
func TestGetFloatOnceFromInt(t *testing.T) {
	data := testData()
	v, err := GetFloatOnce(data, "count")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 42.0 {
		t.Errorf("expected 42.0, got %f", v)
	}
}

// TestGetBoolOnce verifies get bool once.
func TestGetBoolOnce(t *testing.T) {
	data := testData()
	v, err := GetBoolOnce(data, "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != true {
		t.Errorf("expected true, got %v", v)
	}
}

// TestGetTimestampOnce verifies get timestamp once.
func TestGetTimestampOnce(t *testing.T) {
	data := map[string]any{"ts": "2026-01-01 22:10:01"}
	v, err := GetTimestampOnce(data, "ts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "2026-01-01 22:10:01" {
		t.Errorf("expected timestamp, got %q", v)
	}
}

// TestGetListOnce verifies get list once.
func TestGetListOnce(t *testing.T) {
	data := testData()
	v, err := GetListOnce(data, "tags")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(v))
	}
}

// TestGetMappingOnce verifies get mapping once.
func TestGetMappingOnce(t *testing.T) {
	data := testData()
	v, err := GetMappingOnce(data, "metadata")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v["version"] != 1 {
		t.Errorf("expected version 1, got %v", v["version"])
	}
}

// TestGetStructOnce verifies get struct once.
func TestGetStructOnce(t *testing.T) {
	data := testData()
	v, err := GetStructOnce(data, "metadata")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v["author"] != "me" {
		t.Errorf("expected 'me', got %v", v["author"])
	}
}

// TestGetScopeOnce verifies get scope once.
func TestGetScopeOnce(t *testing.T) {
	data := map[string]any{
		"myscope": map[string]any{"x": 1},
	}
	v, err := GetScopeOnce(data, "myscope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v["x"] != 1 {
		t.Errorf("expected 1, got %v", v["x"])
	}
}

// TestGetEnumOnce verifies get enum once.
func TestGetEnumOnce(t *testing.T) {
	data := map[string]any{"color": 2}
	v, err := GetEnumOnce(data, "color")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 2 {
		t.Errorf("expected 2, got %d", v)
	}
}

// TestGetEnumOnceTypeError verifies get enum once type error.
func TestGetEnumOnceTypeError(t *testing.T) {
	data := map[string]any{"color": "red"}
	_, err := GetEnumOnce(data, "color")
	if err == nil {
		t.Fatal("expected type error")
	}
}

// TestGetStringIndex verifies get string index.
func TestGetStringIndex(t *testing.T) {
	data := testData()
	v, err := GetStringIndex(data, 1, "[iteration]", "title")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "second" {
		t.Errorf("expected 'second', got %q", v)
	}
}

// TestGetIntegerIndex verifies get integer index.
func TestGetIntegerIndex(t *testing.T) {
	data := testData()
	v, err := GetIntegerIndex(data, 2, "[iteration]", "value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 300 {
		t.Errorf("expected 300, got %d", v)
	}
}

// TestGetFloatIndex verifies get float index.
func TestGetFloatIndex(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"val": 1.5},
		"iteration_0000000001": map[string]any{"val": 2.5},
	}
	v, err := GetFloatIndex(data, 0, "[iteration]", "val")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 1.5 {
		t.Errorf("expected 1.5, got %f", v)
	}
}

// TestGetBoolIndex verifies get bool index.
func TestGetBoolIndex(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"flag": true},
	}
	v, err := GetBoolIndex(data, 0, "[iteration]", "flag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != true {
		t.Errorf("expected true, got %v", v)
	}
}

// TestGetTimestampIndex verifies get timestamp index.
func TestGetTimestampIndex(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"ts": "2026-01-01 22:10:01"},
	}
	v, err := GetTimestampIndex(data, 0, "[iteration]", "ts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "2026-01-01 22:10:01" {
		t.Errorf("expected timestamp, got %q", v)
	}
}

// TestGetListIndex verifies get list index.
func TestGetListIndex(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"items": []any{1, 2, 3}},
	}
	v, err := GetListIndex(data, 0, "[iteration]", "items")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v) != 3 {
		t.Fatalf("expected 3 items, got %d", len(v))
	}
}

// TestGetMappingIndex verifies get mapping index.
func TestGetMappingIndex(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"cfg": map[string]any{"key": "val"}},
	}
	v, err := GetMappingIndex(data, 0, "[iteration]", "cfg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v["key"] != "val" {
		t.Errorf("expected 'val', got %v", v["key"])
	}
}

// TestGetStructIndex verifies get struct index.
func TestGetStructIndex(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"point": map[string]any{"x": 10}},
	}
	v, err := GetStructIndex(data, 0, "[iteration]", "point")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v["x"] != 10 {
		t.Errorf("expected 10, got %v", v["x"])
	}
}

// TestGetScopeIndex verifies get scope index.
func TestGetScopeIndex(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"inner": map[string]any{"a": 1}},
	}
	v, err := GetScopeIndex(data, 0, "[iteration]", "inner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v["a"] != 1 {
		t.Errorf("expected 1, got %v", v["a"])
	}
}

// TestGetEnumIndex verifies get enum index.
func TestGetEnumIndex(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"color": 2},
	}
	v, err := GetEnumIndex(data, 0, "[iteration]", "color")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 2 {
		t.Errorf("expected 2, got %d", v)
	}
}

// TestGetStringIndexOutOfBounds verifies get string index out of bounds.
func TestGetStringIndexOutOfBounds(t *testing.T) {
	data := testData()
	_, err := GetStringIndex(data, 10, "[iteration]", "title")
	if err == nil {
		t.Fatal("expected out of bounds error")
	}
	if _, ok := err.(*OutOfBoundsError); !ok {
		t.Fatalf("expected *OutOfBoundsError, got %T: %v", err, err)
	}
}

// TestGetStringAll verifies get string all.
func TestGetStringAll(t *testing.T) {
	data := testData()
	v, err := GetStringAll(data, "[iteration]", "title")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v) != 3 {
		t.Fatalf("expected 3 values, got %d: %v", len(v), v)
	}
	if v[0] != "first" || v[1] != "second" || v[2] != "third" {
		t.Errorf("expected ['first', 'second', 'third'], got %v", v)
	}
}

// TestGetIntegerAll verifies get integer all.
func TestGetIntegerAll(t *testing.T) {
	data := testData()
	v, err := GetIntegerAll(data, "[iteration]", "value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v) != 3 {
		t.Fatalf("expected 3 values, got %d: %v", len(v), v)
	}
	if v[0] != 100 || v[1] != 200 || v[2] != 300 {
		t.Errorf("expected [100, 200, 300], got %v", v)
	}
}

// TestGetFloatAll verifies get float all.
func TestGetFloatAll(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"val": 1.5},
		"iteration_0000000001": map[string]any{"val": 2.5},
	}
	v, err := GetFloatAll(data, "[iteration]", "val")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v) != 2 {
		t.Fatalf("expected 2 values, got %d", len(v))
	}
}

// TestGetBoolAll verifies get bool all.
func TestGetBoolAll(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"flag": true},
		"iteration_0000000001": map[string]any{"flag": false},
	}
	v, err := GetBoolAll(data, "[iteration]", "flag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v) != 2 {
		t.Fatalf("expected 2 values, got %d", len(v))
	}
	if v[0] != true || v[1] != false {
		t.Errorf("expected [true, false], got %v", v)
	}
}

// TestGetTimestampAll verifies get timestamp all.
func TestGetTimestampAll(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"ts": "2026-01-01 22:10:01"},
		"iteration_0000000001": map[string]any{"ts": "2026-02-01 22:10:01"},
	}
	v, err := GetTimestampAll(data, "[iteration]", "ts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v) != 2 {
		t.Fatalf("expected 2 values, got %d", len(v))
	}
}

// TestGetListAll verifies get list all.
func TestGetListAll(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"items": []any{1, 2}},
		"iteration_0000000001": map[string]any{"items": []any{3, 4}},
	}
	v, err := GetListAll(data, "[iteration]", "items")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v) != 2 {
		t.Fatalf("expected 2 values, got %d", len(v))
	}
}

// TestGetMappingAll verifies get mapping all.
func TestGetMappingAll(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"cfg": map[string]any{"a": 1}},
		"iteration_0000000001": map[string]any{"cfg": map[string]any{"b": 2}},
	}
	v, err := GetMappingAll(data, "[iteration]", "cfg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v) != 2 {
		t.Fatalf("expected 2 values, got %d", len(v))
	}
}

// TestGetStructAll verifies get struct all.
func TestGetStructAll(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"point": map[string]any{"x": 10}},
		"iteration_0000000001": map[string]any{"point": map[string]any{"x": 20}},
	}
	v, err := GetStructAll(data, "[iteration]", "point")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v) != 2 {
		t.Fatalf("expected 2 values, got %d", len(v))
	}
}

// TestGetScopeAll verifies get scope all.
func TestGetScopeAll(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"inner": map[string]any{"a": 1}},
		"iteration_0000000001": map[string]any{"inner": map[string]any{"a": 2}},
	}
	v, err := GetScopeAll(data, "[iteration]", "inner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v) != 2 {
		t.Fatalf("expected 2 values, got %d", len(v))
	}
}

// TestGetEnumAll verifies get enum all.
func TestGetEnumAll(t *testing.T) {
	data := map[string]any{
		"iteration_0000000000": map[string]any{"color": 0},
		"iteration_0000000001": map[string]any{"color": 1},
	}
	v, err := GetEnumAll(data, "[iteration]", "color")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v) != 2 {
		t.Fatalf("expected 2 values, got %d", len(v))
	}
	if v[0] != 0 || v[1] != 1 {
		t.Errorf("expected [0, 1], got %v", v)
	}
}

// TestGetStringAllEmpty verifies get string all empty.
func TestGetStringAllEmpty(t *testing.T) {
	data := testData()
	v, err := GetStringAll(data, "[iteration]_nonexistent", "title")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v) != 0 {
		t.Fatalf("expected empty, got %v", v)
	}
}

// TestPathNotValidError verifies path not valid error.
func TestPathNotValidError(t *testing.T) {
	err := &PathNotValidError{Segment: "foo", Keys: []string{"a", "b"}}
	msg := err.Error()
	if !strings.Contains(msg, "foo") || !strings.Contains(msg, "a") {
		t.Errorf("unexpected error message: %q", msg)
	}
}

// TestPathCannotBeReachedError verifies path cannot be reached error.
func TestPathCannotBeReachedError(t *testing.T) {
	err := &PathCannotBeReachedError{Segment: "foo", CurrentType: "string"}
	msg := err.Error()
	if !strings.Contains(msg, "foo") || !strings.Contains(msg, "string") {
		t.Errorf("unexpected error message: %q", msg)
	}
}

// TestOutOfBoundsError verifies out of bounds error.
func TestOutOfBoundsError(t *testing.T) {
	err := &OutOfBoundsError{Index: 5, Count: 3, Segment: "[iteration]"}
	msg := err.Error()
	if !strings.Contains(msg, "5") || !strings.Contains(msg, "3") {
		t.Errorf("unexpected error message: %q", msg)
	}
}
