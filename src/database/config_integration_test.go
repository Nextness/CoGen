// Integration tests for database configuration and migration chains.
//go:build integration

package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBaselineAppliesCorrectly verifies that a new temporary database opens
// with the single workspace baseline, has all required tables, indexes, and
// triggers, and reopens without reapplying the baseline.
func TestBaselineAppliesCorrectly(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Verify schema_migrations has all configured migrations.
	var count int
	err := db.DB.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 21 {
		t.Fatalf("expected 21 applied metadata migrations, got %d", count)
	}

	// Verify every workspace table exists.
	expectedTables := []string{
		"pipeline_runs", "searches", "search_revisions",
		"execution_plans", "run_sources", "source_records", "artifacts",
		"run_steps", "pipeline_run_metrics", "audit_events", "works",
		"work_identifiers", "work_revisions", "run_work_stages",
		"people", "author_occurrences", "authorships", "reference_mentions",
		"cache_entries", "run_cache_uses", "artifact_blobs", "run_artifacts",
		"author_identity_resolutions", "author_identity_candidates",
	}
	for _, name := range expectedTables {
		var n int
		err := db.DB.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name,
		).Scan(&n)
		if err != nil {
			t.Fatalf("query table %s: %v", name, err)
		}
		if n != 1 {
			t.Errorf("expected table %q to exist", name)
		}
	}

	// Verify all workspace indexes exist.
	expectedIndexes := []string{
		"idx_search_revisions_search_id", "idx_execution_plans_fingerprint",
		"idx_pipeline_runs_attempt", "idx_source_records_run_source",
		"idx_source_records_hash", "idx_run_steps_artifact_in",
		"idx_run_steps_artifact_out", "idx_run_steps_input_fingerprint", "idx_audit_events_run",
		"idx_audit_events_entity", "idx_audit_events_action",
		"idx_audit_events_correlation", "idx_work_identifiers_work_id",
		"idx_work_revisions_work_id", "idx_work_revisions_run_id",
		"idx_run_work_stages_run_id", "idx_run_work_stages_work_id",
		"idx_author_occurrences_person", "idx_author_occurrences_orcid",
		"idx_authorships_revision", "idx_authorships_occurrence",
		"idx_reference_mentions_revision", "idx_reference_mentions_resolved_work",
		"idx_cache_entries_expiry", "idx_cache_entries_payload",
		"idx_run_cache_uses_run", "idx_run_cache_uses_entry", "idx_run_artifacts_artifact",
		"idx_author_identity_resolutions_run", "idx_author_identity_resolutions_occurrence",
		"idx_author_identity_candidates_resolution", "idx_author_identity_candidates_orcid",
	}
	for _, name := range expectedIndexes {
		var n int
		err := db.DB.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", name,
		).Scan(&n)
		if err != nil {
			t.Fatalf("query index %s: %v", name, err)
		}
		if n != 1 {
			t.Errorf("expected index %q to exist", name)
		}
	}

	// Verify append-only and identity-guard triggers exist.
	expectedTriggers := []string{
		"audit_events_abort_update", "audit_events_abort_delete",
		"work_revisions_abort_update", "work_revisions_abort_delete",
		"author_occurrences_abort_update", "author_occurrences_abort_delete",
		"authorships_abort_update", "authorships_abort_delete",
		"people_abort_blank_orcid_insert", "people_abort_blank_orcid_update",
		"reference_mentions_abort_update", "reference_mentions_abort_delete",
	}
	for _, name := range expectedTriggers {
		var n int
		err := db.DB.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name=?", name,
		).Scan(&n)
		if err != nil {
			t.Fatalf("query trigger %s: %v", name, err)
		}
		if n != 1 {
			t.Errorf("expected trigger %q to exist", name)
		}
	}

	// Verify producer_stage has no default (fresh baseline has no sentinel)
	var notNull bool
	var dflt interface{}
	err = db.DB.QueryRow(
		"SELECT \"notnull\", dflt_value FROM pragma_table_info('work_revisions') WHERE name='producer_stage'",
	).Scan(&notNull, &dflt)
	if err != nil {
		t.Fatal(err)
	}
	if !notNull {
		t.Error("producer_stage must be NOT NULL")
	}
	if dflt != nil {
		t.Errorf("producer_stage must have no default, got %v", dflt)
	}

	// Verify updated_at exists on run_work_stages with a SQL function default
	var colCount int
	var uaDflt string
	err = db.DB.QueryRow(
		"SELECT COUNT(*), dflt_value FROM pragma_table_info('run_work_stages') WHERE name='updated_at'",
	).Scan(&colCount, &uaDflt)
	if err != nil {
		t.Fatal(err)
	}
	if colCount != 1 {
		t.Error("expected updated_at column on run_work_stages")
	}
	if uaDflt != "datetime('now')" {
		t.Errorf("updated_at default must be datetime('now'), got %q", uaDflt)
	}

	// Verify pipeline_runs has the extended columns
	for _, col := range []string{"execution_plan_id", "attempt_number", "visibility_state", "trashed_at", "trash_reason"} {
		err = db.DB.QueryRow(
			"SELECT COUNT(*) FROM pragma_table_info('pipeline_runs') WHERE name=?", col,
		).Scan(&colCount)
		if err != nil {
			t.Fatal(err)
		}
		if colCount != 1 {
			t.Errorf("expected column %q on pipeline_runs", col)
		}
	}

	// Regression: a direct INSERT into run_work_stages must get a non-empty
	// updated_at from the datetime('now') default.
	workID, err := db.Works.CreateByDOI("10.1000/baseline-updated-at")
	if err != nil {
		t.Fatalf("CreateByDOI: %v", err)
	}
	runID, err := db.PipelineRuns.StartRun("baseline test", "q")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	_, err = db.DB.Exec(
		"INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome, reason) VALUES (?, ?, 'parse', 'parsed', 'baseline default')",
		runID, workID)
	if err != nil {
		t.Fatalf("insert stage: %v", err)
	}
	var ua string
	err = db.DB.QueryRow("SELECT updated_at FROM run_work_stages WHERE pipeline_run_id=? AND work_id=?", runID, workID).Scan(&ua)
	if err != nil {
		t.Fatal(err)
	}
	if ua == "" {
		t.Fatal("updated_at must be non-empty from datetime('now') default")
	}

	// Close and reopen to verify idempotency
	db.Close()
	db2, err := Open(db.dbPath, testConfigPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close()

	var count2 int
	err = db2.DB.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count2)
	if err != nil {
		t.Fatal(err)
	}
	if count2 != 21 {
		t.Fatalf("expected 21 migrations after reopen, got %d", count2)
	}
}

