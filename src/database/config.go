// config.go provides the database registry that resolves independent
// migration chains (corpus metadata and PDF) from the SOMETHING
// database configuration file.
package database

import (
	"fmt"
	"path/filepath"
	"strings"

	"analysis/something"
)

// StoreKind identifies one database in the corpus bundle registry.
type StoreKind string

const (
	StoreCorpusMetadata StoreKind = "corpus_metadata"
	StoreCorpusPDF      StoreKind = "corpus_pdf"
)

// MigrationConfig is one fully resolved database-specific migration source.
// ConfigPath and MigrationsDir are absolute so callers do not depend on their
// current working directory after configuration has been loaded.
type MigrationConfig struct {
	ConfigPath    string
	MigrationsDir string
}

// ResolveMigrationConfig loads the database registry and resolves the
// database-specific configuration and migration directory for kind.
func ResolveMigrationConfig(registryPath string, kind StoreKind) (MigrationConfig, error) {
	absRegistry, err := filepath.Abs(registryPath)
	if err != nil {
		return MigrationConfig{}, fmt.Errorf("resolve database registry path: %w", err)
	}
	cfg, err := loadSomethingConfig(absRegistry)
	if err != nil {
		return MigrationConfig{}, err
	}

	registry, registryErr := something.GetStructOnce(cfg, "databases")
	if registryErr != nil {
		// Preserve support for focused migration tests and callers that pass a
		// database-specific configuration directly. Production uses the
		// registry and both production database configs declare migrations_dir.
		if kind != StoreCorpusMetadata {
			return MigrationConfig{}, fmt.Errorf("database registry is missing databases: %w", registryErr)
		}
		entries, entriesErr := getMigrationStructs(cfg)
		if entriesErr != nil || len(entries) == 0 {
			return MigrationConfig{}, fmt.Errorf("database registry is missing databases: %w", registryErr)
		}
		return resolveSpecificMigrationConfig(absRegistry, cfg, true)
	}

	var field string
	switch kind {
	case StoreCorpusMetadata:
		field = "corpus_metadata_config"
	case StoreCorpusPDF:
		field = "corpus_pdf_config"
	default:
		return MigrationConfig{}, fmt.Errorf("unknown database store kind %q", kind)
	}
	configName, ok := registry[field].(string)
	if !ok || configName == "" {
		return MigrationConfig{}, fmt.Errorf("databases.%s must be a non-empty string", field)
	}
	if filepath.IsAbs(configName) {
		return MigrationConfig{}, fmt.Errorf("databases.%s must be relative to the registry", field)
	}
	configPath := filepath.Clean(filepath.Join(filepath.Dir(absRegistry), configName))
	specific, err := loadSomethingConfig(configPath)
	if err != nil {
		return MigrationConfig{}, err
	}
	return resolveSpecificMigrationConfig(configPath, specific, false)
}

// resolveSpecificMigrationConfig resolves specific migration config from the supplied context.
func resolveSpecificMigrationConfig(configPath string, cfg map[string]any, legacy bool) (MigrationConfig, error) {
	migrationsDir := filepath.Join(filepath.Dir(configPath), "..", "migrations")
	if !legacy {
		settings, err := something.GetStructOnce(cfg, "database_migrations")
		if err != nil {
			return MigrationConfig{}, fmt.Errorf("get database_migrations configuration: %w", err)
		}
		rawDir, ok := settings["migrations_dir"].(string)
		if !ok || rawDir == "" {
			return MigrationConfig{}, fmt.Errorf("database_migrations.migrations_dir must be a non-empty string")
		}
		if filepath.IsAbs(rawDir) {
			return MigrationConfig{}, fmt.Errorf("database_migrations.migrations_dir must be relative to its config")
		}
		migrationsDir = filepath.Join(filepath.Dir(configPath), rawDir)
	}
	absMigrations, err := filepath.Abs(filepath.Clean(migrationsDir))
	if err != nil {
		return MigrationConfig{}, fmt.Errorf("resolve migrations directory: %w", err)
	}
	return MigrationConfig{ConfigPath: configPath, MigrationsDir: absMigrations}, nil
}

// loadSomethingConfig loads a .something file using the existing parser.
func loadSomethingConfig(path string) (map[string]any, error) {
	cfg, err := something.LoadSomethingFile(path)
	if err != nil {
		lg.Debug("database configuration load failed", "path", path, "error", err)
		return nil, fmt.Errorf("load something config %s: %w", path, err)
	}
	lg.Debug("database configuration load successful", "path", path, "entries", len(cfg))
	return cfg, nil
}

// getMigrationStructs reads #iteration("_db_migration") entries from the config.
// Returns them in iteration-counter order (as they appear in the file).
func getMigrationStructs(cfg map[string]any) ([]migrationEntry, error) {
	structs, err := something.GetStructAll(cfg, "[iteration]_db_migration")
	if err != nil {
		lg.Debug("migration configuration query failed", "error", err)
		return nil, fmt.Errorf("get migration structs: %w", err)
	}

	var entries []migrationEntry
	for _, s := range structs {
		e := migrationEntry{}

		if v, ok := s["filename"]; ok {
			e.filename, _ = v.(string)
		}
		if v, ok := s["previous"]; ok {
			e.previous, _ = v.(string)
		}
		if v, ok := s["upgrade"]; ok {
			e.upgrade, _ = v.(string)
		}
		if raw, ok := s["supersedes"].([]any); ok {
			for _, value := range raw {
				filename, ok := value.(string)
				if !ok || !validMigrationFilename(filename) {
					return nil, fmt.Errorf("migration %s supersedes must contain filenames in V00001_description.sql form", e.filename)
				}
				if filename == e.filename {
					return nil, fmt.Errorf("migration %s cannot supersede itself", e.filename)
				}
				e.supersedes = append(e.supersedes, filename)
			}
		}

		if e.filename == "" {
			lg.Debug("migration configuration entry validation failed", "reason", "missing_filename")
			continue
		}
		entries = append(entries, e)
	}
	lg.Debug("migration configuration query successful", "entries", len(entries))
	return entries, nil
}

// validMigrationFilename reports whether filename follows the configured VNNNNN_description.sql migration identity form.
func validMigrationFilename(filename string) bool {
	if len(filename) < len("V00001_a.sql") || filename[0] != 'V' || filename[6] != '_' || !strings.HasSuffix(filename, ".sql") {
		return false
	}
	for _, character := range filename[1:6] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return len(strings.TrimSuffix(filename[7:], ".sql")) > 0
}
