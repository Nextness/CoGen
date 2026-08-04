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

// walkOnce walks segments through data. Every segment containing "[iteration]"
// must match exactly one key. Returns the final value.
func walkOnce(data map[string]any, segments []string) (any, error) {
	current := any(data)
	for _, segment := range segments {
		d, err := checkDict(current, segment)
		if err != nil {
			return nil, err
		}
		keys := matchIterationKeys(d, segment)
		if len(keys) == 0 {
			return nil, &PathNotValidError{Segment: segment, Keys: mapKeys(d)}
		}
		if len(keys) > 1 {
			return nil, &PathNotValidError{Segment: segment, Keys: mapKeys(d)}
		}
		current = d[keys[0]]
	}
	return current, nil
}

// walkIndex walks segments through data, using index to select among matching
// iteration keys. The index is only applied to segments containing "[iteration]".
func walkIndex(data map[string]any, index int, segments []string) (any, error) {
	current := any(data)
	for _, segment := range segments {
		d, err := checkDict(current, segment)
		if err != nil {
			return nil, err
		}
		keys := matchIterationKeys(d, segment)
		if len(keys) == 0 {
			return nil, &PathNotValidError{Segment: segment, Keys: mapKeys(d)}
		}
		if strings.Contains(segment, "[iteration]") {
			if index < 0 || index >= len(keys) {
				return nil, &OutOfBoundsError{Index: index, Count: len(keys), Segment: segment}
			}
			current = d[keys[index]]
		} else {
			current = d[keys[0]]
		}
	}
	return current, nil
}