// TestFutureMigrationRollsBackAtomically verifies that a deliberately broken
// future migration (V00002) applies its schema changes atomically: if the
// migration SQL fails, the new table does not persist and the tracking row is
// not recorded.
func TestFutureMigrationRollsBackAtomically(t *testing.T) {
	tempDir := t.TempDir()
	migrationsDir := filepath.Join(tempDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(tempDir, "database.something")

	// Write the baseline migration
	baseData, err := os.ReadFile(filepath.Join("../..", "migrations", "corpus.metadata", "V00001_workspace_baseline.sql"))
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	if err := os.WriteFile(filepath.Join(migrationsDir, "V00001_workspace_baseline.sql"), baseData, 0644); err != nil {
		t.Fatal(err)
	}

	// Write a broken future migration after the baseline
	future := "-- ==UP==\nCREATE TABLE future_test (id INTEGER);\nTHIS IS NOT SQL;\n-- ==DOWN==\n"
	if err := os.WriteFile(filepath.Join(migrationsDir, "V00002_future_broken.sql"), []byte(future), 0644); err != nil {
		t.Fatal(err)
	}

	config := `
db_migration: setup = {
    filename: string;
    previous?: string = "";
    upgrade?: string = "";
}
#iteration("_db_migration"): db_migration = {
    filename = "V00001_workspace_baseline.sql",
};
#iteration("_db_migration"): db_migration = {
    filename = "V00002_future_broken.sql",
};
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	conn, err := sql.Open("sqlite", filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	db := &Database{DB: conn, migrations: migrationsDir}
	if err := db.runMigrations(configPath); err == nil {
		t.Fatal("expected broken future migration to fail")
	}

	// Verify the baseline was applied
	var count int
	if err := conn.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE filename = 'V00001_workspace_baseline.sql'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("baseline must be recorded despite future migration failure")
	}

	// Verify the broken future migration was not applied
	if err := conn.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 applied migration, got %d", count)
	}

	// Verify the future table was rolled back
	var tableCount int
	if err := conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='future_test'").Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if tableCount != 0 {
		t.Fatal("failed future migration left a partially-created table")
	}

	// Verify baseline tables are still accessible
	var articleCount int
	if err := conn.QueryRow("SELECT COUNT(*) FROM articles").Scan(&articleCount); err != nil {
		t.Fatalf("baseline table articles not accessible after failed future migration: %v", err)
	}
}

// TestV00003MigrationApplies verifies that the V00003 migration is applied
// after V00002, creating the four append-only triggers.
func TestV00003MigrationApplies(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Five migrations should be applied: V00001 through V00005.
	var count int
	err := db.DB.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 21 {
		t.Fatalf("expected 21 applied migrations, got %d", count)
	}

	// Verify the four append-only triggers exist
	expectedTriggers := []string{
		"author_occurrences_abort_update", "author_occurrences_abort_delete",
		"authorships_abort_update", "authorships_abort_delete",
	}
	for _, name := range expectedTriggers {
		var n int
		err := db.DB.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name=?", name,
		).Scan(&n)
		if err != nil {
			t.Fatalf("query trigger %s: %v", name, err)
		}
		if n != 1 {
			t.Errorf("expected trigger %q to exist", name)
		}
	}
}

// TestV00002ToV00003Upgrade verifies that an existing database with V00001 and
// V00002 upgrades correctly when V00003 is added to the config. This is a
// true upgrade regression test, distinct from TestV00003MigrationApplies which
// opens a fresh database with all three migrations already configured.
func TestV00002ToV00003Upgrade(t *testing.T) {
	tempDir := t.TempDir()
	migrationsDir := filepath.Join(tempDir, "migrations")
	configDir := filepath.Join(tempDir, "config")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Copy V00001 and V00002 into the temp migrations dir
	baseData, err := os.ReadFile(filepath.Join("..", "..", "migrations", "corpus.metadata", "V00001_workspace_baseline.sql"))
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	if err := os.WriteFile(filepath.Join(migrationsDir, "V00001_workspace_baseline.sql"), baseData, 0644); err != nil {
		t.Fatal(err)
	}
	v2Data, err := os.ReadFile(filepath.Join("..", "..", "migrations", "corpus.metadata", "V00002_author_authorship.sql"))
	if err != nil {
		t.Fatalf("read V00002: %v", err)
	}
	if err := os.WriteFile(filepath.Join(migrationsDir, "V00002_author_authorship.sql"), v2Data, 0644); err != nil {
		t.Fatal(err)
	}

	// Config with only V00001 and V00002
	configV1V2 := `
db_migration: setup = {
    filename: string;
    previous?: string = "";
    upgrade?: string = "";
}
#iteration("_db_migration"): db_migration = {
    filename = "V00001_workspace_baseline.sql",
    upgrade  = "V00002_author_authorship.sql",
};
#iteration("_db_migration"): db_migration = {
    filename = "V00002_author_authorship.sql",
    previous = "V00001_workspace_baseline.sql",
};
`
	configPath := filepath.Join(configDir, "database.something")
	if err := os.WriteFile(configPath, []byte(configV1V2), 0644); err != nil {
		t.Fatal(err)
	}

	// Open resolves migrations as filepath.Dir(configPath)/../migrations,
	// so config at configDir/database.something → parent configDir/ → ../migrations = tempDir/migrations/
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := Open(dbPath, configPath)
	if err != nil {
		t.Fatalf("Open (V1+V2): %v", err)
	}

	// Verify V00003 triggers do NOT exist yet
	for _, name := range []string{
		"author_occurrences_abort_update", "author_occurrences_abort_delete",
		"authorships_abort_update", "authorships_abort_delete",
	} {
		var n int
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name=?", name).Scan(&n); err != nil {
			t.Fatalf("query trigger %s: %v", name, err)
		}
		if n != 0 {
			t.Errorf("V00003 trigger %q should not exist before upgrade, found %d", name, n)
		}
	}

	db.Close()

	// Now add V00003 to the config and reopen
	configV1V2V3 := `
