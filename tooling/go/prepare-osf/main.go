// Command prepare-osf creates a sanitized, self-contained copy of a corpus bundle.
package prepareosf

import (
	"analysis/internal/filehash"
	"analysis/tooling/go/configredaction"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"analysis/backend/workspace"
	"analysis/internal/pathpolicy"
	"analysis/internal/sqliteuri"

	_ "modernc.org/sqlite"
)

// options contains source and new-output paths accepted by the export command.
type options struct{ DB, Config, Out string }

// hashMapping records a reviewer-bearing raw configuration hash and its sanitized replacement.
type hashMapping struct {
	Original  string `json:"original"`
	Sanitized string `json:"sanitized"`
}

// exportManifest records schema identity, copied file hashes, sanitization mappings, and limitations.
type exportManifest struct {
	CreatedAt               string            `json:"created_at"`
	SourceSchemaVersions    map[string]string `json:"source_schema_versions"`
	SanitizedSchemaVersions map[string]string `json:"sanitized_schema_versions"`
	Files                   map[string]string `json:"files"`
	ConfigurationMappings   []hashMapping     `json:"configuration_hash_mappings"`
	BrowserDraftDisclaimer  string            `json:"browser_draft_disclaimer"`
}

// Run parses the copy-only export command and reports failures without publishing partial output.
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("prepare-osf", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var input options
	flags.StringVar(&input.DB, "db", "", "existing metadata database")
	flags.StringVar(&input.Config, "config", "", "optional SOMETHING configuration to sanitize and include")
	flags.StringVar(&input.Out, "out", "", "new output directory")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if err := prepare(context.Background(), input); err != nil {
		fmt.Fprintln(stderr, "prepare-osf:", err)
		return 1
	}
	return 0
}

