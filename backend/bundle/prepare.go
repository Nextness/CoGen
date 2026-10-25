// prepare.go coordinates metadata and PDF schema preparation as one resumable
// bundle operation under exclusive maintenance ownership.
package bundle

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"analysis/backend/database"
	"analysis/backend/database/pdfbinding"
	"analysis/backend/pdfstore"
	"analysis/internal/migrations"
	"analysis/internal/sqliteread"
)

// Paths identifies the metadata and companion database files prepared as one bundle.
type Paths struct {
	Metadata string
	PDF      string
}

// StoreReport describes one store's verified target and migration progress.
type StoreReport struct {
	Target  string
	Applied int
	Adopted int
	Skipped int
}

// Report describes one completed bundle preparation.
type Report struct {
	Metadata StoreReport
	PDF      StoreReport
	Resolved Paths
}

// Readiness reports each store's latest recorded migration and configured target.
type Readiness struct {
	Metadata       string
	MetadataTarget string
	PDF            string
	PDFTarget      string
}

// Prepare validates both configured chains and their migration inputs,
// acquires exclusive maintenance ownership, applies metadata migrations
// followed by PDF migrations, and verifies both targets. When the PDF store
// fails after metadata has committed, it returns the metadata progress so a
// later call can resume forward without downgrading the completed store.
func Prepare(ctx context.Context, registryPath string, paths Paths) (Report, error) {
	if ctx == nil {
		return Report{}, fmt.Errorf("context is required")
	}
	if err := validatePaths(paths); err != nil {
		return Report{}, err
	}
	plan, err := loadPlan(registryPath)
	if err != nil {
		return Report{}, err
	}
	lock, err := database.AcquireMaintenanceLock(ctx, paths.Metadata, database.MaintenanceExclusive)
	if err != nil {
		return Report{}, fmt.Errorf("acquire maintenance ownership: %w", err)
	}
	defer lock.Close()

	if err := preflightHistory(ctx, "metadata", paths.Metadata, plan.metadataEntries); err != nil {
		return Report{}, err
	}
	if err := preflightHistory(ctx, "pdf", paths.PDF, plan.pdfEntries); err != nil {
		return Report{}, err
	}

	metadataReport, err := prepareStore(ctx, "metadata", plan.metadata, plan.metadataEntries, paths.Metadata, nil)
	if err != nil {
		return Report{}, err
	}
	pdfReport, err := prepareStore(ctx, "pdf", plan.pdf, plan.pdfEntries, paths.PDF, pdfstore.VerifySchema)
	if err != nil {
		return Report{Metadata: metadataReport, Resolved: paths}, err
	}
	return Report{Metadata: metadataReport, PDF: pdfReport, Resolved: paths}, nil
}

// PrepareResolved prepares both stores for metadataPath, resolving the
// companion path from the metadata binding. An unbound metadata database is
// bound to the default companion after that companion is prepared. An existing
// binding whose file is missing or unsafe is rejected instead of creating an
// empty replacement.
func PrepareResolved(ctx context.Context, registryPath, metadataPath string) (Report, error) {
	if ctx == nil {
		return Report{}, fmt.Errorf("context is required")
	}
	if strings.TrimSpace(metadataPath) == "" {
		return Report{}, fmt.Errorf("metadata database path is required")
	}
	plan, err := loadPlan(registryPath)
	if err != nil {
		return Report{}, err
	}
	lock, err := database.AcquireMaintenanceLock(ctx, metadataPath, database.MaintenanceExclusive)
	if err != nil {
		return Report{}, fmt.Errorf("acquire maintenance ownership: %w", err)
	}
	defer lock.Close()

	metadataConn, err := database.OpenWritable(metadataPath)
	if err != nil {
		return Report{}, fmt.Errorf("open metadata store: %w", err)
	}
	defer metadataConn.Close()
	if err := preflightHistoryConnection(ctx, "metadata", metadataConn, plan.metadataEntries); err != nil {
		return Report{}, err
	}
	metadataReport, err := prepareStoreConnection(ctx, "metadata", plan.metadata, plan.metadataEntries, metadataConn, nil)
	if err != nil {
		return Report{}, err
	}

	bindings := pdfbinding.New(metadataConn)
	relativePath, found, err := bindings.Binding(ctx)
	if err != nil {
		return Report{Metadata: metadataReport}, err
	}
	initialize := !found
	if !found {
		relativePath = pdfbinding.DefaultStoreFilename
	}
	pdfPath, err := pdfbinding.ResolveStorePath(metadataPath, relativePath)
	if err != nil {
		return Report{Metadata: metadataReport}, fmt.Errorf("resolve PDF store: %w", err)
	}
	if err := validatePaths(Paths{Metadata: metadataPath, PDF: pdfPath}); err != nil {
		return Report{Metadata: metadataReport}, err
	}
	if !initialize {
		info, err := os.Stat(pdfPath)
		if err != nil {
			if os.IsNotExist(err) {
				return Report{Metadata: metadataReport}, fmt.Errorf("bound PDF store %q is missing; restore it or update the metadata binding instead of creating an empty replacement", relativePath)
			}
			return Report{Metadata: metadataReport}, fmt.Errorf("inspect bound PDF store: %w", err)
		}
		if !info.Mode().IsRegular() {
			return Report{Metadata: metadataReport}, fmt.Errorf("bound PDF store %q is not a regular file", relativePath)
		}
	}
	if err := preflightHistory(ctx, "pdf", pdfPath, plan.pdfEntries); err != nil {
		return Report{Metadata: metadataReport}, err
	}
	pdfReport, err := prepareStore(ctx, "pdf", plan.pdf, plan.pdfEntries, pdfPath, pdfstore.VerifySchema)
	if err != nil {
		return Report{Metadata: metadataReport}, err
	}
	if initialize {
		if err := bindings.EnsureBinding(ctx, pdfbinding.DefaultStoreFilename); err != nil {
			return Report{Metadata: metadataReport, PDF: pdfReport}, fmt.Errorf("bind default PDF store: %w", err)
		}
	}
	return Report{
		Metadata: metadataReport,
		PDF:      pdfReport,
		Resolved: Paths{Metadata: metadataPath, PDF: pdfPath},
	}, nil
}

