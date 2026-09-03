// utils.go implements the public path-walking and typed-accessor functions
// for traversing a resolved SOMETHING config map. It provides the Once, Index,
// and All families of accessors (GetStringOnce, GetIntegerIndex, etc.) and
// the ResolveMemberPath helper for dotted/indexed paths. These functions are
// the intended public API for consuming evaluated configuration values.
package something

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// PathNotValidError is raised when a path segment cannot be found in the current dict.
type PathNotValidError struct {
	Segment string
	Keys    []string
}

// Error returns the receiver's diagnostic message.
func (e *PathNotValidError) Error() string {
	return fmt.Sprintf("path segment %q not found; available keys: %s", e.Segment, strings.Join(e.Keys, ", "))
}

// PathCannotBeReachedError is raised when an intermediate value is not a dict.
type PathCannotBeReachedError struct {
	Segment     string
	CurrentType string
}

// Error returns the receiver's diagnostic message.
func (e *PathCannotBeReachedError) Error() string {
	return fmt.Sprintf("cannot reach path segment %q: value is %s, not a dict", e.Segment, e.CurrentType)
}

// OutOfBoundsError is raised when an index is out of bounds for an iteration segment.
type OutOfBoundsError struct {
	Index   int
	Count   int
	Segment string
}

// Error returns the receiver's diagnostic message.
func (e *OutOfBoundsError) Error() string {
	return fmt.Sprintf("index %d out of bounds for segment %q (count=%d)", e.Index, e.Segment, e.Count)
}

// matchIterationKeys returns all keys in data whose name matches segment with
// "[iteration]" expanded to "iteration_" + 10 digits, sorted by counter ascending.
// When segment does not contain "[iteration]" the list contains at most one key.
func matchIterationKeys(data map[string]any, segment string) []string {
	if !strings.Contains(segment, "[iteration]") {
		if _, ok := data[segment]; ok {
			return []string{segment}
		}
		return nil
	}

	// Split on first [iteration]
	parts := strings.SplitN(segment, "[iteration]", 2)
	prefix := parts[0]
	suffix := ""
	if len(parts) > 1 {
		suffix = parts[1]
	}

	type match struct {
		key     string
		counter int
	}
	var matches []match

	for key := range data {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if !strings.HasSuffix(key, suffix) {
			continue
		}

		middle := key[len(prefix) : len(key)-len(suffix)]
		if !strings.HasPrefix(middle, "iteration_") {
			continue
		}

		digits := middle[10:] // len("iteration_") == 10
		if len(digits) != 10 {
			continue
		}
		counter, err := strconv.Atoi(digits)
		if err != nil {
			continue
		}

		matches = append(matches, match{key: key, counter: counter})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].counter < matches[j].counter
	})

	result := make([]string, len(matches))
	for i, m := range matches {
		result[i] = m.key
	}
	return result
}

// checkDict checks dict against the current invariants.
func checkDict(current any, segment string) (map[string]any, error) {
	d, ok := current.(map[string]any)
	if !ok {
		return nil, &PathCannotBeReachedError{
			Segment:     segment,
			CurrentType: fmt.Sprintf("%T", current),
		}
	}
	return d, nil
}

// walkMode selects whether iteration segments require one match, a selected
// match, or all matches.
type walkMode int

const (
	walkOne walkMode = iota
	walkAtIndex
	walkEvery
)

// walk traverses one path according to mode. It is the shared primitive for
// the public Once, Index, and All accessor families.
func walk(data map[string]any, segments []string, mode walkMode, index int) ([]any, error) {
	return walkFrom(data, segments, mode, index)
}

