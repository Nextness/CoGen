// sql_helpers.go provides shared nullable-value and timestamp helpers used
// consistently across all database repositories.
package database

import "time"

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

// timestamp returns the current UTC time in the repository's persisted format.
func timestamp() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}
