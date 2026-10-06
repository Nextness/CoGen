// Package database provides a SQLite-backed storage layer for the corpus pipeline.
//
// Repository conflict handling preserves idempotency while content-addressed
// artifacts verify that a repeated identity has identical metadata and bytes.
package database

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"analysis/database/artifact"
	"analysis/database/audit"
	"analysis/database/cache"
	"analysis/database/run"
	"analysis/database/search"
	"analysis/database/source"
	"analysis/database/work"
	"analysis/logging"
)

var lg = logging.Logger("database")

// Database wraps a SQLite connection and exposes per-table repositories.
type Database struct {
	DB                   *sql.DB
	PipelineRuns         *PipelineRunRepository
	Run                  *run.Store
	Search               *search.Store
	Source               *source.Store
	Artifact             *artifact.Store
	Searches             *SearchRepository
	Revisions            *SearchRevisionRepository
	Plans                *ExecutionPlanRepository
	RunSources           *RunSourceRepository
	SourceRecords        *SourceRecordRepository
	Artifacts            *ArtifactRepository
	RunSteps             *RunStepRepository
	Metrics              *MetricsRepository
	AuditEvents          *AuditEventRepository
	Works                *WorkRepository
	WorkIdentifiers      *WorkIdentifierRepository
	WorkRevisions        *WorkRevisionRepository
	RunWorkStages        *RunWorkStageRepository
	People               *PersonRepository
	AuthorOccs           *AuthorOccurrenceRepository
	Authorships          *AuthorshipRepository
	IdentityResolutions  *AuthorIdentityResolutionRepository
	IdentityCandidates   *AuthorIdentityCandidateRepository
	ReferenceMentions    *ReferenceMentionRepository
	Cache                *cache.Store
	Audit                *audit.Store
	Work                 *work.Store
	ArtifactBlobs        *ArtifactBlobRepository
	RunArtifacts         *RunArtifactRepository
	SourceFilterCounts   *SourceFilterCountRepository
	PipelineRunReviewers *PipelineRunReviewerRepository
	TermMatches          *TermMatchesRepository
	Reviews              *ReviewRepository

	dbPath     string
	migrations string // migration SQL directory
}

// Open opens (or creates) the SQLite database at dbPath, runs pending
// migrations, and initialises repositories. Call Close when done.
func Open(dbPath, configPath string) (*Database, error) {
	conn, err := OpenConfigured(dbPath, configPath, StoreCorpusMetadata)
	if err != nil {
		return nil, err
	}

	d := &Database{
		DB:     conn,
		dbPath: dbPath,
	}
	d.initRepositories()

	lg.Debug("database open successful", "database_path", dbPath)
	return d, nil
}

// MigrateExisting applies the configured metadata migration chain to an existing file and never runs a workspace.
func MigrateExisting(dbPath, configPath string) error {
	if strings.TrimSpace(dbPath) == "" {
		return fmt.Errorf("database path is required")
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("database does not exist")
		}
		return fmt.Errorf("inspect database: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("database path is a directory")
	}
	db, err := Open(dbPath, configPath)
	if err != nil {
		return err
	}
	return db.Close()
}

// initRepositories binds every repository facade to the opened database.
func (d *Database) initRepositories() {
	d.PipelineRuns = &PipelineRunRepository{db: d}
	d.Run = run.New(d.DB)
	d.Search = search.New(d.DB)
	d.Source = source.New(d.DB)
	d.Artifact = artifact.New(d.DB)
	d.Searches = &SearchRepository{db: d}
	d.Revisions = &SearchRevisionRepository{db: d}
	d.Plans = &ExecutionPlanRepository{db: d}
	d.RunSources = &RunSourceRepository{db: d}
	d.SourceRecords = &SourceRecordRepository{db: d}
	d.Artifacts = &ArtifactRepository{db: d}
	d.RunSteps = &RunStepRepository{db: d}
	d.Metrics = &MetricsRepository{db: d}
	d.AuditEvents = &AuditEventRepository{db: d}
	d.Works = &WorkRepository{db: d}
	d.WorkIdentifiers = &WorkIdentifierRepository{db: d}
	d.WorkRevisions = &WorkRevisionRepository{db: d}
	d.RunWorkStages = &RunWorkStageRepository{db: d}
	d.People = &PersonRepository{db: d}
	d.AuthorOccs = &AuthorOccurrenceRepository{db: d}
	d.Authorships = &AuthorshipRepository{db: d}
	d.IdentityResolutions = &AuthorIdentityResolutionRepository{db: d}
	d.IdentityCandidates = &AuthorIdentityCandidateRepository{db: d}
	d.ReferenceMentions = &ReferenceMentionRepository{db: d}
	d.Cache = cache.New(d.DB)
	d.Audit = audit.New(d.DB)
	d.Work = work.New(d.DB)
	d.ArtifactBlobs = &ArtifactBlobRepository{db: d}
	d.RunArtifacts = &RunArtifactRepository{db: d}
	d.SourceFilterCounts = &SourceFilterCountRepository{db: d}
	d.PipelineRunReviewers = &PipelineRunReviewerRepository{db: d}
	d.TermMatches = &TermMatchesRepository{db: d}
	d.Reviews = &ReviewRepository{db: d}
}