db_migration: setup = {
    filename: string;
    previous?: string = "";
    upgrade?: string = "";
}
#iteration("_db_migration"): db_migration = {
    filename = "V00001_workspace_baseline.sql",
    upgrade  = "V00002_author_authorship.sql",
};
#iteration("_db_migration"): db_migration = {
    filename = "V00002_author_authorship.sql",
    previous = "V00001_workspace_baseline.sql",
    upgrade  = "V00003_append_only_authors.sql",
};
#iteration("_db_migration"): db_migration = {
    filename = "V00003_append_only_authors.sql",
    previous = "V00002_author_authorship.sql",
};
`
	if err := os.WriteFile(configPath, []byte(configV1V2V3), 0644); err != nil {
		t.Fatal(err)
	}

	// Copy V00003 into the temp migrations dir
	v3Data, err := os.ReadFile(filepath.Join("..", "..", "migrations", "corpus.metadata", "V00003_append_only_authors.sql"))
	if err != nil {
		t.Fatalf("read V00003: %v", err)
	}
	if err := os.WriteFile(filepath.Join(migrationsDir, "V00003_append_only_authors.sql"), v3Data, 0644); err != nil {
		t.Fatal(err)
	}

	db2, err := Open(dbPath, configPath)
	if err != nil {
		t.Fatalf("Open (V1+V2+V3): %v", err)
	}
	defer db2.Close()

	// Verify three migrations are now applied
	var count int
	if err := db2.DB.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected 3 migrations after upgrade, got %d", count)
	}

	// Verify all four V00003 triggers now exist
	expectedTriggers := []string{
		"author_occurrences_abort_update", "author_occurrences_abort_delete",
		"authorships_abort_update", "authorships_abort_delete",
	}
	for _, name := range expectedTriggers {
		var n int
		if err := db2.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name=?", name).Scan(&n); err != nil {
			t.Fatalf("query trigger %s: %v", name, err)
		}
		if n != 1 {
			t.Errorf("expected trigger %q to exist after upgrade", name)
		}
	}
}

// TestV00003ToV00004Upgrade verifies that an existing V00003 database gains
// the non-blank people.orcid guards when V00004 is added to the config.
func TestV00003ToV00004Upgrade(t *testing.T) {
	tempDir := t.TempDir()
	migrationsDir := filepath.Join(tempDir, "migrations")
	configDir := filepath.Join(tempDir, "config")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	for _, filename := range []string{
		"V00001_workspace_baseline.sql",
		"V00002_author_authorship.sql",
		"V00003_append_only_authors.sql",
	} {
		data, err := os.ReadFile(filepath.Join("..", "..", "migrations", "corpus.metadata", filename))
		if err != nil {
			t.Fatalf("read %s: %v", filename, err)
		}
		if err := os.WriteFile(filepath.Join(migrationsDir, filename), data, 0644); err != nil {
			t.Fatalf("write %s: %v", filename, err)
		}
	}

	configPath := filepath.Join(configDir, "database.something")
	configV1V2V3 := `