// VerifyReadiness verifies that both stores are at the supported schema pair
// with complete required history and protection contracts. It never creates
// files, applies migrations, or changes either schema, and returns each store's
// latest recorded migration even when readiness fails. Callers that need a
// stable pair across both reads hold shared maintenance ownership for the check
// and their subsequent work.
func VerifyReadiness(ctx context.Context, registryPath string, paths Paths) (Readiness, error) {
	if ctx == nil {
		return Readiness{}, fmt.Errorf("context is required")
	}
	if err := validatePaths(paths); err != nil {
		return Readiness{}, err
	}
	plan, err := loadPlan(registryPath)
	if err != nil {
		return Readiness{}, err
	}
	readiness := Readiness{
		MetadataTarget: plan.metadataEntries[len(plan.metadataEntries)-1].Filename,
		PDFTarget:      plan.pdfEntries[len(plan.pdfEntries)-1].Filename,
	}
	var failures []error
	readiness.Metadata, err = currentTarget(ctx, "metadata", paths.Metadata, plan.metadataEntries)
	if err != nil {
		failures = append(failures, err)
	}
	readiness.PDF, err = currentTarget(ctx, "pdf", paths.PDF, plan.pdfEntries)
	if err != nil {
		failures = append(failures, err)
	} else if err := verifyPDFProtection(ctx, paths.PDF); err != nil {
		failures = append(failures, err)
	}
	return readiness, errors.Join(failures...)
}

// VerifyMetadataReadiness verifies only the metadata store and reports both
// configured targets without requiring or opening a companion.
func VerifyMetadataReadiness(ctx context.Context, registryPath, metadataPath string) (Readiness, error) {
	if ctx == nil {
		return Readiness{}, fmt.Errorf("context is required")
	}
	if strings.TrimSpace(metadataPath) == "" {
		return Readiness{}, fmt.Errorf("metadata database path is required")
	}
	plan, err := loadPlan(registryPath)
	if err != nil {
		return Readiness{}, err
	}
	readiness := Readiness{
		MetadataTarget: plan.metadataEntries[len(plan.metadataEntries)-1].Filename,
		PDFTarget:      plan.pdfEntries[len(plan.pdfEntries)-1].Filename,
	}
	readiness.Metadata, err = currentTarget(ctx, "metadata", metadataPath, plan.metadataEntries)
	return readiness, err
}

// VerifyPDFReadiness verifies only the companion store, including its required
// protection contracts, and reports both configured targets.
func VerifyPDFReadiness(ctx context.Context, registryPath, pdfPath string) (Readiness, error) {
	if ctx == nil {
		return Readiness{}, fmt.Errorf("context is required")
	}
	if strings.TrimSpace(pdfPath) == "" {
		return Readiness{}, fmt.Errorf("PDF store path is required")
	}
	plan, err := loadPlan(registryPath)
	if err != nil {
		return Readiness{}, err
	}
	readiness := Readiness{
		MetadataTarget: plan.metadataEntries[len(plan.metadataEntries)-1].Filename,
		PDFTarget:      plan.pdfEntries[len(plan.pdfEntries)-1].Filename,
	}
	readiness.PDF, err = currentTarget(ctx, "pdf", pdfPath, plan.pdfEntries)
	if err != nil {
		return readiness, err
	}
	if err := verifyPDFProtection(ctx, pdfPath); err != nil {
		return readiness, err
	}
	return readiness, nil
}

