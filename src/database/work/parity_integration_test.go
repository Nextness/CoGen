// parity_integration_test.go verifies the legacy flat repositories and the
// work family store return and persist identical work, revision, stage,
// reference, and term evidence on separate copies of one migrated fixture.
//go:build integration

package work_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"analysis/database"
	"analysis/database/work"
)

// TestWorkFamilyParityWithLegacyRepositories verifies the adapters and family
// store agree on reads, writes, and errors for the core work-family resources.
func TestWorkFamilyParityWithLegacyRepositories(t *testing.T) {
	ctx := context.Background()
	fixturePath := filepath.Join(t.TempDir(), "fixture.db")
	fixture, err := database.Open(fixturePath, familyConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	runID, err := fixture.Run.StartRun(ctx, "work-parity", "")
	if err != nil {
		t.Fatal(err)
	}
	workID, err := fixture.Works.CreateByDOI("10.1000/parity-work")
	if err != nil {
		t.Fatal(err)
	}
	revision := &database.WorkRevision{
		WorkID: workID, PipelineRunID: runID, ProducerStage: database.ProducerStageParse, Title: "parity", Year: 2024,
	}
	revisionID, err := fixture.WorkRevisions.Create(revision)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.RunWorkStages.SetOutcome(runID, workID, database.StageNameParse, database.OutcomeParsed, "parity"); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.ReferenceMentions.Create(&database.ReferenceMention{
		WorkRevisionID: revisionID, MentionOrder: 1, RawReference: "parity", DOI: "10.1000/parity-target",
	}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.TermMatches.ReplaceRunTermData(runID, map[string][]string{"scopus": {"alpha"}}, map[int64]map[string][]string{
		revisionID: {"title": {"alpha"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.WorkIdentifiers.Insert(workID, "openalex", "W42"); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Close(); err != nil {
		t.Fatal(err)
	}

	legacyPath := filepath.Join(t.TempDir(), "legacy.db")
	familyPath := filepath.Join(t.TempDir(), "family.db")
	copyDatabaseFile(t, fixturePath, legacyPath)
	copyDatabaseFile(t, fixturePath, familyPath)

	legacy, err := database.OpenExisting(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	family, err := database.OpenExisting(familyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer family.Close()
	store := work.New(family.DB)

	// Read parity for work and identifier identity.
	legacyWork, err := legacy.Works.GetByDOI("10.1000/parity-work")
	if err != nil {
		t.Fatal(err)
	}
	familyWork, err := store.GetWorkByDOI(ctx, "10.1000/parity-work")
	if err != nil {
		t.Fatal(err)
	}
	assertWorkParity(t, "GetWorkByDOI", legacyWork, familyWork)

	legacyIdentifier, err := legacy.WorkIdentifiers.GetByNamespaceAndIdentifier("openalex", "W42")
	if err != nil {
		t.Fatal(err)
	}
	familyIdentifier, err := store.GetIdentifierByNamespaceAndIdentifier(ctx, "openalex", "W42")
	if err != nil {
		t.Fatal(err)
	}
	assertIdentifierParity(t, "GetIdentifierByNamespaceAndIdentifier", legacyIdentifier, familyIdentifier)

	// Read parity for revision, stage, and reference projections.
	legacyRevision, err := legacy.WorkRevisions.GetByID(revisionID)
	if err != nil {
		t.Fatal(err)
	}
	familyRevision, err := store.GetRevisionByID(ctx, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	assertRevisionParity(t, "GetRevisionByID", legacyRevision, familyRevision)

	legacyStage, err := legacy.RunWorkStages.GetByRunAndWork(runID, workID, database.StageNameParse)
	if err != nil {
		t.Fatal(err)
	}
	familyStage, err := store.GetStageOutcome(ctx, runID, workID, work.StageNameParse)
	if err != nil {
		t.Fatal(err)
	}
	assertStageParity(t, "GetStageOutcome", legacyStage, familyStage)

	legacyReferences, err := legacy.ReferenceMentions.GetByRevisionID(revisionID)
	if err != nil {
		t.Fatal(err)
	}
	familyReferences, err := store.ListReferencesByRevisionID(ctx, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(legacyReferences) != len(familyReferences) {
		t.Fatalf("reference parity length: legacy=%d family=%d", len(legacyReferences), len(familyReferences))
	}
	for index := range legacyReferences {
		assertReferenceParity(t, "ListReferencesByRevisionID", legacyReferences[index], familyReferences[index])
	}

	legacyTerms, err := legacy.TermMatches.GetRunTerms(runID)
	if err != nil {
		t.Fatal(err)
	}
	familyTerms, err := store.ListRunTerms(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(legacyTerms) != len(familyTerms) {
		t.Fatalf("term parity length: legacy=%d family=%d", len(legacyTerms), len(familyTerms))
	}
	for index := range legacyTerms {
		if legacyTerms[index].Term != familyTerms[index].Term || legacyTerms[index].SourceName != familyTerms[index].SourceName {
			t.Fatalf("term parity mismatch: legacy=%+v family=%+v", legacyTerms[index], familyTerms[index])
		}
	}

	// Missing-read identity parity.
	legacyMissing, err := legacy.Works.GetByDOI("10.1000/absent")
	if err != nil || legacyMissing != nil {
		t.Fatalf("legacy missing work = %+v err=%v", legacyMissing, err)
	}
	familyMissing, err := store.GetWorkByDOI(ctx, "10.1000/absent")
	if err != nil || familyMissing != nil {
		t.Fatalf("family missing work = %+v err=%v", familyMissing, err)
	}

	// Work write parity.
	legacyNewWork, err := legacy.Works.CreateByDOI("10.1000/parity-new")
	if err != nil {
		t.Fatal(err)
	}
	familyNewWork, err := store.CreateWorkByDOI(ctx, "10.1000/parity-new")
	if err != nil {
		t.Fatal(err)
	}
	if legacyNewWork != familyNewWork {
		t.Fatalf("CreateWorkByDOI id parity: legacy=%d family=%d", legacyNewWork, familyNewWork)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, doi FROM works WHERE id=?", legacyNewWork)

	// Revision write parity, including the adapter's hash copy-back.
	legacyNewRevision := &database.WorkRevision{
		WorkID: legacyNewWork, PipelineRunID: runID, ProducerStage: database.ProducerStageEnrich, Title: "parity-new",
	}
	familyNewRevision := &work.Revision{
		WorkID: familyNewWork, PipelineRunID: runID, ProducerStage: work.ProducerStageEnrich, Title: "parity-new",
	}
	legacyRevisionID, err := legacy.WorkRevisions.Create(legacyNewRevision)
	if err != nil {
		t.Fatal(err)
	}
	familyRevisionID, err := store.CreateRevision(ctx, familyNewRevision)
	if err != nil {
		t.Fatal(err)
	}
	if legacyRevisionID != familyRevisionID {
		t.Fatalf("CreateRevision id parity: legacy=%d family=%d", legacyRevisionID, familyRevisionID)
	}
	if legacyNewRevision.PayloadHash != familyNewRevision.PayloadHash || legacyNewRevision.FieldSchemaVersion != familyNewRevision.FieldSchemaVersion {
		t.Fatalf("CreateRevision hash parity: legacy=%q/%q family=%q/%q",
			legacyNewRevision.PayloadHash, legacyNewRevision.FieldSchemaVersion,
			familyNewRevision.PayloadHash, familyNewRevision.FieldSchemaVersion)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, work_id, producer_stage, payload_hash, field_schema_version, title, year FROM work_revisions WHERE id=?",
		legacyRevisionID)

	// Stage write parity.
	if err := legacy.RunWorkStages.SetOutcome(runID, legacyNewWork, database.StageNameValidate, database.OutcomeValid, "ok"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetStageOutcome(ctx, work.StageOutcomeInput{
		RunID: runID, WorkID: familyNewWork, StageName: work.StageNameValidate, Outcome: work.OutcomeValid, Reason: "ok",
	}); err != nil {
		t.Fatal(err)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT pipeline_run_id, work_id, stage_name, outcome, reason FROM run_work_stages WHERE pipeline_run_id=? ORDER BY id", runID)

	// Reference write parity, including DOI resolution and copy-back.
	legacyNewMention := &database.ReferenceMention{
		WorkRevisionID: legacyRevisionID, MentionOrder: 1, RawReference: "new", DOI: "https://doi.org/10.1000/parity-work",
	}
	familyNewMention := &work.Reference{
		WorkRevisionID: familyRevisionID, MentionOrder: 1, RawReference: "new", DOI: "https://doi.org/10.1000/parity-work",
	}
	legacyMentionID, err := legacy.ReferenceMentions.Create(legacyNewMention)
	if err != nil {
		t.Fatal(err)
	}
	familyMentionID, err := store.CreateReference(ctx, familyNewMention)
	if err != nil {
		t.Fatal(err)
	}
	if legacyMentionID != familyMentionID {
		t.Fatalf("CreateReference id parity: legacy=%d family=%d", legacyMentionID, familyMentionID)
	}
	if legacyNewMention.DOI != familyNewMention.DOI || legacyNewMention.ResolvedWorkID != familyNewMention.ResolvedWorkID {
		t.Fatalf("CreateReference resolution parity: legacy=%q/%d family=%q/%d",
			legacyNewMention.DOI, legacyNewMention.ResolvedWorkID, familyNewMention.DOI, familyNewMention.ResolvedWorkID)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT id, work_revision_id, resolved_work_id, mention_order, doi FROM reference_mentions WHERE id=?", legacyMentionID)

	// Term replacement parity.
	if err := legacy.TermMatches.ReplaceRunTermData(runID, map[string][]string{"ieee": {"beta"}}, map[int64]map[string][]string{
		legacyRevisionID: {"abstract": {"beta"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceRunTermData(ctx, work.ReplaceTermDataInput{
		RunID: runID, TermsBySource: map[string][]string{"ieee": {"beta"}},
		Matches: map[int64]map[string][]string{familyRevisionID: {"abstract": {"beta"}}},
	}); err != nil {
		t.Fatal(err)
	}
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT pipeline_run_id, source_name, term FROM run_search_terms WHERE pipeline_run_id=? ORDER BY id", runID)
	assertQueryParity(t, legacy.DB, family.DB,
		"SELECT pipeline_run_id, work_revision_id, field, term FROM work_revision_term_matches WHERE pipeline_run_id=? ORDER BY id", runID)

	// Error parity for conflicts, invalid vocabulary, and invalid input.
	assertErrorParity(t, "empty DOI",
		legacyCreateWorkError(legacy, " "),
		familyCreateWorkError(ctx, store, " "))
	assertErrorParity(t, "identifier ownership conflict",
		legacyIdentifierError(legacy, workID, "openalex", "W42"),
		familyIdentifierError(ctx, store, workID, "openalex", "W42"))
	assertErrorParity(t, "invalid stage outcome",
		legacy.RunWorkStages.SetOutcome(runID, workID, database.StageNameParse, database.OutcomeValid, ""),
		store.SetStageOutcome(ctx, work.StageOutcomeInput{
			RunID: runID, WorkID: workID, StageName: work.StageNameParse, Outcome: work.OutcomeValid,
		}))
	assertErrorParity(t, "invalid producer stage",
		legacyRevisionError(legacy, workID, runID),
		familyRevisionError(ctx, store, workID, runID))
}

// legacyCreateWorkError attempts one invalid work create and returns only its error.
func legacyCreateWorkError(db *database.Database, doi string) error {
	_, err := db.Works.CreateByDOI(doi)
	return err
}

// familyCreateWorkError attempts one invalid work create and returns only its error.
func familyCreateWorkError(ctx context.Context, store *work.Store, doi string) error {
	_, err := store.CreateWorkByDOI(ctx, doi)
	return err
}

// legacyIdentifierError attempts one conflicting identifier insert and returns only its error.
func legacyIdentifierError(db *database.Database, workID int64, namespace, identifier string) error {
	_, err := db.WorkIdentifiers.Insert(workID, namespace, identifier)
	return err
}

// familyIdentifierError attempts one conflicting identifier insert and returns only its error.
func familyIdentifierError(ctx context.Context, store *work.Store, workID int64, namespace, identifier string) error {
	_, err := store.InsertIdentifier(ctx, work.InsertIdentifierInput{WorkID: workID, Namespace: namespace, Identifier: identifier})
	return err
}

// legacyRevisionError attempts one invalid revision create and returns only its error.
func legacyRevisionError(db *database.Database, workID, runID int64) error {
	_, err := db.WorkRevisions.Create(&database.WorkRevision{WorkID: workID, PipelineRunID: runID, ProducerStage: "bogus"})
	return err
}

// familyRevisionError attempts one invalid revision create and returns only its error.
func familyRevisionError(ctx context.Context, store *work.Store, workID, runID int64) error {
	_, err := store.CreateRevision(ctx, &work.Revision{WorkID: workID, PipelineRunID: runID, ProducerStage: "bogus"})
	return err
}

// assertWorkParity compares legacy and family work projections.
func assertWorkParity(t *testing.T, operation string, legacy *database.Work, family *work.Work) {
	t.Helper()
	if (legacy == nil) != (family == nil) {
		t.Fatalf("%s nil mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
	if legacy == nil {
		return
	}
	if legacy.ID != family.ID || legacy.DOI != family.DOI || legacy.CreatedAt != family.CreatedAt {
		t.Fatalf("%s mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
}

// assertIdentifierParity compares legacy and family identifier projections.
func assertIdentifierParity(t *testing.T, operation string, legacy *database.WorkIdentifier, family *work.Identifier) {
	t.Helper()
	if (legacy == nil) != (family == nil) {
		t.Fatalf("%s nil mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
	if legacy == nil {
		return
	}
	if legacy.ID != family.ID || legacy.WorkID != family.WorkID || legacy.Namespace != family.Namespace ||
		legacy.Identifier != family.Identifier || legacy.CreatedAt != family.CreatedAt {
		t.Fatalf("%s mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
}

// assertRevisionParity compares legacy and family revision projections.
func assertRevisionParity(t *testing.T, operation string, legacy *database.WorkRevision, family *work.Revision) {
	t.Helper()
	if (legacy == nil) != (family == nil) {
		t.Fatalf("%s nil mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
	if legacy == nil {
		return
	}
	if !reflect.DeepEqual(*legacy, database.WorkRevision{
		ID: family.ID, WorkID: family.WorkID, PipelineRunID: family.PipelineRunID,
		ProducerStage: family.ProducerStage, FieldSchemaVersion: family.FieldSchemaVersion,
		PayloadHash: family.PayloadHash, Title: family.Title, Abstract: family.Abstract,
		Year: family.Year, Journal: family.Journal, Publisher: family.Publisher, Source: family.Source,
		Keywords: family.Keywords, KeywordsPlus: family.KeywordsPlus, CitationCount: family.CitationCount,
		ReferenceCount: family.ReferenceCount, ExtensionData: family.ExtensionData, CreatedAt: family.CreatedAt,
	}) {
		t.Fatalf("%s mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
}

// assertStageParity compares legacy and family stage projections.
func assertStageParity(t *testing.T, operation string, legacy *database.RunWorkStage, family *work.StageOutcome) {
	t.Helper()
	if (legacy == nil) != (family == nil) {
		t.Fatalf("%s nil mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
	if legacy == nil {
		return
	}
	if legacy.ID != family.ID || legacy.PipelineRunID != family.PipelineRunID || legacy.WorkID != family.WorkID ||
		legacy.StageName != family.StageName || legacy.Outcome != family.Outcome || legacy.Reason != family.Reason ||
		legacy.CreatedAt != family.CreatedAt || legacy.UpdatedAt != family.UpdatedAt {
		t.Fatalf("%s mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
}

// assertReferenceParity compares legacy and family reference projections.
func assertReferenceParity(t *testing.T, operation string, legacy *database.ReferenceMention, family *work.Reference) {
	t.Helper()
	if legacy == nil || family == nil {
		t.Fatalf("%s nil projection: legacy=%+v family=%+v", operation, legacy, family)
	}
	if legacy.ID != family.ID || legacy.WorkRevisionID != family.WorkRevisionID ||
		legacy.ResolvedWorkID != family.ResolvedWorkID || legacy.MentionOrder != family.MentionOrder ||
		legacy.RawReference != family.RawReference || legacy.DOI != family.DOI || legacy.Title != family.Title ||
		legacy.Author != family.Author || legacy.Year != family.Year || legacy.Source != family.Source ||
		legacy.CreatedAt != family.CreatedAt {
		t.Fatalf("%s mismatch: legacy=%+v family=%+v", operation, legacy, family)
	}
}

// assertQueryParity runs the same read on both databases and compares every returned row.
func assertQueryParity(t *testing.T, legacy, family *sql.DB, query string, args ...any) {
	t.Helper()
	legacyRows := queryRowValues(t, legacy, query, args...)
	familyRows := queryRowValues(t, family, query, args...)
	if !reflect.DeepEqual(legacyRows, familyRows) {
		t.Fatalf("query parity mismatch:\nlegacy=%v\nfamily=%v", legacyRows, familyRows)
	}
}

// queryRowValues renders every row of one query as ordered comparable text.
func queryRowValues(t *testing.T, db *sql.DB, query string, args ...any) [][]string {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var result [][]string
	for rows.Next() {
		values := make([]any, len(names))
		pointers := make([]any, len(names))
		for index := range values {
			pointers[index] = &values[index]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		rendered := make([]string, len(names))
		for index, value := range values {
			rendered[index] = formatDatabaseValue(value)
		}
		result = append(result, rendered)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

// copyDatabaseFile copies one checkpointed fixture into an independent working copy.
func copyDatabaseFile(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// formatDatabaseValue renders one scanned SQLite value as comparable text.
func formatDatabaseValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "<null>"
	case []byte:
		return string(typed)
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

// assertErrorParity verifies legacy and family operations reject input with the same error text.
func assertErrorParity(t *testing.T, operation string, legacyErr, familyErr error) {
	t.Helper()
	if (legacyErr == nil) != (familyErr == nil) {
		t.Fatalf("%s error mismatch: legacy=%v family=%v", operation, legacyErr, familyErr)
	}
	if legacyErr != nil && legacyErr.Error() != familyErr.Error() {
		t.Fatalf("%s error text mismatch: legacy=%q family=%q", operation, legacyErr, familyErr)
	}
}