// prepare validates source ownership, sanitizes temporary snapshots, and atomically publishes a new bundle.
func prepare(ctx context.Context, input options) error {
	if strings.TrimSpace(input.DB) == "" || strings.TrimSpace(input.Out) == "" {
		return fmt.Errorf("--db and --out are required")
	}
	dbPath, err := existingFile(input.DB)
	if err != nil {
		return fmt.Errorf("metadata database: %w", err)
	}
	outPath, err := filepath.Abs(input.Out)
	if err != nil {
		return fmt.Errorf("resolve output: %w", err)
	}
	if _, err := os.Stat(outPath); err == nil {
		return fmt.Errorf("output already exists: %s", outPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect output: %w", err)
	}
	configPath := ""
	if input.Config != "" {
		configPath, err = existingFile(input.Config)
		if err != nil {
			return fmt.Errorf("configuration: %w", err)
		}
	}
	for _, source := range []string{dbPath, configPath} {
		if source != "" && samePath(source, outPath) {
			return fmt.Errorf("output aliases input %s", source)
		}
	}

	source, err := sql.Open("sqlite", sqliteReadURI(dbPath))
	if err != nil {
		return err
	}
	defer source.Close()
	sourceMetadataVersion, err := schemaVersionDB(ctx, source)
	if err != nil {
		return fmt.Errorf("read source metadata schema version: %w", err)
	}
	relativePDF, err := newExportStore(source).binding(ctx)
	if err != nil {
		return err
	}
	pdfPath, err := safeCompanionPath(dbPath, relativePDF)
	if err != nil {
		return err
	}
	if _, err := existingFile(pdfPath); err != nil {
		return fmt.Errorf("bound PDF database: %w", err)
	}
	if samePath(pdfPath, outPath) {
		return fmt.Errorf("output aliases bound PDF database")
	}
	sourcePDFVersion, err := schemaVersion(ctx, pdfPath)
	if err != nil {
		return fmt.Errorf("read source PDF schema version: %w", err)
	}

	parent := filepath.Dir(outPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create output parent: %w", err)
	}
	temporary, err := os.MkdirTemp(parent, "."+filepath.Base(outPath)+".tmp-")
	if err != nil {
		return fmt.Errorf("create temporary export: %w", err)
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(temporary)
		}
	}()

	metadataCopy := filepath.Join(temporary, filepath.Base(dbPath))
	if err := snapshotSQLite(ctx, source, metadataCopy); err != nil {
		return fmt.Errorf("snapshot metadata database: %w", err)
	}
	pdfSource, err := sql.Open("sqlite", sqliteReadURI(pdfPath))
	if err != nil {
		return fmt.Errorf("open bound PDF database: %w", err)
	}
	pdfCopy := filepath.Join(temporary, filepath.Clean(relativePDF))
	if err := os.MkdirAll(filepath.Dir(pdfCopy), 0o755); err != nil {
		pdfSource.Close()
		return err
	}
	if err := snapshotSQLite(ctx, pdfSource, pdfCopy); err != nil {
		pdfSource.Close()
		return fmt.Errorf("snapshot PDF database: %w", err)
	}
	if err := pdfSource.Close(); err != nil {
		return err
	}

	var copiedConfig string
	if configPath != "" {
		copiedConfig, err = configredaction.Copy(configPath, temporary)
		if err != nil {
			return err
		}
		loaded, err := workspace.Load(copiedConfig)
		if err != nil {
			return fmt.Errorf("evaluate sanitized configuration: %w", err)
		}
		for _, run := range loaded.Runs {
			if run.Reviewer.Username != "" || run.Reviewer.Email != "" {
				return fmt.Errorf("sanitized configuration still evaluates to a nonempty reviewer")
			}
		}
	}

	copyDB, err := sql.Open("sqlite", sqliteWritableURI(metadataCopy))
	if err != nil {
		return err
	}
	mappings, metadataVersion, err := sanitizeMetadata(ctx, copyDB)
	if err != nil {
		copyDB.Close()
		return err
	}
	if err := copyDB.Close(); err != nil {
		return err
	}
	pdfVersion, err := schemaVersion(ctx, pdfCopy)
	if err != nil {
		return err
	}
	files, err := exportedFileHashes(temporary)
	if err != nil {
		return err
	}
	manifest := exportManifest{
		CreatedAt:               time.Now().UTC().Format(time.RFC3339Nano),
		SourceSchemaVersions:    map[string]string{"metadata": sourceMetadataVersion, "pdf": sourcePDFVersion},
		SanitizedSchemaVersions: map[string]string{"metadata": metadataVersion, "pdf": pdfVersion},
		Files:                   files, ConfigurationMappings: mappings,
		BrowserDraftDisclaimer: "Browser-local drafts and arbitrary note or provider text were not scanned or copied by this export.",
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(filepath.Join(temporary, "export-manifest.json"), encoded, 0o644); err != nil {
		return err
	}
	if err := os.Rename(temporary, outPath); err != nil {
		return fmt.Errorf("publish export: %w", err)
	}
	published = true
	return nil
}

// sanitizeMetadata redacts reviewer identity and rewires copied content-addressed configuration artifacts.
func sanitizeMetadata(ctx context.Context, db *sql.DB) ([]hashMapping, string, error) {
	if _, err := db.ExecContext(ctx, "PRAGMA secure_delete=ON"); err != nil {
		return nil, "", fmt.Errorf("enable secure deletion in export copy: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()
	store := newExportStore(db).withTx(tx)
	if err := store.redactReviewers(ctx); err != nil {
		return nil, "", err
	}
	if err := store.regenerateCorpusID(ctx); err != nil {
		return nil, "", err
	}
	items, err := store.workspaceConfigArtifacts(ctx)
	if err != nil {
		return nil, "", err
	}
	mappings := []hashMapping{}
	for _, item := range items {
		sanitized, changed, err := configredaction.Redact(item.Data)
		if err != nil {
			return nil, "", fmt.Errorf("sanitize workspace configuration artifact %s: %w", item.ContentHash, err)
		}
		if !changed || bytes.Equal(sanitized, item.Data) {
			continue
		}
		digest := sha256.Sum256(sanitized)
		newHash := hex.EncodeToString(digest[:])
		newID, found, err := store.artifactByHash(ctx, newHash)
		if err != nil {
			return nil, "", err
		}
		if !found {
			newID, err = store.insertArtifact(ctx, newHash, int64(len(sanitized)), item.ContentType)
			if err != nil {
				return nil, "", err
			}
			if err := store.insertArtifactBlob(ctx, newID, item.PipelineRunID, sanitized); err != nil {
				return nil, "", err
			}
		}
		if err := store.rewireRunArtifacts(ctx, newID, item.ID); err != nil {
			return nil, "", err
		}
		if err := store.rewireSearchRevisions(ctx, newHash, item.ContentHash); err != nil {
			return nil, "", err
		}
		if err := store.rewireRunStepInputs(ctx, newID, newHash, item.ID); err != nil {
			return nil, "", err
		}
		if err := store.rewireRunStepOutputs(ctx, newID, newHash, item.ID); err != nil {
			return nil, "", err
		}
		references, err := store.artifactReferenceCount(ctx, item.ID)
		if err != nil {
			return nil, "", err
		}
		if references != 0 {
			return nil, "", fmt.Errorf("original reviewer configuration artifact remains referenced")
		}
		if err := store.deleteArtifactBlob(ctx, item.ID); err != nil {
			return nil, "", err
		}
		if err := store.deleteArtifact(ctx, item.ID); err != nil {
			return nil, "", err
		}
		mappings = append(mappings, hashMapping{Original: item.ContentHash, Sanitized: newHash})
	}
	var foreignKeyFailures int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&foreignKeyFailures); err != nil {
		return nil, "", err
	}
	if foreignKeyFailures != 0 {
		return nil, "", fmt.Errorf("sanitized metadata has %d foreign key failures", foreignKeyFailures)
	}
	var version string
	if err := tx.QueryRowContext(ctx, "SELECT filename FROM schema_migrations ORDER BY rowid DESC LIMIT 1").Scan(&version); err != nil {
		return nil, "", err
	}
	if err := validateArtifactBlobs(ctx, store); err != nil {
		return nil, "", err
	}
	if err := tx.Commit(); err != nil {
		return nil, "", err
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].Original < mappings[j].Original })
	return mappings, version, nil
}

