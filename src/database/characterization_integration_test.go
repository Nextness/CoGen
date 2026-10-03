// characterization_integration_test.go pins database-layer contracts that the
// type-safe SQL migration must preserve. The critical operations marked for
// direct old-versus-new comparison are recorded in PARITY.md.
//go:build integration

package database

import (
	"context"
	"errors"
	"strings"
	"testing"

	"analysis/manifest"
)

// TestCharacterizationReadNotFoundReturnsNilError verifies every representative
// optional read keeps the current (nil, nil) not-found identity instead of
// leaking sql.ErrNoRows or a typed not-found error.
func TestCharacterizationReadNotFoundReturnsNilError(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	cases := []struct {
		name string
		read func() (bool, error)
	}{
		{"SearchRepository.GetByID", func() (bool, error) { value, err := db.Searches.GetByID(999); return value == nil, err }},
		{"SearchRepository.GetBySearchID", func() (bool, error) { value, err := db.Searches.GetBySearchID("absent"); return value == nil, err }},
		{"SearchRevisionRepository.GetByID", func() (bool, error) { value, err := db.Revisions.GetByID(999); return value == nil, err }},
		{"SearchRevisionRepository.GetBySearchAndRevision", func() (bool, error) {
			value, err := db.Revisions.GetBySearchAndRevision(999, "absent")
			return value == nil, err
		}},
		{"ExecutionPlanRepository.GetByID", func() (bool, error) { value, err := db.Plans.GetByID(999); return value == nil, err }},
		{"ExecutionPlanRepository.GetByFingerprint", func() (bool, error) { value, err := db.Plans.GetByFingerprint(999, "absent"); return value == nil, err }},
		{"PipelineRunRepository.GetByID", func() (bool, error) { value, err := db.PipelineRuns.GetByID(999); return value == nil, err }},
		{"PipelineRunReviewerRepository.Get", func() (bool, error) { value, err := db.PipelineRunReviewers.Get(999); return value == nil, err }},
		{"MetricsRepository.Get", func() (bool, error) { value, err := db.Metrics.Get(999, "absent", ""); return value == nil, err }},
		{"SourceFilterCountRepository.GetByRunAndSource", func() (bool, error) {
			value, err := db.SourceFilterCounts.GetByRunAndSource(999, "absent")
			return value == nil, err
		}},
		{"ArtifactRepository.GetByID", func() (bool, error) { value, err := db.Artifacts.GetByID(999); return value == nil, err }},
		{"ArtifactRepository.GetByHash", func() (bool, error) { value, err := db.Artifacts.GetByHash("absent"); return value == nil, err }},
		{"ArtifactBlobRepository.GetByArtifactID", func() (bool, error) { value, err := db.ArtifactBlobs.GetByArtifactID(999); return value == nil, err }},
		{"CacheEntryRepository.Get", func() (bool, error) {
			value, err := db.CacheEntries.Get("absent", "absent", "absent", "absent")
			return value == nil, err
		}},
		{"CacheEntryRepository.GetGlobal", func() (bool, error) {
			value, err := db.CacheEntries.GetGlobal("absent", "absent", "absent", "absent")
			return value == nil, err
		}},
		{"RunCacheUseRepository.FindEntry", func() (bool, error) {
			value, err := db.RunCacheUses.FindEntry(999, "global", "absent", "absent", "absent", "absent")
			return value == nil, err
		}},
		{"RunCacheUseRepository.FindAnyEntry", func() (bool, error) {
			value, err := db.RunCacheUses.FindAnyEntry(999, "absent", "absent", "absent", "absent")
			return value == nil, err
		}},
		{"WorkRepository.GetByID", func() (bool, error) { value, err := db.Works.GetByID(999); return value == nil, err }},
		{"WorkRepository.GetByDOI", func() (bool, error) { value, err := db.Works.GetByDOI("10.1000/absent"); return value == nil, err }},
		{"WorkIdentifierRepository.GetByID", func() (bool, error) { value, err := db.WorkIdentifiers.GetByID(999); return value == nil, err }},
		{"WorkIdentifierRepository.GetByNamespaceAndIdentifier", func() (bool, error) {
			value, err := db.WorkIdentifiers.GetByNamespaceAndIdentifier("absent", "absent")
			return value == nil, err
		}},
		{"WorkRevisionRepository.GetByID", func() (bool, error) { value, err := db.WorkRevisions.GetByID(999); return value == nil, err }},
		{"RunWorkStageRepository.GetByRunAndWork", func() (bool, error) {
			value, err := db.RunWorkStages.GetByRunAndWork(999, 999, "validate")
			return value == nil, err
		}},
		{"PersonRepository.GetByID", func() (bool, error) { value, err := db.People.GetByID(999); return value == nil, err }},
		{"PersonRepository.GetByORCID", func() (bool, error) {
			value, err := db.People.GetByORCID("0000-0002-1825-0097")
			return value == nil, err
		}},
		{"AuthorOccurrenceRepository.GetByID", func() (bool, error) { value, err := db.AuthorOccs.GetByID(999); return value == nil, err }},
		{"ReferenceMentionRepository.GetByID", func() (bool, error) { value, err := db.ReferenceMentions.GetByID(999); return value == nil, err }},
		{"ReviewRepository.GetContextByRun", func() (bool, error) { value, err := db.Reviews.GetContextByRun(ctx, 999); return value == nil, err }},
		{"ReviewRepository.GetNote", func() (bool, error) { value, err := db.Reviews.GetNote(ctx, 999, 999); return value == nil, err }},
		{"ReviewRepository.GetAnchor", func() (bool, error) { value, err := db.Reviews.GetAnchor(ctx, 999, "absent"); return value == nil, err }},
		{"ReviewRepository.GetWorkReviewVersion", func() (bool, error) {
			value, err := db.Reviews.GetWorkReviewVersion(ctx, 999, 999, 999)
			return value == nil, err
		}},
		{"ReviewRepository.GetNoteVersion", func() (bool, error) {
			value, err := db.Reviews.GetNoteVersion(ctx, 999, 999, 999)
			return value == nil, err
		}},
		{"ReviewRepository.GetAnchorVersion", func() (bool, error) {
			value, err := db.Reviews.GetAnchorVersion(ctx, 999, "absent", 999)
			return value == nil, err
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			isNil, err := test.read()
			if err != nil {
				t.Fatalf("not-found read returned error: %v", err)
			}
			if !isNil {
				t.Fatal("not-found read returned a value, want nil")
			}
		})
	}
}

