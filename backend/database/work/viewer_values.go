// viewer_values.go preserves optional metadata in viewer projections while
// pipeline read models retain their established zero-value behavior.
package work

import "database/sql"

// textPointer preserves SQL text presence, including a present empty string.
func textPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

// numberPointer preserves SQL integer presence, including a present zero.
func numberPointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	return &value.Int64
}

// rowText preserves optional text returned by a dynamic viewer query.
func rowText(value any) *string {
	if value == nil {
		return nil
	}
	text := stringValue(value)
	return &text
}

// rowNumber preserves optional integers returned by a dynamic viewer query.
func rowNumber(value any) *int64 {
	if value == nil {
		return nil
	}
	number := int64Value(value)
	return &number
}
