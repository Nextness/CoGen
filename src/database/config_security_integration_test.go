// config_security_integration_test.go verifies registry containment checks.
//go:build integration

package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveMigrationConfigRejectsEscapingPaths verifies configuration and migration directories stay inside their configured roots.
func TestResolveMigrationConfigRejectsEscapingPaths(t *testing.T) {
	bundle := t.TempDir()
	configDir := filepath.Join(bundle, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(configDir, "database.something")
	registry := `databases_config: setup = {
    corpus_metadata_config: string;
    corpus_pdf_config: string;
}
databases: databases_config = {
    corpus_metadata_config = "../outside.something",
    corpus_pdf_config = "database.something",
};`
	if err := os.WriteFile(registryPath, []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveMigrationConfig(registryPath, StoreCorpusMetadata); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("registry lexical escape error = %v", err)
	}

	registry = `databases_config: setup = {
    corpus_metadata_config: string;
    corpus_pdf_config: string;
}
databases: databases_config = {
    corpus_metadata_config = "database.corpus.something",
    corpus_pdf_config = "database.corpus.something",
};`
	if err := os.WriteFile(registryPath, []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	specific := `database_migration_config: setup = {
    migrations_dir: string;
}
database_migrations: database_migration_config = {
    migrations_dir = "../../outside",
};
db_migration: setup = {
    filename: string;
}
#iteration("_db_migration"): db_migration = { filename = "V00001_test.sql", };`
	if err := os.WriteFile(filepath.Join(configDir, "database.corpus.something"), []byte(specific), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveMigrationConfig(registryPath, StoreCorpusMetadata); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("migration lexical escape error = %v", err)
	}
}

// TestResolveMigrationConfigRejectsSymbolicLinkEscape verifies resolved paths cannot leave their roots through symbolic links.
func TestResolveMigrationConfigRejectsSymbolicLinkEscape(t *testing.T) {
	bundle := t.TempDir()
	configDir := filepath.Join(bundle, "config")
	outside := t.TempDir()
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(configDir, "database.something")
	registry := `databases_config: setup = {
    corpus_metadata_config: string;
    corpus_pdf_config: string;
}
databases: databases_config = {
    corpus_metadata_config = "linked.something",
    corpus_pdf_config = "linked.something",
};`
	if err := os.WriteFile(registryPath, []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	specificPath := filepath.Join(outside, "database.corpus.something")
	if err := os.WriteFile(specificPath, []byte("database_migration_config: setup = {\n    migrations_dir: string;\n}\ndatabase_migrations: database_migration_config = {\n    migrations_dir = \"migrations\",\n};"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(specificPath, filepath.Join(configDir, "linked.something")); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	if _, err := ResolveMigrationConfig(registryPath, StoreCorpusMetadata); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("registry symbolic-link escape error = %v", err)
	}
}