// TestCharacterizationEmptyListSliceIdentity verifies each representative list
// read keeps its exact nil-versus-empty slice behavior so later generated
// queries cannot silently change API JSON from null to [] or the reverse.
func TestCharacterizationEmptyListSliceIdentity(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	cases := []struct {
		name    string
		wantNil bool
		read    func() (bool, error)
	}{
		{"SearchRepository.List", true, func() (bool, error) { value, err := db.Searches.List(); return value == nil, err }},
		{"SearchRevisionRepository.ListBySearch", true, func() (bool, error) {
			value, err := db.Revisions.ListBySearch(999)
			return value == nil, err
		}},
		{"ExecutionPlanRepository.ListBySearchRevision", true, func() (bool, error) {
			value, err := db.Plans.ListBySearchRevision(999)
			return value == nil, err
		}},
		{"RunSourceRepository.ListByRun", true, func() (bool, error) { value, err := db.RunSources.ListByRun(999); return value == nil, err }},
		{"SourceRecordRepository.ListBySource", true, func() (bool, error) { value, err := db.SourceRecords.ListBySource(999); return value == nil, err }},
		{"RunStepRepository.ListByRun", true, func() (bool, error) { value, err := db.RunSteps.ListByRun(999); return value == nil, err }},
		{"SourceFilterCountRepository.ListByRun", true, func() (bool, error) {
			value, err := db.SourceFilterCounts.ListByRun(999)
			return value == nil, err
		}},
		{"PipelineRunRepository.ListByPlan", true, func() (bool, error) { value, err := db.PipelineRuns.ListByPlan(999); return value == nil, err }},
		{"PipelineRunRepository.ListByVisibility", true, func() (bool, error) {
			value, err := db.PipelineRuns.ListByVisibility("trashed")
			return value == nil, err
		}},
		{"MetricsRepository.ListByRun", true, func() (bool, error) { value, err := db.Metrics.ListByRun(999); return value == nil, err }},
		{"AuditEventRepository.ListByRun", true, func() (bool, error) { value, err := db.AuditEvents.ListByRun(999); return value == nil, err }},
		{"AuditEventRepository.ListByEntity", true, func() (bool, error) {
			value, err := db.AuditEvents.ListByEntity("absent", "absent")
			return value == nil, err
		}},
		{"WorkRepository.ListByIDs nil", true, func() (bool, error) { value, err := db.Works.ListByIDs(nil); return value == nil, err }},
		{"WorkRepository.ListByIDs absent", true, func() (bool, error) { value, err := db.Works.ListByIDs([]int64{999}); return value == nil, err }},
		{"WorkIdentifierRepository.GetByWorkID", true, func() (bool, error) {
			value, err := db.WorkIdentifiers.GetByWorkID(999)
			return value == nil, err
		}},
		{"WorkRevisionRepository.GetByWorkID", true, func() (bool, error) {
			value, err := db.WorkRevisions.GetByWorkID(999)
			return value == nil, err
		}},
		{"WorkRevisionRepository.GetByRunID", true, func() (bool, error) {
			value, err := db.WorkRevisions.GetByRunID(999)
			return value == nil, err
		}},
		{"RunWorkStageRepository.GetByRunID", true, func() (bool, error) { value, err := db.RunWorkStages.GetByRunID(999); return value == nil, err }},
		{"RunWorkStageRepository.GetByWorkID", true, func() (bool, error) { value, err := db.RunWorkStages.GetByWorkID(999); return value == nil, err }},
		{"AuthorOccurrenceRepository.GetByPersonID", true, func() (bool, error) {
			value, err := db.AuthorOccs.GetByPersonID(999)
			return value == nil, err
		}},
		{"AuthorshipRepository.GetByRevisionID", true, func() (bool, error) {
			value, err := db.Authorships.GetByRevisionID(999)
			return value == nil, err
		}},
		{"AuthorshipRepository.GetByOccurrenceID", true, func() (bool, error) {
			value, err := db.Authorships.GetByOccurrenceID(999)
			return value == nil, err
		}},
		{"TermMatchesRepository.GetRunTerms", true, func() (bool, error) { value, err := db.TermMatches.GetRunTerms(999); return value == nil, err }},
		{"RunCacheUseRepository.ListByRun", false, func() (bool, error) { value, err := db.RunCacheUses.ListByRun(999); return value == nil, err }},
		{"ReferenceMentionRepository.GetByRevisionID", false, func() (bool, error) {
			value, err := db.ReferenceMentions.GetByRevisionID(999)
			return value == nil, err
		}},
		{"ReferenceMentionRepository.GetByResolvedWorkID", false, func() (bool, error) {
			value, err := db.ReferenceMentions.GetByResolvedWorkID(999)
			return value == nil, err
		}},
		{"ReviewRepository.ListNotes", false, func() (bool, error) {
			value, err := db.Reviews.ListNotes(ctx, 999, 999, 0, 10, false)
			return value == nil, err
		}},
		{"ReviewRepository.ListNoteVersions", false, func() (bool, error) {
			value, err := db.Reviews.ListNoteVersions(ctx, 999, 999, 0, 10)
			return value == nil, err
		}},
		{"ReviewRepository.ListWorkReviewVersions", false, func() (bool, error) {
			value, err := db.Reviews.ListWorkReviewVersions(ctx, 999, 999, 0, 10)
			return value == nil, err
		}},
		{"ReviewRepository.ListAnchors", false, func() (bool, error) {
			value, err := db.Reviews.ListAnchors(ctx, 999, 999, "", 10)
			return value == nil, err
		}},
		{"ReviewRepository.ListAnchorVersions", false, func() (bool, error) {
			value, err := db.Reviews.ListAnchorVersions(ctx, 999, "absent", 0, 10)
			return value == nil, err
		}},
		{"ReviewRepository.ListBacklinks", false, func() (bool, error) {
			value, err := db.Reviews.ListBacklinks(ctx, 999, "pdf_page", "1", 0, 0, 10)
			return value == nil, err
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			isNil, err := test.read()
			if err != nil {
				t.Fatalf("empty list read returned error: %v", err)
			}
			if isNil != test.wantNil {
				t.Fatalf("nil slice = %v, want %v", isNil, test.wantNil)
			}
		})
	}
}