db_migration: setup = {
    filename: string;
    previous?: string = "";
    upgrade?: string = "";
}
#iteration("_db_migration"): db_migration = {
    filename = "V00001_workspace_baseline.sql",
    upgrade  = "V00002_author_authorship.sql",
};
#iteration("_db_migration"): db_migration = {
    filename = "V00002_author_authorship.sql",
    previous = "V00001_workspace_baseline.sql",
    upgrade  = "V00003_append_only_authors.sql",
};
#iteration("_db_migration"): db_migration = {
    filename = "V00003_append_only_authors.sql",
    previous = "V00002_author_authorship.sql",
};
`
	if err := os.WriteFile(configPath, []byte(configV1V2V3), 0644); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := Open(dbPath, configPath)
	if err != nil {
		t.Fatalf("Open (V1+V2+V3): %v", err)
	}

	for _, name := range []string{"people_abort_blank_orcid_insert", "people_abort_blank_orcid_update"} {
		var n int
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name=?", name).Scan(&n); err != nil {
			t.Fatalf("query trigger %s: %v", name, err)
		}
		if n != 0 {
			t.Errorf("V00004 trigger %q should not exist before upgrade", name)
		}
	}
	db.Close()

	v4Data, err := os.ReadFile(filepath.Join("..", "..", "migrations", "corpus.metadata", "V00004_people_identity_guards.sql"))
	if err != nil {
		t.Fatalf("read V00004: %v", err)
	}
	if err := os.WriteFile(filepath.Join(migrationsDir, "V00004_people_identity_guards.sql"), v4Data, 0644); err != nil {
		t.Fatal(err)
	}
	configV1V2V3V4 := configV1V2V3[:len(configV1V2V3)-1] + `