// migrationPlan holds both resolved chains and their validated migration inputs.
type migrationPlan struct {
	metadata        database.MigrationConfig
	pdf             database.MigrationConfig
	metadataEntries []migrations.Entry
	pdfEntries      []migrations.Entry
}

// loadPlan resolves both configured chains and validates their endpoints and
// readable migration inputs before either store changes.
func loadPlan(registryPath string) (migrationPlan, error) {
	metadataConfig, err := database.ResolveMigrationConfig(registryPath, database.StoreCorpusMetadata)
	if err != nil {
		return migrationPlan{}, fmt.Errorf("resolve metadata migrations: %w", err)
	}
	pdfConfig, err := database.ResolveMigrationConfig(registryPath, database.StoreCorpusPDF)
	if err != nil {
		return migrationPlan{}, fmt.Errorf("resolve PDF migrations: %w", err)
	}
	if err := VerifyPair(metadataConfig.ConfigPath, pdfConfig.ConfigPath, SupportedPair); err != nil {
		return migrationPlan{}, err
	}
	metadataEntries, err := migrations.LoadChain(metadataConfig.ConfigPath)
	if err != nil {
		return migrationPlan{}, fmt.Errorf("load metadata migration chain: %w", err)
	}
	if err := migrations.ValidateInputs(metadataEntries, metadataConfig.MigrationsDir); err != nil {
		return migrationPlan{}, fmt.Errorf("validate metadata migration inputs: %w", err)
	}
	pdfEntries, err := migrations.LoadChain(pdfConfig.ConfigPath)
	if err != nil {
		return migrationPlan{}, fmt.Errorf("load PDF migration chain: %w", err)
	}
	if err := migrations.ValidateInputs(pdfEntries, pdfConfig.MigrationsDir); err != nil {
		return migrationPlan{}, fmt.Errorf("validate PDF migration inputs: %w", err)
	}
	return migrationPlan{metadata: metadataConfig, pdf: pdfConfig, metadataEntries: metadataEntries, pdfEntries: pdfEntries}, nil
}

// preflightHistory verifies that an existing store records only a complete,
// ordered prefix of its configured chain. A missing store is allowed because
// preparation may create it.
func preflightHistory(ctx context.Context, name, path string, entries []migrations.Entry) error {
	applied, exists, err := readApplied(ctx, path)
	if err != nil {
		return fmt.Errorf("read %s migration history: %w", name, err)
	}
	if !exists {
		return nil
	}
	return preflightApplied(name, entries, applied)
}

// preflightHistoryConnection verifies one open store's recorded history.
func preflightHistoryConnection(ctx context.Context, name string, conn *sql.DB, entries []migrations.Entry) error {
	applied, err := migrations.Applied(ctx, conn)
	if err != nil {
		return fmt.Errorf("read %s migration history: %w", name, err)
	}
	return preflightApplied(name, entries, applied)
}

// preflightApplied verifies one recorded history against its configured chain.
func preflightApplied(name string, entries []migrations.Entry, applied []string) error {
	if _, err := migrations.VerifyHistory(entries, applied); err != nil {
		return fmt.Errorf("%s migration history: %w", name, err)
	}
	return nil
}

// prepareStore opens one store, applies pending migrations, verifies its
// target, and optionally verifies its protection contracts.
func prepareStore(ctx context.Context, name string, config database.MigrationConfig, entries []migrations.Entry, path string, verify func(context.Context, *sql.DB) error) (StoreReport, error) {
	conn, err := database.OpenWritable(path)
	if err != nil {
		return StoreReport{}, fmt.Errorf("open %s store: %w", name, err)
	}
	defer conn.Close()
	return prepareStoreConnection(ctx, name, config, entries, conn, verify)
}