// TestCharacterizationNullableScanningRoundTrip verifies nullable text and
// integer columns scan into the documented zero values and pointers.
func TestCharacterizationNullableScanningRoundTrip(t *testing.T) {
	t.Run("run source optional fields", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		runID, err := db.PipelineRuns.StartRun("characterization nullable", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.RunSources.Create(runID, "scopus", "csv", "scopus.csv", "", "", 0, ""); err != nil {
			t.Fatal(err)
		}
		sources, err := db.RunSources.ListByRun(runID)
		if err != nil || len(sources) != 1 {
			t.Fatalf("sources=%d err=%v", len(sources), err)
		}
		source := sources[0]
		if source.Query != "" || source.RequestedFields != "" || source.ExportDate != "" || source.ResultCountComparison != "" {
			t.Fatalf("optional text = %+v, want empty values", source)
		}
		if source.ExpectedResultCount == nil || *source.ExpectedResultCount != 0 {
			t.Fatalf("expected result count = %v, want pointer to zero", source.ExpectedResultCount)
		}
		if source.ObservedResultCount != nil {
			t.Fatalf("observed result count = %v, want nil", source.ObservedResultCount)
		}
	})
	t.Run("cache entry payload and expiry", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		if _, err := db.CacheEntries.Upsert(&CacheEntry{
			Provider: "crossref", Namespace: "works", RequestFingerprint: "nullable-negative",
			ResponseStatus: 404, FetchedAt: "2026-01-01T00:00:00Z", ExtractorVersion: "1",
		}); err != nil {
			t.Fatal(err)
		}
		negative, err := db.CacheEntries.Get("crossref", "works", "nullable-negative", "1")
		if err != nil || negative == nil {
			t.Fatalf("negative entry=%+v err=%v", negative, err)
		}
		if negative.PayloadArtifactID != nil || negative.ExpiresAt != "" {
			t.Fatalf("negative entry nullable fields = %+v", negative)
		}
		artifactID, err := db.Artifacts.Create("nullable-cache-payload", "application/json", 4)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.CacheEntries.Upsert(&CacheEntry{
			Provider: "crossref", Namespace: "works", RequestFingerprint: "nullable-positive",
			ResponseStatus: 200, PayloadArtifactID: &artifactID, FetchedAt: "2026-01-01T00:00:00Z",
			ExpiresAt: "2026-02-01T00:00:00Z", ExtractorVersion: "1",
		}); err != nil {
			t.Fatal(err)
		}
		positive, err := db.CacheEntries.Get("crossref", "works", "nullable-positive", "1")
		if err != nil || positive == nil {
			t.Fatalf("positive entry=%+v err=%v", positive, err)
		}
		if positive.PayloadArtifactID == nil || *positive.PayloadArtifactID != artifactID || positive.ExpiresAt != "2026-02-01T00:00:00Z" {
			t.Fatalf("positive entry nullable fields = %+v", positive)
		}
	})
	t.Run("work revision nullable projection", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		runID, err := db.PipelineRuns.StartRun("characterization revision", "")
		if err != nil {
			t.Fatal(err)
		}
		workID, err := db.Works.CreateByDOI("10.1000/characterization-revision")
		if err != nil {
			t.Fatal(err)
		}
		revisionID, err := db.WorkRevisions.Create(&WorkRevision{
			WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageParse,
		})
		if err != nil {
			t.Fatal(err)
		}
		revision, err := db.WorkRevisions.GetByID(revisionID)
		if err != nil || revision == nil {
			t.Fatalf("revision=%+v err=%v", revision, err)
		}
		if revision.Title != "" || revision.Abstract != "" || revision.Year != 0 || revision.Journal != "" ||
			revision.Publisher != "" || revision.Source != "" || revision.Keywords != "" ||
			revision.KeywordsPlus != "" || revision.CitationCount != 0 || revision.ReferenceCount != 0 ||
			revision.ExtensionData != "" {
			t.Fatalf("nullable revision projection = %+v, want zero values", revision)
		}
	})
	t.Run("reference mention nullable projection", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		revisionID := createReferenceMentionTestRevision(t, db, "10.1000/characterization-mention")
		mentionID, err := db.ReferenceMentions.Create(&ReferenceMention{
			WorkRevisionID: revisionID, MentionOrder: 1, RawReference: "raw reference",
		})
		if err != nil {
			t.Fatal(err)
		}
		mention, err := db.ReferenceMentions.GetByID(mentionID)
		if err != nil || mention == nil {
			t.Fatalf("mention=%+v err=%v", mention, err)
		}
		if mention.ResolvedWorkID != 0 || mention.DOI != "" || mention.Title != "" || mention.Author != "" ||
			mention.Year != 0 || mention.Source != "" {
			t.Fatalf("nullable mention projection = %+v, want zero values", mention)
		}
	})
	t.Run("audit event nullable projection", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		if _, err := db.AuditEvents.Insert(&manifest.AuditEvent{
			OccurredAt: "2026-01-01T00:00:00Z", Actor: "pipeline",
			EntityType: "work", EntityID: "1", Action: manifest.AuditFieldEnriched,
		}); err != nil {
			t.Fatal(err)
		}
		events, err := db.AuditEvents.ListByEntity("work", "1")
		if err != nil || len(events) != 1 {
			t.Fatalf("events=%d err=%v", len(events), err)
		}
		event := events[0]
		if event.PipelineRunID != nil || event.BeforeJSON != "" || event.AfterJSON != "" ||
			event.MetadataJSON != "" || event.CorrelationID != "" {
			t.Fatalf("nullable audit projection = %+v, want zero values", event)
		}
	})
}

