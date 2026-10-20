// test_access.go exposes the wrapped connection to test builds only. It is
// compiled for the repository's integration and end-to-end tag sets so tests
// can keep exercising low-level database behavior without reopening production
// access to the raw connection.
//go:build integration || e2e

package database

import "database/sql"

// RawDBForTest returns the wrapped connection for tests that must bypass the
// family data-access layers. Production code must use the family stores or the
// platform methods instead.
func (d *Database) RawDBForTest() *sql.DB { return d.db }
