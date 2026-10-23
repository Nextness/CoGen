// migrations.go reports the metadata schema version through its platform connection.
package database

import (
	"database/sql"
	"fmt"
)

// SchemaVersion returns the most recently applied migration filename. It is
// recorded in each resolved manifest so plan fingerprints describe the schema
// that interpreted the input.
func (d *Database) SchemaVersion() (string, error) {
	var version sql.NullString
	err := d.db.QueryRow("SELECT filename FROM schema_migrations ORDER BY rowid DESC LIMIT 1").Scan(&version)
	if err == sql.ErrNoRows || !version.Valid {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get schema version: %w", err)
	}
	return version.String, nil
}