// TestCharacterizationExplicitOrdering verifies representative list reads use
// their declared ORDER BY rather than insertion or primary-key accident.
func TestCharacterizationExplicitOrdering(t *testing.T) {
	t.Run("search revisions order by id", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		searchID, err := db.Searches.Create("characterization ordering")
		if err != nil {
			t.Fatal(err)
		}
		zetaID, _, err := db.Revisions.Create(searchID, "zeta", "config-z", "manifest-z")
		if err != nil {
			t.Fatal(err)
		}
		alphaID, _, err := db.Revisions.Create(searchID, "alpha", "config-a", "manifest-a")
		if err != nil {
			t.Fatal(err)
		}
		revisions, err := db.Revisions.ListBySearch(searchID)
		if err != nil || len(revisions) != 2 {
			t.Fatalf("revisions=%d err=%v", len(revisions), err)
		}
		if revisions[0].ID != zetaID || revisions[1].ID != alphaID {
			t.Fatalf("revision order = [%d, %d], want [%d, %d]", revisions[0].ID, revisions[1].ID, zetaID, alphaID)
		}
	})
	t.Run("execution plans order by id", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		searchID, _ := db.Searches.Create("characterization plans")
		revisionID, _, _ := db.Revisions.Create(searchID, "r1", "config", "manifest")
		zetaID, err := db.Plans.Create(revisionID, "fp-zeta", "manifest-z")
		if err != nil {
			t.Fatal(err)
		}
		alphaID, err := db.Plans.Create(revisionID, "fp-alpha", "manifest-a")
		if err != nil {
			t.Fatal(err)
		}
		plans, err := db.Plans.ListBySearchRevision(revisionID)
		if err != nil || len(plans) != 2 {
			t.Fatalf("plans=%d err=%v", len(plans), err)
		}
		if plans[0].ID != zetaID || plans[1].ID != alphaID {
			t.Fatalf("plan order = [%d, %d], want [%d, %d]", plans[0].ID, plans[1].ID, zetaID, alphaID)
		}
	})
	t.Run("run sources order by id", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		runID, err := db.PipelineRuns.StartRun("characterization sources", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.RunSources.Create(runID, "ieee", "csv", "ieee.csv", "", "", 0, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := db.RunSources.Create(runID, "scopus", "csv", "scopus.csv", "", "", 0, ""); err != nil {
			t.Fatal(err)
		}
		sources, err := db.RunSources.ListByRun(runID)
		if err != nil || len(sources) != 2 {
			t.Fatalf("sources=%d err=%v", len(sources), err)
		}
		if sources[0].SourceName != "ieee" || sources[1].SourceName != "scopus" {
			t.Fatalf("source order = [%q, %q], want insertion ID order", sources[0].SourceName, sources[1].SourceName)
		}
	})
	t.Run("source records order by record index", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		runID, _ := db.PipelineRuns.StartRun("characterization records", "")
		sourceID, err := db.RunSources.Create(runID, "scopus", "csv", "scopus.csv", "", "", 0, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, index := range []int{2, 0, 1} {
			if _, err := db.SourceRecords.Create(sourceID, index, "record", "hash"); err != nil {
				t.Fatal(err)
			}
		}
		records, err := db.SourceRecords.ListBySource(sourceID)
		if err != nil || len(records) != 3 {
			t.Fatalf("records=%d err=%v", len(records), err)
		}
		for index, record := range records {
			if record.RecordIndex != index {
				t.Fatalf("record order = [%d, %d, %d], want [0, 1, 2]", records[0].RecordIndex, records[1].RecordIndex, records[2].RecordIndex)
			}
		}
	})
	t.Run("metrics order by metric then source", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		runID, err := db.PipelineRuns.StartRun("characterization metrics", "")
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Metrics.Set(runID, "zeta", "", 1); err != nil {
			t.Fatal(err)
		}
		if err := db.Metrics.Set(runID, "alpha", "", 2); err != nil {
			t.Fatal(err)
		}
		metrics, err := db.Metrics.ListByRun(runID)
		if err != nil || len(metrics) != 2 {
			t.Fatalf("metrics=%d err=%v", len(metrics), err)
		}
		if metrics[0].Metric != "alpha" || metrics[1].Metric != "zeta" {
			t.Fatalf("metric order = [%q, %q], want [alpha, zeta]", metrics[0].Metric, metrics[1].Metric)
		}
	})
	t.Run("audit events order by id not occurrence time", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		runID, err := db.PipelineRuns.StartRun("characterization audit", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.AuditEvents.Insert(&manifest.AuditEvent{
			OccurredAt: "2026-01-02T00:00:00Z", Actor: "pipeline", PipelineRunID: runID,
			EntityType: "pipeline_run", EntityID: "1", Action: manifest.AuditRunCompleted,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.AuditEvents.Insert(&manifest.AuditEvent{
			OccurredAt: "2026-01-01T00:00:00Z", Actor: "pipeline", PipelineRunID: runID,
			EntityType: "pipeline_run", EntityID: "1", Action: manifest.AuditRunStarted,
		}); err != nil {
			t.Fatal(err)
		}
		events, err := db.AuditEvents.ListByRun(runID)
		if err != nil || len(events) != 2 {
			t.Fatalf("events=%d err=%v", len(events), err)
		}
		if events[0].Action != string(manifest.AuditRunCompleted) || events[1].Action != string(manifest.AuditRunStarted) {
			t.Fatalf("audit order = [%q, %q], want insertion ID order", events[0].Action, events[1].Action)
		}
	})
	t.Run("authorships order by author order", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		runID, err := db.PipelineRuns.StartRun("characterization authors", "")
		if err != nil {
			t.Fatal(err)
		}
		workID, err := db.Works.CreateByDOI("10.1000/characterization-authors")
		if err != nil {
			t.Fatal(err)
		}
		revisionID, err := db.WorkRevisions.Create(&WorkRevision{
			WorkID: workID, PipelineRunID: runID, ProducerStage: ProducerStageParse,
		})
		if err != nil {
			t.Fatal(err)
		}
		firstID, err := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "First Author"})
		if err != nil {
			t.Fatal(err)
		}
		secondID, err := db.AuthorOccs.Create(&AuthorOccurrence{CitationName: "Second Author"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Authorships.Create(&Authorship{
			WorkRevisionID: revisionID, AuthorOccurrenceID: secondID, AuthorOrder: 2,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Authorships.Create(&Authorship{
			WorkRevisionID: revisionID, AuthorOccurrenceID: firstID, AuthorOrder: 1,
		}); err != nil {
			t.Fatal(err)
		}
		authorships, err := db.Authorships.GetByRevisionID(revisionID)
		if err != nil || len(authorships) != 2 {
			t.Fatalf("authorships=%d err=%v", len(authorships), err)
		}
		if authorships[0].AuthorOrder != 1 || authorships[1].AuthorOrder != 2 {
			t.Fatalf("authorship order = [%d, %d], want [1, 2]", authorships[0].AuthorOrder, authorships[1].AuthorOrder)
		}
	})
	t.Run("reference mentions order by mention order", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		revisionID := createReferenceMentionTestRevision(t, db, "10.1000/characterization-mention-order")
		if _, err := db.ReferenceMentions.Create(&ReferenceMention{
			WorkRevisionID: revisionID, MentionOrder: 2, RawReference: "second",
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ReferenceMentions.Create(&ReferenceMention{
			WorkRevisionID: revisionID, MentionOrder: 1, RawReference: "first",
		}); err != nil {
			t.Fatal(err)
		}
		mentions, err := db.ReferenceMentions.GetByRevisionID(revisionID)
		if err != nil || len(mentions) != 2 {
			t.Fatalf("mentions=%d err=%v", len(mentions), err)
		}
		if mentions[0].RawReference != "first" || mentions[1].RawReference != "second" {
			t.Fatalf("mention order = [%q, %q], want mention_order order", mentions[0].RawReference, mentions[1].RawReference)
		}
	})
}

// TestCharacterizationReviewCompareAndSwapAndReplay verifies stale expected
// versions conflict, identical writes report no change, and heads move only
// through the affected-row compare-and-swap.
func TestCharacterizationReviewCompareAndSwapAndReplay(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	runID, _, _, revisionID, _ := createReviewLineageFixture(t, db)
	contextRecord, _, err := db.Reviews.CreateContext(ctx, runID, nil)
	if err != nil {
		t.Fatal(err)
	}

	state, changed, err := db.Reviews.AppendWorkReview(ctx, contextRecord.ID, revisionID, nil, "approved", nil, nil)
	if err != nil || !changed || state.Version == nil {
		t.Fatalf("first decision: state=%+v changed=%v err=%v", state, changed, err)
	}
	versionID := state.Version.ID
	replayed, changed, err := db.Reviews.AppendWorkReview(ctx, contextRecord.ID, revisionID, &versionID, "approved", nil, nil)
	if err != nil || changed || replayed.Version == nil || replayed.Version.ID != versionID {
		t.Fatalf("replayed decision: state=%+v changed=%v err=%v", replayed, changed, err)
	}
	stale := int64(999999)
	if _, _, err := db.Reviews.AppendWorkReview(ctx, contextRecord.ID, revisionID, &stale, "removed", []string{"duplicate"}, nil); !IsReviewConflict(err) {
		t.Fatalf("stale decision error = %v, want review conflict", err)
	}
	var versionCount int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM work_review_versions WHERE work_revision_id=?", revisionID).Scan(&versionCount); err != nil {
		t.Fatal(err)
	}
	if versionCount != 1 {
		t.Fatalf("work review versions = %d, want 1 after replay and conflict", versionCount)
	}

	note, err := db.Reviews.CreateNote(ctx, contextRecord.ID, revisionID, "Characterization note body")
	if err != nil {
		t.Fatal(err)
	}
	replayedNote, changed, err := db.Reviews.AppendNoteVersion(ctx, contextRecord.ID, note.ID, note.Version.ID, "active", "Characterization note body")
	if err != nil || changed || replayedNote.Version.ID != note.Version.ID {
		t.Fatalf("replayed note: note=%+v changed=%v err=%v", replayedNote, changed, err)
	}
	staleNoteVersion := note.Version.ID + 999
	if _, _, err := db.Reviews.AppendNoteVersion(ctx, contextRecord.ID, note.ID, staleNoteVersion, "active", "stale body"); !IsReviewConflict(err) {
		t.Fatalf("stale note error = %v, want review conflict", err)
	}

	hash := strings.Repeat("b", 64)
	anchor, err := db.Reviews.CreateAnchor(ctx, contextRecord.ID, revisionID, "characterization-anchor", hash, 1, "selected text",
		[]AnchorRectangle{{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.1}})
	if err != nil {
		t.Fatal(err)
	}
	replayedAnchor, changed, err := db.Reviews.AppendAnchorVersion(ctx, contextRecord.ID, anchor.ID, anchor.Version.ID,
		"active", hash, 1, "selected text", []AnchorRectangle{{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.1}})
	if err != nil || changed || replayedAnchor.Version.ID != anchor.Version.ID {
		t.Fatalf("replayed anchor: anchor=%+v changed=%v err=%v", replayedAnchor, changed, err)
	}
	if _, _, err := db.Reviews.AppendAnchorVersion(ctx, contextRecord.ID, anchor.ID, anchor.Version.ID+999,
		"deleted", hash, 0, "", nil); !IsReviewConflict(err) {
		t.Fatalf("stale anchor error = %v, want review conflict", err)
	}
	moved, changed, err := db.Reviews.AppendAnchorVersion(ctx, contextRecord.ID, anchor.ID, anchor.Version.ID,
		"active", hash, 2, "moved text", []AnchorRectangle{{X: 0.4, Y: 0.4, Width: 0.2, Height: 0.2}})
	if err != nil || !changed || moved.Version.ID == anchor.Version.ID {
		t.Fatalf("moved anchor: anchor=%+v changed=%v err=%v", moved, changed, err)
	}
	versions, err := db.Reviews.ListAnchorVersions(ctx, contextRecord.ID, anchor.ID, 0, 10)
	if err != nil || len(versions) != 2 || versions[0].ID != moved.Version.ID {
		t.Fatalf("anchor versions=%d err=%v", len(versions), err)
	}
}

// TestCharacterizationContextCancellation verifies repository reads and
// transaction-owned writes propagate a canceled context and leave no partial rows.
func TestCharacterizationContextCancellation(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	runID, secondRunID, _, revisionID, _ := createReviewLineageFixture(t, db)
	contextRecord, _, err := db.Reviews.CreateContext(context.Background(), runID, nil)
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	if _, _, err := db.Reviews.CreateContext(canceled, secondRunID, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled CreateContext error = %v, want context.Canceled", err)
	}
	var contexts int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM review_contexts WHERE pipeline_run_id=?", secondRunID).Scan(&contexts); err != nil {
		t.Fatal(err)
	}
	if contexts != 0 {
		t.Fatalf("canceled CreateContext wrote %d contexts", contexts)
	}
	if _, _, err := db.Reviews.AppendWorkReview(canceled, contextRecord.ID, revisionID, nil, "approved", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled AppendWorkReview error = %v, want context.Canceled", err)
	}
	var versions int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM work_review_versions").Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != 0 {
		t.Fatalf("canceled AppendWorkReview wrote %d versions", versions)
	}
	if _, err := db.Reviews.ListNotes(canceled, contextRecord.ID, revisionID, 0, 10, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled ListNotes error = %v, want context.Canceled", err)
	}
}