// walkFrom continues a traversal from the current value while preserving the
// existing public error types and path text for every selection mode.
func walkFrom(current any, segments []string, mode walkMode, index int) ([]any, error) {
	if len(segments) == 0 {
		return []any{current}, nil
	}

	segment := segments[0]
	rest := segments[1:]
	d, err := checkDict(current, segment)
	if err != nil {
		return nil, err
	}
	keys := matchIterationKeys(d, segment)
	if len(keys) == 0 {
		if mode == walkEvery && strings.Contains(segment, "[iteration]") {
			return nil, nil
		}
		return nil, &PathNotValidError{Segment: segment, Keys: mapKeys(d)}
	}

	switch mode {
	case walkOne:
		if len(keys) > 1 {
			return nil, &PathNotValidError{Segment: segment, Keys: mapKeys(d)}
		}
		return walkFrom(d[keys[0]], rest, mode, index)
	case walkAtIndex:
		key := keys[0]
		if strings.Contains(segment, "[iteration]") {
			if index < 0 || index >= len(keys) {
				return nil, &OutOfBoundsError{Index: index, Count: len(keys), Segment: segment}
			}
			key = keys[index]
		}
		return walkFrom(d[key], rest, mode, index)
	case walkEvery:
		results := make([]any, 0, len(keys))
		for _, key := range keys {
			if len(rest) == 0 {
				results = append(results, d[key])
				continue
			}
			subMap, ok := d[key].(map[string]any)
			if !ok {
				return nil, &PathCannotBeReachedError{Segment: key, CurrentType: fmt.Sprintf("%T", d[key])}
			}
			subResults, err := walkFrom(subMap, rest, mode, index)
			if err != nil {
				return nil, err
			}
			results = append(results, subResults...)
		}
		return results, nil
	default:
		return nil, fmt.Errorf("unknown path traversal mode %d", mode)
	}
}

// walkOnce returns the one value selected by a Once traversal.
func walkOnce(data map[string]any, segments []string) (any, error) {
	values, err := walk(data, segments, walkOne, 0)
	if err != nil {
		return nil, err
	}
	return values[0], nil
}

// walkIndex returns the one value selected by an Index traversal.
func walkIndex(data map[string]any, index int, segments []string) (any, error) {
	values, err := walk(data, segments, walkAtIndex, index)
	if err != nil {
		return nil, err
	}
	return values[0], nil
}

// walkAll returns every value selected by an All traversal.
func walkAll(data map[string]any, segments []string) ([]any, error) {
	return walk(data, segments, walkEvery, 0)
}

// checkType checks type against the current invariants.
func checkType(val any, expected string, path []string) error {
	pathStr := strings.Join(path, ".")
	switch expected {
	case "string":
		if _, ok := val.(string); !ok {
			return fmt.Errorf("expected string at %q, got %T", pathStr, val)
		}
	case "integer":
		if _, ok := val.(int); !ok {
			return fmt.Errorf("expected integer at %q, got %T", pathStr, val)
		}
	case "float":
		if _, ok := val.(float64); !ok {
			if _, ok := val.(int); !ok {
				return fmt.Errorf("expected float at %q, got %T", pathStr, val)
			}
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("expected boolean at %q, got %T", pathStr, val)
		}
	case "list":
		if _, ok := val.([]any); !ok {
			return fmt.Errorf("expected list at %q, got %T", pathStr, val)
		}
	case "mapping", "struct", "scope":
		if _, ok := val.(map[string]any); !ok {
			return fmt.Errorf("expected %s at %q, got %T", expected, pathStr, val)
		}
	case "enum":
		if _, ok := val.(int); !ok {
			return fmt.Errorf("expected enum (int) at %q, got %T", pathStr, val)
		}
	}
	return nil
}

// valueAt selects and checks one typed value for the Once and Index accessors.
func valueAt(data map[string]any, path []string, mode walkMode, index int, expected string) (any, error) {
	values, err := walk(data, path, mode, index)
	if err != nil {
		return nil, err
	}
	value := values[0]
	if err := checkType(value, expected, path); err != nil {
		return nil, err
	}
	return value, nil
}

// valuesAt selects and checks every typed value for the All accessors.
func valuesAt(data map[string]any, path []string, expected string) ([]any, error) {
	values, err := walk(data, path, walkEvery, 0)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if err := checkType(value, expected, path); err != nil {
			return nil, err
		}
	}
	return values, nil
}

// GetStringOnce returns a string value at path.
func GetStringOnce(data map[string]any, path ...string) (string, error) {
	val, err := valueAt(data, path, walkOne, 0, "string")
	if err != nil {
		return "", err
	}
	return val.(string), nil
}

// GetIntegerOnce returns an integer value at path.
func GetIntegerOnce(data map[string]any, path ...string) (int, error) {
	val, err := valueAt(data, path, walkOne, 0, "integer")
	if err != nil {
		return 0, err
	}
	return val.(int), nil
}

