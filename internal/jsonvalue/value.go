// Package jsonvalue projects optional application values into stable JSON null-or-value shapes.
package jsonvalue

// Optional preserves the distinction between an absent pointer and a recorded zero value.
func Optional[T any](value *T) any {
	if value == nil {
		return nil
	}
	return *value
}