// validateArtifactBlobs recomputes every copied artifact size and SHA-256 identity.
func validateArtifactBlobs(ctx context.Context, store *exportStore) error {
	blobs, err := store.artifactBlobs(ctx)
	if err != nil {
		return err
	}
	for _, blob := range blobs {
		digest := sha256.Sum256(blob.Data)
		if blob.ByteSize != int64(len(blob.Data)) || blob.ContentHash != hex.EncodeToString(digest[:]) {
			return fmt.Errorf("artifact blob hash or size validation failed for %s", blob.ContentHash)
		}
	}
	return nil
}

// snapshotSQLite uses VACUUM INTO to capture a WAL-consistent source without overwriting a destination.
func snapshotSQLite(ctx context.Context, source *sql.DB, destination string) error {
	if _, err := os.Stat(destination); err == nil {
		return fmt.Errorf("snapshot destination exists")
	}
	_, err := source.ExecContext(ctx, "VACUUM INTO ?", destination)
	return err
}

// schemaVersion opens one SQLite file read-only and returns its latest recorded migration filename.
func schemaVersion(ctx context.Context, path string) (string, error) {
	db, err := sql.Open("sqlite", sqliteReadURI(path))
	if err != nil {
		return "", err
	}
	defer db.Close()
	return schemaVersionDB(ctx, db)
}

// schemaVersionDB returns the latest recorded migration filename from an open SQLite connection.
func schemaVersionDB(ctx context.Context, db *sql.DB) (string, error) {
	var version string
	if err := db.QueryRowContext(ctx, "SELECT filename FROM schema_migrations ORDER BY rowid DESC LIMIT 1").Scan(&version); err != nil {
		return "", err
	}
	return version, nil
}

// exportedFileHashes hashes every regular copied file and rejects unsupported file types.
func exportedFileHashes(root string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() == "export-manifest.json" {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("export contains unsupported file type: %s", path)
		}
		hash, err := filehash.SHA256(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(mustRelative(root, path))] = hash
		return nil
	})
	return files, err
}

// safeCompanionPath resolves a clean bundle-relative PDF binding and rejects a
// resolved target that escapes the metadata bundle through a symbolic link.
func safeCompanionPath(metadataPath, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) || filepath.Clean(relative) != relative || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe companion binding %q", relative)
	}
	companion, err := pathpolicy.ResolveExistingWithin(filepath.Dir(metadataPath), relative)
	if err != nil {
		return "", fmt.Errorf("bound PDF database escapes metadata bundle: %w", err)
	}
	if _, err := existingFile(companion); err != nil {
		return "", err
	}
	return companion, nil
}

// existingFile returns a resolved absolute path only for an existing regular file.
func existingFile(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("not a regular file: %s", resolved)
	}
	return resolved, nil
}

// samePath detects aliases using filesystem identity when both targets exist.
func samePath(left, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr == nil && rightErr == nil {
		return os.SameFile(leftInfo, rightInfo)
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

// sqliteReadURI returns an existing-only read URI with the project busy timeout.
func sqliteReadURI(path string) string {
	return sqliteuri.File(path, map[string][]string{
		"mode":    {"ro"},
		"_pragma": {"busy_timeout(5000)"},
	})
}

// sqliteWritableURI returns a writable SQLite URI with the project pragmas.
func sqliteWritableURI(path string) string {
	return sqliteuri.File(path, map[string][]string{
		"_pragma": {"foreign_keys(1)", "busy_timeout(5000)"},
	})
}

// mustRelative returns the previously validated path relative to its export root.
func mustRelative(root, path string) string {
	value, err := filepath.Rel(root, path)
	if err != nil {
		panic(err)
	}
	return value
}