// walkAll walks segments through data, branching on every "[iteration]" segment
// that matches multiple keys. Returns all terminal values reachable.
func walkAll(data map[string]any, segments []string) ([]any, error) {
	if len(segments) == 0 {
		return []any{data}, nil
	}

	segment := segments[0]
	rest := segments[1:]

	d, err := checkDict(data, segment)
	if err != nil {
		return nil, err
	}
	keys := matchIterationKeys(d, segment)
	if len(keys) == 0 {
		if strings.Contains(segment, "[iteration]") {
			return nil, nil
		}
		return nil, &PathNotValidError{Segment: segment, Keys: mapKeys(d)}
	}

	var results []any
	for _, key := range keys {
		if len(rest) == 0 {
			results = append(results, d[key])
			continue
		}
		subMap, ok := d[key].(map[string]any)
		if !ok {
			return nil, &PathCannotBeReachedError{
				Segment:     key,
				CurrentType: fmt.Sprintf("%T", d[key]),
			}
		}
		subResults, err := walkAll(subMap, rest)
		if err != nil {
			return nil, err
		}
		results = append(results, subResults...)
	}
	return results, nil
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

// GetStringOnce returns a string value at path.
func GetStringOnce(data map[string]any, path ...string) (string, error) {
	val, err := walkOnce(data, path)
	if err != nil {
		return "", err
	}
	if err := checkType(val, "string", path); err != nil {
		return "", err
	}
	return val.(string), nil
}

// GetIntegerOnce returns an integer value at path.
func GetIntegerOnce(data map[string]any, path ...string) (int, error) {
	val, err := walkOnce(data, path)
	if err != nil {
		return 0, err
	}
	if err := checkType(val, "integer", path); err != nil {
		return 0, err
	}
	return val.(int), nil
}

// GetFloatOnce returns a float value at path.
func GetFloatOnce(data map[string]any, path ...string) (float64, error) {
	val, err := walkOnce(data, path)
	if err != nil {
		return 0, err
	}
	if err := checkType(val, "float", path); err != nil {
		return 0, err
	}
	if f, ok := val.(float64); ok {
		return f, nil
	}
	return float64(val.(int)), nil
}

// GetBoolOnce returns a boolean value at path.
func GetBoolOnce(data map[string]any, path ...string) (bool, error) {
	val, err := walkOnce(data, path)
	if err != nil {
		return false, err
	}
	if err := checkType(val, "boolean", path); err != nil {
		return false, err
	}
	return val.(bool), nil
}

// GetTimestampOnce returns a timestamp (string) value at path.
func GetTimestampOnce(data map[string]any, path ...string) (string, error) {
	val, err := walkOnce(data, path)
	if err != nil {
		return "", err
	}
	if err := checkType(val, "string", path); err != nil {
		return "", err
	}
	return val.(string), nil
}

// GetListOnce returns a list value at path.
func GetListOnce(data map[string]any, path ...string) ([]any, error) {
	val, err := walkOnce(data, path)
	if err != nil {
		return nil, err
	}
	if err := checkType(val, "list", path); err != nil {
		return nil, err
	}
	return val.([]any), nil
}

// GetMappingOnce returns a mapping (dict) value at path.
func GetMappingOnce(data map[string]any, path ...string) (map[string]any, error) {
	val, err := walkOnce(data, path)
	if err != nil {
		return nil, err
	}
	if err := checkType(val, "mapping", path); err != nil {
		return nil, err
	}
	return val.(map[string]any), nil
}

// GetStructOnce returns a struct (dict) value at path.
func GetStructOnce(data map[string]any, path ...string) (map[string]any, error) {
	val, err := walkOnce(data, path)
	if err != nil {
		return nil, err
	}
	if err := checkType(val, "struct", path); err != nil {
		return nil, err
	}
	return val.(map[string]any), nil
}

// GetScopeOnce returns a scope (dict) value at path.
func GetScopeOnce(data map[string]any, path ...string) (map[string]any, error) {
	val, err := walkOnce(data, path)
	if err != nil {
		return nil, err
	}
	if err := checkType(val, "scope", path); err != nil {
		return nil, err
	}
	return val.(map[string]any), nil
}

// GetEnumOnce returns an enum (integer ordinal) value at path.
func GetEnumOnce(data map[string]any, path ...string) (int, error) {
	val, err := walkOnce(data, path)
	if err != nil {
		return 0, err
	}
	if err := checkType(val, "enum", path); err != nil {
		return 0, err
	}
	return val.(int), nil
}

// GetStringIndex returns a string at path, using index to select among iterations.
func GetStringIndex(data map[string]any, index int, path ...string) (string, error) {
	val, err := walkIndex(data, index, path)
	if err != nil {
		return "", err
	}
	if err := checkType(val, "string", path); err != nil {
		return "", err
	}
	return val.(string), nil
}

// GetIntegerIndex returns an integer at path, using index to select among iterations.
func GetIntegerIndex(data map[string]any, index int, path ...string) (int, error) {
	val, err := walkIndex(data, index, path)
	if err != nil {
		return 0, err
	}
	if err := checkType(val, "integer", path); err != nil {
		return 0, err
	}
	return val.(int), nil
}

// GetFloatIndex returns a float at path, using index to select among iterations.
func GetFloatIndex(data map[string]any, index int, path ...string) (float64, error) {
	val, err := walkIndex(data, index, path)
	if err != nil {
		return 0, err
	}
	if err := checkType(val, "float", path); err != nil {
		return 0, err
	}
	if f, ok := val.(float64); ok {
		return f, nil
	}
	return float64(val.(int)), nil
}

// GetBoolIndex returns a boolean at path, using index to select among iterations.
func GetBoolIndex(data map[string]any, index int, path ...string) (bool, error) {
	val, err := walkIndex(data, index, path)
	if err != nil {
		return false, err
	}
	if err := checkType(val, "boolean", path); err != nil {
		return false, err
	}
	return val.(bool), nil
}

// GetTimestampIndex returns a timestamp at path, using index to select among iterations.
func GetTimestampIndex(data map[string]any, index int, path ...string) (string, error) {
	val, err := walkIndex(data, index, path)
	if err != nil {
		return "", err
	}
	if err := checkType(val, "string", path); err != nil {
		return "", err
	}
	return val.(string), nil
}

// GetListIndex returns a list at path, using index to select among iterations.
func GetListIndex(data map[string]any, index int, path ...string) ([]any, error) {
	val, err := walkIndex(data, index, path)
	if err != nil {
		return nil, err
	}
	if err := checkType(val, "list", path); err != nil {
		return nil, err
	}
	return val.([]any), nil
}

// GetMappingIndex returns a mapping at path, using index to select among iterations.
func GetMappingIndex(data map[string]any, index int, path ...string) (map[string]any, error) {
	val, err := walkIndex(data, index, path)
	if err != nil {
		return nil, err
	}
	if err := checkType(val, "mapping", path); err != nil {
		return nil, err
	}
	return val.(map[string]any), nil
}

// GetStructIndex returns a struct at path, using index to select among iterations.
func GetStructIndex(data map[string]any, index int, path ...string) (map[string]any, error) {
	val, err := walkIndex(data, index, path)
	if err != nil {
		return nil, err
	}
	if err := checkType(val, "struct", path); err != nil {
		return nil, err
	}
	return val.(map[string]any), nil
}

// GetScopeIndex returns a scope at path, using index to select among iterations.
func GetScopeIndex(data map[string]any, index int, path ...string) (map[string]any, error) {
	val, err := walkIndex(data, index, path)
	if err != nil {
		return nil, err
	}
	if err := checkType(val, "scope", path); err != nil {
		return nil, err
	}
	return val.(map[string]any), nil
}

// GetEnumIndex returns an enum at path, using index to select among iterations.
func GetEnumIndex(data map[string]any, index int, path ...string) (int, error) {
	val, err := walkIndex(data, index, path)
	if err != nil {
		return 0, err
	}
	if err := checkType(val, "enum", path); err != nil {
		return 0, err
	}
	return val.(int), nil
}

// GetStringAll returns all string values reachable at path.
func GetStringAll(data map[string]any, path ...string) ([]string, error) {
	vals, err := walkAll(data, path)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(vals))
	for i, v := range vals {
		if err := checkType(v, "string", path); err != nil {
			return nil, err
		}
		result[i] = v.(string)
	}
	return result, nil
}