// TestCharacterizationTransactionRollback verifies a mid-transaction failure
// rolls back every earlier statement in the same review mutation.
func TestCharacterizationTransactionRollback(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	runID, secondRunID, _, revisionID, _ := createReviewLineageFixture(t, db)
	contextRecord, _, err := db.Reviews.CreateContext(ctx, runID, nil)
	if err != nil {
		t.Fatal(err)
	}
	var auditsBefore int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&auditsBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec(`CREATE TRIGGER characterization_abort_audit
		BEFORE INSERT ON audit_events
		BEGIN
			SELECT RAISE(ABORT, 'characterization injected audit failure');
		END`); err != nil {
		t.Fatal(err)
	}
	defer db.DB.Exec("DROP TRIGGER IF EXISTS characterization_abort_audit")

	if _, _, err := db.Reviews.AppendWorkReview(ctx, contextRecord.ID, revisionID, nil, "approved", nil, nil); err == nil || !strings.Contains(err.Error(), "characterization injected audit failure") {
		t.Fatalf("decision error = %v, want injected audit failure", err)
	}
	var versions, audits int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM work_review_versions").Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if versions != 0 || audits != auditsBefore {
		t.Fatalf("failed decision left versions=%d audits=%d, want versions=0 and audits=%d", versions, audits, auditsBefore)
	}
	if _, err := db.Reviews.CreateNote(ctx, contextRecord.ID, revisionID, "Rollback note body"); err == nil || !strings.Contains(err.Error(), "characterization injected audit failure") {
		t.Fatalf("note error = %v, want injected audit failure", err)
	}
	var notes, noteVersions, noteHeads int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM review_notes").Scan(&notes); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM review_note_versions").Scan(&noteVersions); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM review_context_note_heads").Scan(&noteHeads); err != nil {
		t.Fatal(err)
	}
	if notes != 0 || noteVersions != 0 || noteHeads != 0 {
		t.Fatalf("failed note left notes=%d versions=%d heads=%d, want rollback", notes, noteVersions, noteHeads)
	}
	if _, _, err := db.Reviews.CreateContext(ctx, secondRunID, nil); err == nil || !strings.Contains(err.Error(), "characterization injected audit failure") {
		t.Fatalf("context error = %v, want injected audit failure", err)
	}
	var contexts int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM review_contexts WHERE pipeline_run_id=?", secondRunID).Scan(&contexts); err != nil {
		t.Fatal(err)
	}
	if contexts != 0 {
		t.Fatalf("failed context left %d rows, want rollback", contexts)
	}
}