#iteration("_db_migration"): db_migration = {
    filename = "V00004_people_identity_guards.sql",
    previous = "V00003_append_only_authors.sql",
};
`
	if err := os.WriteFile(configPath, []byte(configV1V2V3V4), 0644); err != nil {
		t.Fatal(err)
	}

	db, err = Open(dbPath, configPath)
	if err != nil {
		t.Fatalf("Open (V1+V2+V3+V4): %v", err)
	}
	defer db.Close()

	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("expected 4 migrations after upgrade, got %d", count)
	}
	for _, name := range []string{"people_abort_blank_orcid_insert", "people_abort_blank_orcid_update"} {
		var n int
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name=?", name).Scan(&n); err != nil {
			t.Fatalf("query trigger %s: %v", name, err)
		}
		if n != 1 {
			t.Errorf("expected trigger %q after upgrade", name)
		}
	}

	_, err = db.DB.Exec("INSERT INTO people (orcid) VALUES ('   ')")
	if err == nil || !strings.Contains(err.Error(), "people.orcid must not be null, empty, or whitespace") {
		t.Fatalf("expected non-blank ORCID guard after upgrade, got: %v", err)
	}
}

// TestV00002MigrationApplies verifies that the V00002 migration is applied
// after the baseline, creating the people, author_occurrences, and
// authorships tables with their indexes.
func TestV00002MigrationApplies(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Five migrations should be applied: V00001 through V00005.
	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 21 {
		t.Fatalf("expected 21 applied metadata migrations, got %d", count)
	}

	// Verify the new tables exist
	expectedTables := []string{"people", "author_occurrences", "authorships"}
	for _, name := range expectedTables {
		var n int
		err := db.DB.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name,
		).Scan(&n)
		if err != nil {
			t.Fatalf("query table %s: %v", name, err)
		}
		if n != 1 {
			t.Errorf("expected table %q to exist", name)
		}
	}

	// Verify the new indexes exist
	expectedIndexes := []string{
		"idx_author_occurrences_person", "idx_author_occurrences_orcid",
		"idx_authorships_revision", "idx_authorships_occurrence",
	}
	for _, name := range expectedIndexes {
		var n int
		err := db.DB.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", name,
		).Scan(&n)
		if err != nil {
			t.Fatalf("query index %s: %v", name, err)
		}
		if n != 1 {
			t.Errorf("expected index %q to exist", name)
		}
	}
}

// registry_test.go tests the production database registry to verify
// that each store kind resolves its own independent migration chain
// correctly.

// TestProductionDatabaseRegistryResolvesIndependentMigrationChains verifies production database registry resolves independent migration chains.
func TestProductionDatabaseRegistryResolvesIndependentMigrationChains(t *testing.T) {
	registry := filepath.Join("..", "..", "config", "database.something")
	metadataConfig, err := ResolveMigrationConfig(registry, StoreCorpusMetadata)
	if err != nil {
		t.Fatal(err)
	}
	pdfConfig, err := ResolveMigrationConfig(registry, StoreCorpusPDF)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(metadataConfig.ConfigPath) != "database.corpus.metadata.something" || filepath.Base(metadataConfig.MigrationsDir) != "corpus.metadata" {
		t.Fatalf("metadata migration configuration resolved incorrectly: %+v", metadataConfig)
	}
	if filepath.Base(pdfConfig.ConfigPath) != "database.corpus.pdf.something" || filepath.Base(pdfConfig.MigrationsDir) != "corpus.pdf" {
		t.Fatalf("PDF migration configuration resolved incorrectly: %+v", pdfConfig)
	}
}

// TestProductionRegistryCreatesMetadataAndPDFStores verifies production registry creates metadata and pdf stores.
func TestProductionRegistryCreatesMetadataAndPDFStores(t *testing.T) {
	registry := filepath.Join("..", "..", "config", "database.something")
	metadata, err := Open(filepath.Join(t.TempDir(), "corpus.metadata.db"), registry)
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	var metadataMigrations int
	if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&metadataMigrations); err != nil {
		t.Fatal(err)
	}
	if metadataMigrations != 21 {
		t.Fatalf("metadata migrations = %d, want 21", metadataMigrations)
	}
	for _, table := range []string{"pdf_store_binding", "pdf_audit_links"} {
		var found int
		if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&found); err != nil || found != 1 {
			t.Fatalf("metadata table %s: found=%d err=%v", table, found, err)
		}
	}

	pdf, err := OpenConfigured(filepath.Join(t.TempDir(), "corpus.pdf.db"), registry, StoreCorpusPDF)
	if err != nil {
		t.Fatal(err)
	}
	defer pdf.Close()
	var pdfMigrations int
	if err := pdf.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&pdfMigrations); err != nil {
		t.Fatal(err)
	}
	if pdfMigrations != 2 {
		t.Fatalf("PDF migrations = %d, want 2", pdfMigrations)
	}
	for _, table := range []string{"pdf_blobs", "pdf_documents", "pdf_audit_outbox"} {
		var found int
		if err := pdf.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&found); err != nil || found != 1 {
			t.Fatalf("PDF table %s: found=%d err=%v", table, found, err)
		}
	}
	for _, column := range []string{"status", "content_hash", "inventoried_at", "updated_at"} {
		var found int
		if err := pdf.QueryRow("SELECT COUNT(*) FROM pragma_table_info('pdf_documents') WHERE name=?", column).Scan(&found); err != nil || found != 1 {
			t.Fatalf("PDF inventory column %s: found=%d err=%v", column, found, err)
		}
	}
}

// TestPDFInventoryMigrationPreservesLegacyDocumentStates verifies pdf inventory migration preserves legacy document states.
func TestPDFInventoryMigrationPreservesLegacyDocumentStates(t *testing.T) {
	registry := filepath.Join("..", "..", "config", "database.something")
	path := filepath.Join(t.TempDir(), "corpus.pdf.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	v1Path := filepath.Join("..", "..", "migrations", "corpus.pdf", "V00001_pdf_store_baseline.sql")
	v1, err := extractUpSQL(v1Path)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec(v1); err != nil {
		db.Close()
		t.Fatal(err)
	}
	checksum, err := fileChecksum(v1Path)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (
		filename TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT (datetime('now')), checksum TEXT NOT NULL);
		INSERT INTO schema_migrations (filename, checksum) VALUES ('V00001_pdf_store_baseline.sql', ?);
		INSERT INTO pdf_blobs (content_hash, byte_size, data, created_at)
		VALUES ('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 7, '%PDF-ok', '2026-01-01T00:00:00Z');
		INSERT INTO pdf_documents (doi, status, content_hash, source, downloaded_at, updated_at)
		VALUES ('10.1000/available', 'downloaded', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'manual', '2026-01-02T00:00:00Z', '2026-01-02T00:00:00Z');
		INSERT INTO pdf_documents (doi, status, error_class, updated_at)
		VALUES ('10.1000/missing', 'failed', 'network', '2026-01-03T00:00:00Z')`, checksum); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := OpenConfigured(path, registry, StoreCorpusPDF)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	rows, err := upgraded.Query("SELECT doi, status, content_hash, inventoried_at FROM pdf_documents ORDER BY doi")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var states []struct {
		doi, status string
		hash, at    sql.NullString
	}
	for rows.Next() {
		var state struct {
			doi, status string
			hash, at    sql.NullString
		}
		if err := rows.Scan(&state.doi, &state.status, &state.hash, &state.at); err != nil {
			t.Fatal(err)
		}
		states = append(states, state)
	}
	if len(states) != 2 || states[0].status != "available" || !states[0].hash.Valid || !states[0].at.Valid ||
		states[1].status != "not_available" || states[1].hash.Valid || states[1].at.Valid {
		t.Fatalf("migrated PDF inventory states = %#v", states)
	}
}