// GetIntegerAll returns all integer values reachable at path.
func GetIntegerAll(data map[string]any, path ...string) ([]int, error) {
	vals, err := walkAll(data, path)
	if err != nil {
		return nil, err
	}
	result := make([]int, len(vals))
	for i, v := range vals {
		if err := checkType(v, "integer", path); err != nil {
			return nil, err
		}
		result[i] = v.(int)
	}
	return result, nil
}

// GetFloatAll returns all float values reachable at path.
func GetFloatAll(data map[string]any, path ...string) ([]float64, error) {
	vals, err := walkAll(data, path)
	if err != nil {
		return nil, err
	}
	result := make([]float64, len(vals))
	for i, v := range vals {
		if err := checkType(v, "float", path); err != nil {
			return nil, err
		}
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
	vals, err := walkAll(data, path)
	if err != nil {
		return nil, err
	}
	result := make([]bool, len(vals))
	for i, v := range vals {
		if err := checkType(v, "boolean", path); err != nil {
			return nil, err
		}
		result[i] = v.(bool)
	}
	return result, nil
}

// GetTimestampAll returns all timestamp values reachable at path.
func GetTimestampAll(data map[string]any, path ...string) ([]string, error) {
	vals, err := walkAll(data, path)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(vals))
	for i, v := range vals {
		if err := checkType(v, "string", path); err != nil {
			return nil, err
		}
		result[i] = v.(string)
	}
	return result, nil
}

// GetListAll returns all list values reachable at path.
func GetListAll(data map[string]any, path ...string) ([]any, error) {
	// walkAll already returns []any, but we need to check each is a list
	vals, err := walkAll(data, path)
	if err != nil {
		return nil, err
	}
	// For list type, we return the values directly since walkAll already
	// collects them. But we need to type-check each.
	for _, v := range vals {
		if err := checkType(v, "list", path); err != nil {
			return nil, err
		}
	}
	return vals, nil
}

// GetMappingAll returns all mapping values reachable at path.
func GetMappingAll(data map[string]any, path ...string) ([]map[string]any, error) {
	vals, err := walkAll(data, path)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(vals))
	for i, v := range vals {
		if err := checkType(v, "mapping", path); err != nil {
			return nil, err
		}
		result[i] = v.(map[string]any)
	}
	return result, nil
}

// GetStructAll returns all struct values reachable at path.
func GetStructAll(data map[string]any, path ...string) ([]map[string]any, error) {
	vals, err := walkAll(data, path)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(vals))
	for i, v := range vals {
		if err := checkType(v, "struct", path); err != nil {
			return nil, err
		}
		result[i] = v.(map[string]any)
	}
	return result, nil
}

// GetScopeAll returns all scope values reachable at path.
func GetScopeAll(data map[string]any, path ...string) ([]map[string]any, error) {
	vals, err := walkAll(data, path)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(vals))
	for i, v := range vals {
		if err := checkType(v, "scope", path); err != nil {
			return nil, err
		}
		result[i] = v.(map[string]any)
	}
	return result, nil
}

// GetEnumAll returns all enum (int) values reachable at path.
func GetEnumAll(data map[string]any, path ...string) ([]int, error) {
	vals, err := walkAll(data, path)
	if err != nil {
		return nil, err
	}
	result := make([]int, len(vals))
	for i, v := range vals {
		if err := checkType(v, "enum", path); err != nil {
			return nil, err
		}
		result[i] = v.(int)
	}
	return result, nil
}