// TestCharacterizationCacheReplayIdentity verifies replay returns the exact
// immutable response recorded for a run even after a newer version exists.
func TestCharacterizationCacheReplayIdentity(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	runID, err := db.PipelineRuns.StartRun("characterization cache replay", "")
	if err != nil {
		t.Fatal(err)
	}
	artifactID, err := db.Artifacts.Create("characterization-cache-payload", "application/json", 4)
	if err != nil {
		t.Fatal(err)
	}
	firstID, err := db.CacheEntries.Upsert(&CacheEntry{
		Provider: "crossref", Namespace: "works", RequestFingerprint: "replay-key",
		ResponseStatus: 200, PayloadArtifactID: &artifactID,
		FetchedAt: "2026-01-01T00:00:00Z", ExtractorVersion: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.RunCacheUses.Create(&RunCacheUse{
		PipelineRunID: runID, CacheEntryID: firstID, CacheLayer: "global", Outcome: "hit",
	}); err != nil {
		t.Fatal(err)
	}
	secondID, err := db.CacheEntries.Upsert(&CacheEntry{
		Provider: "crossref", Namespace: "works", RequestFingerprint: "replay-key",
		ResponseStatus: 404, FetchedAt: "2026-01-02T00:00:00Z", ExtractorVersion: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if secondID == firstID {
		t.Fatal("new cache version reused the immutable response ID")
	}
	for attempt := 0; attempt < 2; attempt++ {
		recorded, err := db.RunCacheUses.FindEntry(runID, "global", "crossref", "works", "replay-key", "1")
		if err != nil || recorded == nil || recorded.ID != firstID || recorded.ResponseStatus != 200 {
			t.Fatalf("replayed FindEntry = %+v err=%v, want original entry %d", recorded, err, firstID)
		}
		anyEntry, err := db.RunCacheUses.FindAnyEntry(runID, "crossref", "works", "replay-key", "1")
		if err != nil || anyEntry == nil || anyEntry.ID != firstID {
			t.Fatalf("replayed FindAnyEntry = %+v err=%v, want original entry %d", anyEntry, err, firstID)
		}
	}
	latest, err := db.CacheEntries.Get("crossref", "works", "replay-key", "1")
	if err != nil || latest == nil || latest.ID != secondID || latest.ResponseStatus != 404 {
		t.Fatalf("latest cache entry = %+v err=%v, want %d", latest, err, secondID)
	}
	uses, err := db.RunCacheUses.ListByRun(runID)
	if err != nil || len(uses) != 1 || uses[0].CacheEntryID != firstID {
		t.Fatalf("run cache uses = %+v err=%v, want one original use", uses, err)
	}
}