// GetFloatOnce returns a float value at path.
func GetFloatOnce(data map[string]any, path ...string) (float64, error) {
	val, err := valueAt(data, path, walkOne, 0, "float")
	if err != nil {
		return 0, err
	}
	if f, ok := val.(float64); ok {
		return f, nil
	}
	return float64(val.(int)), nil
}

// GetBoolOnce returns a boolean value at path.
func GetBoolOnce(data map[string]any, path ...string) (bool, error) {
	val, err := valueAt(data, path, walkOne, 0, "boolean")
	if err != nil {
		return false, err
	}
	return val.(bool), nil
}

// GetTimestampOnce returns a timestamp (string) value at path.
func GetTimestampOnce(data map[string]any, path ...string) (string, error) {
	val, err := valueAt(data, path, walkOne, 0, "string")
	if err != nil {
		return "", err
	}
	return val.(string), nil
}

// GetListOnce returns a list value at path.
func GetListOnce(data map[string]any, path ...string) ([]any, error) {
	val, err := valueAt(data, path, walkOne, 0, "list")
	if err != nil {
		return nil, err
	}
	return val.([]any), nil
}

// GetMappingOnce returns a mapping (dict) value at path.
func GetMappingOnce(data map[string]any, path ...string) (map[string]any, error) {
	val, err := valueAt(data, path, walkOne, 0, "mapping")
	if err != nil {
		return nil, err
	}
	return val.(map[string]any), nil
}

// GetStructOnce returns a struct (dict) value at path.
func GetStructOnce(data map[string]any, path ...string) (map[string]any, error) {
	val, err := valueAt(data, path, walkOne, 0, "struct")
	if err != nil {
		return nil, err
	}
	return val.(map[string]any), nil
}

// GetScopeOnce returns a scope (dict) value at path.
func GetScopeOnce(data map[string]any, path ...string) (map[string]any, error) {
	val, err := valueAt(data, path, walkOne, 0, "scope")
	if err != nil {
		return nil, err
	}
	return val.(map[string]any), nil
}

// GetEnumOnce returns an enum (integer ordinal) value at path.
func GetEnumOnce(data map[string]any, path ...string) (int, error) {
	val, err := valueAt(data, path, walkOne, 0, "enum")
	if err != nil {
		return 0, err
	}
	return val.(int), nil
}

// GetStringIndex returns a string at path, using index to select among iterations.
func GetStringIndex(data map[string]any, index int, path ...string) (string, error) {
	val, err := valueAt(data, path, walkAtIndex, index, "string")
	if err != nil {
		return "", err
	}
	return val.(string), nil
}

// GetIntegerIndex returns an integer at path, using index to select among iterations.
func GetIntegerIndex(data map[string]any, index int, path ...string) (int, error) {
	val, err := valueAt(data, path, walkAtIndex, index, "integer")
	if err != nil {
		return 0, err
	}
	return val.(int), nil
}

// GetFloatIndex returns a float at path, using index to select among iterations.
func GetFloatIndex(data map[string]any, index int, path ...string) (float64, error) {
	val, err := valueAt(data, path, walkAtIndex, index, "float")
	if err != nil {
		return 0, err
	}
	if f, ok := val.(float64); ok {
		return f, nil
	}
	return float64(val.(int)), nil
}

// GetBoolIndex returns a boolean at path, using index to select among iterations.
func GetBoolIndex(data map[string]any, index int, path ...string) (bool, error) {
	val, err := valueAt(data, path, walkAtIndex, index, "boolean")
	if err != nil {
		return false, err
	}
	return val.(bool), nil
}

// GetTimestampIndex returns a timestamp at path, using index to select among iterations.
func GetTimestampIndex(data map[string]any, index int, path ...string) (string, error) {
	val, err := valueAt(data, path, walkAtIndex, index, "string")
	if err != nil {
		return "", err
	}
	return val.(string), nil
}

// GetListIndex returns a list at path, using index to select among iterations.
func GetListIndex(data map[string]any, index int, path ...string) ([]any, error) {
	val, err := valueAt(data, path, walkAtIndex, index, "list")
	if err != nil {
		return nil, err
	}
	return val.([]any), nil
}

// GetMappingIndex returns a mapping at path, using index to select among iterations.
func GetMappingIndex(data map[string]any, index int, path ...string) (map[string]any, error) {
	val, err := valueAt(data, path, walkAtIndex, index, "mapping")
	if err != nil {
		return nil, err
	}
	return val.(map[string]any), nil
}