// prepareStoreConnection applies pending migrations to an open store and
// verifies its target and optional protection contracts.
func prepareStoreConnection(ctx context.Context, name string, config database.MigrationConfig, entries []migrations.Entry, conn *sql.DB, verify func(context.Context, *sql.DB) error) (StoreReport, error) {
	result, err := migrations.ApplyContext(ctx, conn, config.MigrationsDir, config.ConfigPath)
	if err != nil {
		return StoreReport{}, fmt.Errorf("apply %s migrations: %w", name, err)
	}
	if result.Configured != len(entries) {
		return StoreReport{}, fmt.Errorf("%s migration chain changed during preparation", name)
	}
	applied, err := migrations.Applied(ctx, conn)
	if err != nil {
		return StoreReport{}, fmt.Errorf("read %s migration history: %w", name, err)
	}
	current, err := migrations.VerifyHistory(entries, applied)
	if err != nil {
		return StoreReport{}, fmt.Errorf("verify %s migration history: %w", name, err)
	}
	if current != len(entries)-1 {
		return StoreReport{}, fmt.Errorf("%s schema is not current after preparation", name)
	}
	if verify != nil {
		if err := verify(ctx, conn); err != nil {
			return StoreReport{}, fmt.Errorf("verify %s protection contracts: %w", name, err)
		}
	}
	return StoreReport{
		Target:  entries[current].Filename,
		Applied: result.Applied,
		Adopted: result.Adopted,
		Skipped: result.Skipped,
	}, nil
}

// currentTarget verifies one existing store against its configured chain and
// returns its latest recorded migration.
func currentTarget(ctx context.Context, name, path string, entries []migrations.Entry) (string, error) {
	applied, exists, err := readApplied(ctx, path)
	if err != nil {
		return "", fmt.Errorf("read %s migration history: %w", name, err)
	}
	if !exists {
		return "", fmt.Errorf("%s database does not exist", name)
	}
	actual := ""
	if len(applied) > 0 {
		actual = applied[len(applied)-1]
	}
	current, err := migrations.VerifyHistory(entries, applied)
	if err != nil {
		return actual, fmt.Errorf("%s migration history: %w", name, err)
	}
	target := entries[len(entries)-1].Filename
	if current != len(entries)-1 {
		return actual, fmt.Errorf("%s schema is not current: actual %q, target %q", name, actual, target)
	}
	return actual, nil
}

// verifyPDFProtection verifies the companion protection contracts through a read-only connection.
func verifyPDFProtection(ctx context.Context, path string) error {
	conn, err := sqliteread.Open(path, "sqlite")
	if err != nil {
		return fmt.Errorf("open PDF store read-only: %w", err)
	}
	defer conn.Close()
	if err := conn.PingContext(ctx); err != nil {
		return fmt.Errorf("read PDF store: %w", err)
	}
	if err := pdfstore.VerifySchema(ctx, conn); err != nil {
		return err
	}
	return nil
}

// readApplied reads one existing store's migration history read-only. It
// returns exists=false without opening a missing file.
func readApplied(ctx context.Context, path string) ([]string, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("inspect database: %w", err)
	}
	if info.IsDir() {
		return nil, false, fmt.Errorf("database path is a directory")
	}
	conn, err := sqliteread.Open(path, "sqlite")
	if err != nil {
		return nil, true, fmt.Errorf("open database read-only: %w", err)
	}
	defer conn.Close()
	if err := conn.PingContext(ctx); err != nil {
		return nil, true, fmt.Errorf("read database: %w", err)
	}
	applied, err := migrations.Applied(ctx, conn)
	if err != nil {
		return nil, true, err
	}
	return applied, true, nil
}

// validatePaths rejects empty and duplicate bundle paths.
func validatePaths(paths Paths) error {
	if strings.TrimSpace(paths.Metadata) == "" {
		return fmt.Errorf("metadata database path is required")
	}
	if strings.TrimSpace(paths.PDF) == "" {
		return fmt.Errorf("PDF store path is required")
	}
	metadataPath, err := canonicalPath(paths.Metadata)
	if err != nil {
		return fmt.Errorf("resolve metadata database path: %w", err)
	}
	pdfPath, err := canonicalPath(paths.PDF)
	if err != nil {
		return fmt.Errorf("resolve PDF store path: %w", err)
	}
	if metadataPath == pdfPath || sameFile(metadataPath, pdfPath) {
		return fmt.Errorf("PDF store path must differ from the metadata database path")
	}
	return nil
}

// canonicalPath resolves one bundle path through existing symbolic links, including a missing final component.
func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		return resolved, nil
	}
	if resolvedDir, err := filepath.EvalSymlinks(filepath.Dir(absolute)); err == nil {
		return filepath.Join(resolvedDir, filepath.Base(absolute)), nil
	}
	return absolute, nil
}

// sameFile reports whether two existing paths identify the same file.
func sameFile(left, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}
