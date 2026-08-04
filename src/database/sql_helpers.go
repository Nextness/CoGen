// sql_helpers.go provides shared nullable-value helpers (nullStr, nullInt)
// and JSON marshalling utilities used consistently across all database
// repositories.
package database

// nullStr represents optional text consistently across workspace repositories.
func nullStr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// nullInt represents optional integer values consistently across workspace repositories.
func nullInt(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}