// TestProductionMetadataUpgradePreservesAppliedBasenames verifies production metadata upgrade preserves applied basenames.
func TestProductionMetadataUpgradePreservesAppliedBasenames(t *testing.T) {
	registry := filepath.Join("..", "..", "config", "database.something")
	databasePath := filepath.Join(t.TempDir(), "corpus.metadata.db")
	metadata, err := Open(databasePath, registry)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := metadata.DB.Exec(`DROP TABLE pdf_audit_links;
		DROP TABLE pdf_store_binding;
		DELETE FROM schema_migrations WHERE filename IN
		('V00019_pdf_store_binding.sql', 'V00020_pdf_gather_audit_links.sql', 'V00021_rename_pdf_audit_links.sql')`); err != nil {
		metadata.Close()
		t.Fatal(err)
	}
	if err := metadata.Close(); err != nil {
		t.Fatal(err)
	}

	metadata, err = Open(databasePath, registry)
	if err != nil {
		t.Fatal(err)
	}
	defer metadata.Close()
	var total, historical, additions int
	if err := metadata.DB.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&total); err != nil {
		t.Fatal(err)
	}
	if err := metadata.DB.QueryRow(`SELECT COUNT(*) FROM schema_migrations
		WHERE filename='V00018_source_filter_counts.sql'`).Scan(&historical); err != nil {
		t.Fatal(err)
	}
	if err := metadata.DB.QueryRow(`SELECT COUNT(*) FROM schema_migrations
		WHERE filename IN ('V00019_pdf_store_binding.sql', 'V00020_pdf_gather_audit_links.sql',
		'V00021_rename_pdf_audit_links.sql')`).Scan(&additions); err != nil {
		t.Fatal(err)
	}
	if total != 21 || historical != 1 || additions != 3 {
		t.Fatalf("migration history after V00018 upgrade: total=%d historical=%d additions=%d", total, historical, additions)
	}
}