// GetStructIndex returns a struct at path, using index to select among iterations.
func GetStructIndex(data map[string]any, index int, path ...string) (map[string]any, error) {
	val, err := valueAt(data, path, walkAtIndex, index, "struct")
	if err != nil {
		return nil, err
	}
	return val.(map[string]any), nil
}

// GetScopeIndex returns a scope at path, using index to select among iterations.
func GetScopeIndex(data map[string]any, index int, path ...string) (map[string]any, error) {
	val, err := valueAt(data, path, walkAtIndex, index, "scope")
	if err != nil {
		return nil, err
	}
	return val.(map[string]any), nil
}

// GetEnumIndex returns an enum at path, using index to select among iterations.
func GetEnumIndex(data map[string]any, index int, path ...string) (int, error) {
	val, err := valueAt(data, path, walkAtIndex, index, "enum")
	if err != nil {
		return 0, err
	}
	return val.(int), nil
}

// GetStringAll returns all string values reachable at path.
func GetStringAll(data map[string]any, path ...string) ([]string, error) {
	vals, err := valuesAt(data, path, "string")
	if err != nil {
		return nil, err
	}
	result := make([]string, len(vals))
	for i, v := range vals {
		result[i] = v.(string)
	}
	return result, nil
}

// GetIntegerAll returns all integer values reachable at path.
func GetIntegerAll(data map[string]any, path ...string) ([]int, error) {
	vals, err := valuesAt(data, path, "integer")
	if err != nil {
		return nil, err
	}
	result := make([]int, len(vals))
	for i, v := range vals {
		result[i] = v.(int)
	}
	return result, nil
}

// GetFloatAll returns all float values reachable at path.
func GetFloatAll(data map[string]any, path ...string) ([]float64, error) {
	vals, err := valuesAt(data, path, "float")
	if err != nil {
		return nil, err
	}
	result := make([]float64, len(vals))
	for i, v := range vals {
		if f, ok := v.(float64); ok {
			result[i] = f
		} else {
			result[i] = float64(v.(int))
		}
	}
	return result, nil
}

// GetBoolAll returns all boolean values reachable at path.
func GetBoolAll(data map[string]any, path ...string) ([]bool, error) {
	vals, err := valuesAt(data, path, "boolean")
	if err != nil {
		return nil, err
	}
	result := make([]bool, len(vals))
	for i, v := range vals {
		result[i] = v.(bool)
	}
	return result, nil
}

// GetTimestampAll returns all timestamp values reachable at path.
func GetTimestampAll(data map[string]any, path ...string) ([]string, error) {
	vals, err := valuesAt(data, path, "string")
	if err != nil {
		return nil, err
	}
	result := make([]string, len(vals))
	for i, v := range vals {
		result[i] = v.(string)
	}
	return result, nil
}

// GetListAll returns all list values reachable at path.
func GetListAll(data map[string]any, path ...string) ([]any, error) {
	return valuesAt(data, path, "list")
}

// GetMappingAll returns all mapping values reachable at path.
func GetMappingAll(data map[string]any, path ...string) ([]map[string]any, error) {
	vals, err := valuesAt(data, path, "mapping")
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(vals))
	for i, v := range vals {
		result[i] = v.(map[string]any)
	}
	return result, nil
}

// GetStructAll returns all struct values reachable at path.
func GetStructAll(data map[string]any, path ...string) ([]map[string]any, error) {
	vals, err := valuesAt(data, path, "struct")
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(vals))
	for i, v := range vals {
		result[i] = v.(map[string]any)
	}
	return result, nil
}

// GetScopeAll returns all scope values reachable at path.
func GetScopeAll(data map[string]any, path ...string) ([]map[string]any, error) {
	vals, err := valuesAt(data, path, "scope")
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(vals))
	for i, v := range vals {
		result[i] = v.(map[string]any)
	}
	return result, nil
}

// GetEnumAll returns all enum (int) values reachable at path.
func GetEnumAll(data map[string]any, path ...string) ([]int, error) {
	vals, err := valuesAt(data, path, "enum")
	if err != nil {
		return nil, err
	}
	result := make([]int, len(vals))
	for i, v := range vals {
		result[i] = v.(int)
	}
	return result, nil
}
