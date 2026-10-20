// characterization_integration_test.go pins database-layer contracts that the
// type-safe SQL migration must preserve. The critical operations marked for
// direct old-versus-new comparison are recorded in PARITY.md.
//go:build integration

package database

import (
	"context"
	"testing"

	"analysis/database/artifact"
	"analysis/database/author"
	"analysis/database/cache"
	"analysis/database/run"
	"analysis/database/search"
	"analysis/database/source"
	"analysis/database/work"
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
		{"SearchStore.GetSearchByID", func() (bool, error) { value, err := db.Search.GetSearchByID(ctx, 999); return value == nil, err }},
		{"SearchStore.GetSearchBySearchID", func() (bool, error) {
			value, err := db.Search.GetSearchBySearchID(ctx, "absent")
			return value == nil, err
		}},
		{"SearchStore.GetRevisionByID", func() (bool, error) { value, err := db.Search.GetRevisionByID(ctx, 999); return value == nil, err }},
		{"SearchStore.GetRevisionBySearchAndLabel", func() (bool, error) {
			value, err := db.Search.GetRevisionBySearchAndLabel(ctx, 999, "absent")
			return value == nil, err
		}},
		{"SearchStore.GetPlanByID", func() (bool, error) { value, err := db.Search.GetPlanByID(ctx, 999); return value == nil, err }},
		{"SearchStore.GetPlanByFingerprint", func() (bool, error) {
			value, err := db.Search.GetPlanByFingerprint(ctx, 999, "absent")
			return value == nil, err
		}},
		{"RunStore.GetByID", func() (bool, error) { value, err := db.Run.GetByID(ctx, 999); return value == nil, err }},
		{"RunStore.GetReviewer", func() (bool, error) { value, err := db.Run.GetReviewer(ctx, 999); return value == nil, err }},
		{"RunStore.GetMetric", func() (bool, error) { value, err := db.Run.GetMetric(ctx, 999, "absent", ""); return value == nil, err }},
		{"SourceStore.GetFilterCount", func() (bool, error) {
			value, err := db.Source.GetFilterCount(ctx, 999, "absent")
			return value == nil, err
		}},
		{"ArtifactStore.GetByID", func() (bool, error) { value, err := db.Artifact.GetByID(ctx, 999); return value == nil, err }},
		{"ArtifactStore.GetByHash", func() (bool, error) { value, err := db.Artifact.GetByHash(ctx, "absent"); return value == nil, err }},
		{"ArtifactStore.GetBlobByArtifactID", func() (bool, error) {
			value, err := db.Artifact.GetBlobByArtifactID(ctx, 999)
			return value == nil, err
		}},
		{"CacheStore.LatestEntry", func() (bool, error) {
			value, err := db.Cache.LatestEntry(ctx, cache.Key{Provider: "absent", Namespace: "absent", RequestFingerprint: "absent", ExtractorVersion: "absent"})
			return value == nil, err
		}},
		{"CacheStore.GlobalEntry", func() (bool, error) {
			value, err := db.Cache.GlobalEntry(ctx, cache.Key{Provider: "absent", Namespace: "absent", RequestFingerprint: "absent", ExtractorVersion: "absent"})
			return value == nil, err
		}},
		{"CacheStore.FindEntry", func() (bool, error) {
			value, err := db.Cache.FindEntry(ctx, 999, "global", cache.Key{Provider: "absent", Namespace: "absent", RequestFingerprint: "absent", ExtractorVersion: "absent"})
			return value == nil, err
		}},
		{"CacheStore.FindAnyEntry", func() (bool, error) {
			value, err := db.Cache.FindAnyEntry(ctx, 999, cache.Key{Provider: "absent", Namespace: "absent", RequestFingerprint: "absent", ExtractorVersion: "absent"})
			return value == nil, err
		}},
		{"WorkStore.GetWorkByID", func() (bool, error) { value, err := db.Work.GetWorkByID(ctx, 999); return value == nil, err }},
		{"WorkStore.GetWorkByDOI", func() (bool, error) {
			value, err := db.Work.GetWorkByDOI(ctx, "10.1000/absent")
			return value == nil, err
		}},
		{"WorkStore.GetIdentifierByID", func() (bool, error) { value, err := db.Work.GetIdentifierByID(ctx, 999); return value == nil, err }},
		{"WorkStore.GetIdentifierByNamespaceAndIdentifier", func() (bool, error) {
			value, err := db.Work.GetIdentifierByNamespaceAndIdentifier(ctx, "absent", "absent")
			return value == nil, err
		}},
		{"WorkStore.GetRevisionByID", func() (bool, error) { value, err := db.Work.GetRevisionByID(ctx, 999); return value == nil, err }},
		{"WorkStore.GetStageOutcome", func() (bool, error) {
			value, err := db.Work.GetStageOutcome(ctx, 999, 999, "validate")
			return value == nil, err
		}},
		{"AuthorStore.GetPersonByID", func() (bool, error) { value, err := db.Author.GetPersonByID(ctx, 999); return value == nil, err }},
		{"AuthorStore.GetPersonByORCID", func() (bool, error) {
			value, err := db.Author.GetPersonByORCID(ctx, "0000-0002-1825-0097")
			return value == nil, err
		}},
		{"AuthorStore.GetOccurrenceByID", func() (bool, error) {
			value, err := db.Author.GetOccurrenceByID(ctx, 999)
			return value == nil, err
		}},
		{"WorkStore.GetReferenceByID", func() (bool, error) { value, err := db.Work.GetReferenceByID(ctx, 999); return value == nil, err }},
		{"ReviewStore.GetContextByRun", func() (bool, error) { value, err := db.Review.GetContextByRun(ctx, 999); return value == nil, err }},
		{"ReviewStore.GetNote", func() (bool, error) { value, err := db.Review.GetNote(ctx, 999, 999); return value == nil, err }},
		{"ReviewStore.GetAnchor", func() (bool, error) { value, err := db.Review.GetAnchor(ctx, 999, "absent"); return value == nil, err }},
		{"ReviewStore.GetWorkReviewVersion", func() (bool, error) {
			value, err := db.Review.GetWorkReviewVersion(ctx, 999, 999, 999)
			return value == nil, err
		}},
		{"ReviewStore.GetNoteVersion", func() (bool, error) {
			value, err := db.Review.GetNoteVersion(ctx, 999, 999, 999)
			return value == nil, err
		}},
		{"ReviewStore.GetAnchorVersion", func() (bool, error) {
			value, err := db.Review.GetAnchorVersion(ctx, 999, "absent", 999)
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
		{"SearchStore.ListSearches", true, func() (bool, error) { value, err := db.Search.ListSearches(ctx); return value == nil, err }},
		{"SearchStore.ListRevisionsBySearch", true, func() (bool, error) {
			value, err := db.Search.ListRevisionsBySearch(ctx, 999)
			return value == nil, err
		}},
		{"SearchStore.ListPlansBySearchRevision", true, func() (bool, error) {
			value, err := db.Search.ListPlansBySearchRevision(ctx, 999)
			return value == nil, err
		}},
		{"SourceStore.ListSourcesByRun", true, func() (bool, error) {
			value, err := db.Source.ListSourcesByRun(ctx, 999)
			return value == nil, err
		}},
		{"SourceStore.ListRecordsBySource", true, func() (bool, error) {
			value, err := db.Source.ListRecordsBySource(ctx, 999)
			return value == nil, err
		}},
		{"RunStore.ListStepsByRun", true, func() (bool, error) { value, err := db.Run.ListStepsByRun(ctx, 999); return value == nil, err }},
		{"SourceStore.ListFilterCountsByRun", true, func() (bool, error) {
			value, err := db.Source.ListFilterCountsByRun(ctx, 999)
			return value == nil, err
		}},
		{"RunStore.ListByPlan", true, func() (bool, error) { value, err := db.Run.ListByPlan(ctx, 999); return value == nil, err }},
		{"RunStore.ListByVisibility", true, func() (bool, error) {
			value, err := db.Run.ListByVisibility(ctx, "trashed")
			return value == nil, err
		}},
		{"RunStore.ListMetricsByRun", true, func() (bool, error) { value, err := db.Run.ListMetricsByRun(ctx, 999); return value == nil, err }},
		{"AuditStore.ListByRun", true, func() (bool, error) { value, err := db.Audit.ListByRun(ctx, 999); return value == nil, err }},
		{"AuditStore.ListByEntity", true, func() (bool, error) {
			value, err := db.Audit.ListByEntity(ctx, "absent", "absent")
			return value == nil, err
		}},
		{"WorkStore.ListWorksByIDs nil", true, func() (bool, error) { value, err := db.Work.ListWorksByIDs(ctx, nil); return value == nil, err }},
		{"WorkStore.ListWorksByIDs absent", true, func() (bool, error) {
			value, err := db.Work.ListWorksByIDs(ctx, []int64{999})
			return value == nil, err
		}},
		{"WorkStore.ListIdentifiersByWorkID", true, func() (bool, error) {
			value, err := db.Work.ListIdentifiersByWorkID(ctx, 999)
			return value == nil, err
		}},
		{"WorkStore.ListRevisionsByWorkID", true, func() (bool, error) {
			value, err := db.Work.ListRevisionsByWorkID(ctx, 999)
			return value == nil, err
		}},
		{"WorkStore.ListRevisionsByRunID", true, func() (bool, error) {
			value, err := db.Work.ListRevisionsByRunID(ctx, 999)
			return value == nil, err
		}},
		{"WorkStore.ListStageOutcomesByRun", true, func() (bool, error) {
			value, err := db.Work.ListStageOutcomesByRun(ctx, 999)
			return value == nil, err
		}},
		{"WorkStore.ListStageOutcomesByWork", true, func() (bool, error) {
			value, err := db.Work.ListStageOutcomesByWork(ctx, 999)
			return value == nil, err
		}},
		{"AuthorStore.ListOccurrencesByPersonID", true, func() (bool, error) {
			value, err := db.Author.ListOccurrencesByPersonID(ctx, 999)
			return value == nil, err
		}},
		{"AuthorStore.ListAuthorshipsByRevisionID", true, func() (bool, error) {
			value, err := db.Author.ListAuthorshipsByRevisionID(ctx, 999)
			return value == nil, err
		}},
		{"AuthorStore.ListAuthorshipsByOccurrenceID", true, func() (bool, error) {
			value, err := db.Author.ListAuthorshipsByOccurrenceID(ctx, 999)
			return value == nil, err
		}},
		{"WorkStore.ListRunTerms", true, func() (bool, error) { value, err := db.Work.ListRunTerms(ctx, 999); return value == nil, err }},
		{"CacheStore.ListUsesByRun", false, func() (bool, error) { value, err := db.Cache.ListUsesByRun(ctx, 999); return value == nil, err }},
		{"WorkStore.ListReferencesByRevisionID", false, func() (bool, error) {
			value, err := db.Work.ListReferencesByRevisionID(ctx, 999)
			return value == nil, err
		}},
		{"WorkStore.ListReferencesByResolvedWorkID", false, func() (bool, error) {
			value, err := db.Work.ListReferencesByResolvedWorkID(ctx, 999)
			return value == nil, err
		}},
		{"ReviewStore.ListNotes", false, func() (bool, error) {
			value, err := db.Review.ListNotes(ctx, 999, 999, 0, 10, false)
			return value == nil, err
		}},
		{"ReviewStore.ListNoteVersions", false, func() (bool, error) {
			value, err := db.Review.ListNoteVersions(ctx, 999, 999, 0, 10)
			return value == nil, err
		}},
		{"ReviewStore.ListWorkReviewVersions", false, func() (bool, error) {
			value, err := db.Review.ListWorkReviewVersions(ctx, 999, 999, 0, 10)
			return value == nil, err
		}},
		{"ReviewStore.ListAnchors", false, func() (bool, error) {
			value, err := db.Review.ListAnchors(ctx, 999, 999, "", 10)
			return value == nil, err
		}},
		{"ReviewStore.ListAnchorVersions", false, func() (bool, error) {
			value, err := db.Review.ListAnchorVersions(ctx, 999, "absent", 0, 10)
			return value == nil, err
		}},
		{"ReviewStore.ListBacklinks", false, func() (bool, error) {
			value, err := db.Review.ListBacklinks(ctx, 999, "pdf_page", "1", 0, 0, 10)
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
	ctx := context.Background()
	t.Run("run source optional fields", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		runID, err := db.Run.StartRun(ctx, "characterization nullable", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Source.CreateSource(ctx, source.CreateSourceInput{
			RunID: runID, Name: "scopus", Type: "csv", ExpectedFile: "scopus.csv",
		}); err != nil {
			t.Fatal(err)
		}
		sources, err := db.Source.ListSourcesByRun(ctx, runID)
		if err != nil || len(sources) != 1 {
			t.Fatalf("sources=%d err=%v", len(sources), err)
		}
		found := sources[0]
		if found.Query != "" || found.RequestedFields != "" || found.ExportDate != "" || found.ResultCountComparison != "" {
			t.Fatalf("optional text = %+v, want empty values", found)
		}
		if found.ExpectedResultCount == nil || *found.ExpectedResultCount != 0 {
			t.Fatalf("expected result count = %v, want pointer to zero", found.ExpectedResultCount)
		}
		if found.ObservedResultCount != nil {
			t.Fatalf("observed result count = %v, want nil", found.ObservedResultCount)
		}
	})
	t.Run("cache entry payload and expiry", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		if _, err := db.Cache.AppendEntry(ctx, &cache.Entry{
			Provider: "crossref", Namespace: "works", RequestFingerprint: "nullable-negative",
			ResponseStatus: 404, FetchedAt: "2026-01-01T00:00:00Z", ExtractorVersion: "1",
		}); err != nil {
			t.Fatal(err)
		}
		negative, err := db.Cache.LatestEntry(ctx, cache.Key{Provider: "crossref", Namespace: "works", RequestFingerprint: "nullable-negative", ExtractorVersion: "1"})
		if err != nil || negative == nil {
			t.Fatalf("negative entry=%+v err=%v", negative, err)
		}
		if negative.PayloadArtifactID != nil || negative.ExpiresAt != "" {
			t.Fatalf("negative entry nullable fields = %+v", negative)
		}
		artifactID, err := db.Artifact.Create(ctx, artifact.CreateInput{ContentHash: "nullable-cache-payload", ContentType: "application/json", ByteSize: 4})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Cache.AppendEntry(ctx, &cache.Entry{
			Provider: "crossref", Namespace: "works", RequestFingerprint: "nullable-positive",
			ResponseStatus: 200, PayloadArtifactID: &artifactID, FetchedAt: "2026-01-01T00:00:00Z",
			ExpiresAt: "2026-02-01T00:00:00Z", ExtractorVersion: "1",
		}); err != nil {
			t.Fatal(err)
		}
		positive, err := db.Cache.LatestEntry(ctx, cache.Key{Provider: "crossref", Namespace: "works", RequestFingerprint: "nullable-positive", ExtractorVersion: "1"})
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
		runID, err := db.Run.StartRun(ctx, "characterization revision", "")
		if err != nil {
			t.Fatal(err)
		}
		workID, err := db.Work.CreateWorkByDOI(ctx, "10.1000/characterization-revision")
		if err != nil {
			t.Fatal(err)
		}
		revisionID, err := db.Work.CreateRevision(ctx, &work.Revision{
			WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageParse,
		})
		if err != nil {
			t.Fatal(err)
		}
		revision, err := db.Work.GetRevisionByID(ctx, revisionID)
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
		mentionID, err := db.Work.CreateReference(ctx, &work.Reference{
			WorkRevisionID: revisionID, MentionOrder: 1, RawReference: "raw reference",
		})
		if err != nil {
			t.Fatal(err)
		}
		mention, err := db.Work.GetReferenceByID(ctx, mentionID)
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
		if _, err := db.Audit.Insert(ctx, &manifest.AuditEvent{
			OccurredAt: "2026-01-01T00:00:00Z", Actor: "pipeline",
			EntityType: "work", EntityID: "1", Action: manifest.AuditFieldEnriched,
		}); err != nil {
			t.Fatal(err)
		}
		events, err := db.Audit.ListByEntity(ctx, "work", "1")
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
	ctx := context.Background()
	t.Run("search revisions order by id", func(t *testing.T) {
		db := openTestDB(t)
		defer db.Close()
		searchID, err := db.Search.FindOrCreateSearch(ctx, "characterization ordering")
		if err != nil {
			t.Fatal(err)
		}
		zetaID, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
			SearchID: searchID, RevisionLabel: "zeta", ConfigArtifactHash: "config-z", ResolvedManifestHash: "manifest-z",
		})
		if err != nil {
			t.Fatal(err)
		}
		alphaID, _, err := db.Search.UpsertRevision(ctx, search.RevisionInput{
			SearchID: searchID, RevisionLabel: "alpha", ConfigArtifactHash: "config-a", ResolvedManifestHash: "manifest-a",
		})
		if err != nil {
			t.Fatal(err)
		}
		revisions, err := db.Search.ListRevisionsBySearch(ctx, searchID)
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
		searchID, _ := db.Search.FindOrCreateSearch(ctx, "characterization plans")
		revisionID, _, _ := db.Search.UpsertRevision(ctx, search.RevisionInput{SearchID: searchID, RevisionLabel: "r1", ConfigArtifactHash: "config", ResolvedManifestHash: "manifest"})
		zetaID, err := db.Search.FindOrCreatePlan(ctx, search.PlanInput{SearchRevisionID: revisionID, ExecutionFingerprint: "fp-zeta", ResolvedManifestHash: "manifest-z"})
		if err != nil {
			t.Fatal(err)
		}
		alphaID, err := db.Search.FindOrCreatePlan(ctx, search.PlanInput{SearchRevisionID: revisionID, ExecutionFingerprint: "fp-alpha", ResolvedManifestHash: "manifest-a"})
		if err != nil {
			t.Fatal(err)
		}
		plans, err := db.Search.ListPlansBySearchRevision(ctx, revisionID)
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
		runID, err := db.Run.StartRun(ctx, "characterization sources", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Source.CreateSource(ctx, source.CreateSourceInput{RunID: runID, Name: "ieee", Type: "csv", ExpectedFile: "ieee.csv"}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Source.CreateSource(ctx, source.CreateSourceInput{RunID: runID, Name: "scopus", Type: "csv", ExpectedFile: "scopus.csv"}); err != nil {
			t.Fatal(err)
		}
		sources, err := db.Source.ListSourcesByRun(ctx, runID)
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
		runID, _ := db.Run.StartRun(ctx, "characterization records", "")
		sourceID, err := db.Source.CreateSource(ctx, source.CreateSourceInput{RunID: runID, Name: "scopus", Type: "csv", ExpectedFile: "scopus.csv"})
		if err != nil {
			t.Fatal(err)
		}
		for _, index := range []int{2, 0, 1} {
			if _, err := db.Source.CreateRecord(ctx, source.CreateRecordInput{RunSourceID: sourceID, RecordIndex: index, RawPayload: "record", ContentHash: "hash"}); err != nil {
				t.Fatal(err)
			}
		}
		records, err := db.Source.ListRecordsBySource(ctx, sourceID)
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
		runID, err := db.Run.StartRun(ctx, "characterization metrics", "")
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Run.SetMetric(ctx, run.MetricInput{RunID: runID, Metric: "zeta", Value: 1}); err != nil {
			t.Fatal(err)
		}
		if err := db.Run.SetMetric(ctx, run.MetricInput{RunID: runID, Metric: "alpha", Value: 2}); err != nil {
			t.Fatal(err)
		}
		metrics, err := db.Run.ListMetricsByRun(ctx, runID)
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
		runID, err := db.Run.StartRun(ctx, "characterization audit", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Audit.Insert(ctx, &manifest.AuditEvent{
			OccurredAt: "2026-01-02T00:00:00Z", Actor: "pipeline", PipelineRunID: runID,
			EntityType: "pipeline_run", EntityID: "1", Action: manifest.AuditRunCompleted,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Audit.Insert(ctx, &manifest.AuditEvent{
			OccurredAt: "2026-01-01T00:00:00Z", Actor: "pipeline", PipelineRunID: runID,
			EntityType: "pipeline_run", EntityID: "1", Action: manifest.AuditRunStarted,
		}); err != nil {
			t.Fatal(err)
		}
		events, err := db.Audit.ListByRun(ctx, runID)
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
		runID, err := db.Run.StartRun(ctx, "characterization authors", "")
		if err != nil {
			t.Fatal(err)
		}
		workID, err := db.Work.CreateWorkByDOI(ctx, "10.1000/characterization-authors")
		if err != nil {
			t.Fatal(err)
		}
		revisionID, err := db.Work.CreateRevision(ctx, &work.Revision{
			WorkID: workID, PipelineRunID: runID, ProducerStage: work.ProducerStageParse,
		})
		if err != nil {
			t.Fatal(err)
		}
		firstID, err := db.Author.CreateOccurrence(ctx, &author.Occurrence{CitationName: "First Author"})
		if err != nil {
			t.Fatal(err)
		}
		secondID, err := db.Author.CreateOccurrence(ctx, &author.Occurrence{CitationName: "Second Author"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Author.CreateAuthorship(ctx, &author.Authorship{
			WorkRevisionID: revisionID, AuthorOccurrenceID: secondID, AuthorOrder: 2,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Author.CreateAuthorship(ctx, &author.Authorship{
			WorkRevisionID: revisionID, AuthorOccurrenceID: firstID, AuthorOrder: 1,
		}); err != nil {
			t.Fatal(err)
		}
		authorships, err := db.Author.ListAuthorshipsByRevisionID(ctx, revisionID)
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
		if _, err := db.Work.CreateReference(ctx, &work.Reference{
			WorkRevisionID: revisionID, MentionOrder: 2, RawReference: "second",
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Work.CreateReference(ctx, &work.Reference{
			WorkRevisionID: revisionID, MentionOrder: 1, RawReference: "first",
		}); err != nil {
			t.Fatal(err)
		}
		mentions, err := db.Work.ListReferencesByRevisionID(ctx, revisionID)
		if err != nil || len(mentions) != 2 {
			t.Fatalf("mentions=%d err=%v", len(mentions), err)
		}
		if mentions[0].RawReference != "first" || mentions[1].RawReference != "second" {
			t.Fatalf("mention order = [%q, %q], want mention_order order", mentions[0].RawReference, mentions[1].RawReference)
		}
	})
}

// TestCharacterizationCacheReplayIdentity verifies replay returns the exact
// immutable response recorded for a run even after a newer version exists.
func TestCharacterizationCacheReplayIdentity(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()
	runID, err := db.Run.StartRun(ctx, "characterization cache replay", "")
	if err != nil {
		t.Fatal(err)
	}
	artifactID, err := db.Artifact.Create(ctx, artifact.CreateInput{ContentHash: "characterization-cache-payload", ContentType: "application/json", ByteSize: 4})
	if err != nil {
		t.Fatal(err)
	}
	firstID, err := db.Cache.AppendEntry(ctx, &cache.Entry{
		Provider: "crossref", Namespace: "works", RequestFingerprint: "replay-key",
		ResponseStatus: 200, PayloadArtifactID: &artifactID,
		FetchedAt: "2026-01-01T00:00:00Z", ExtractorVersion: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Cache.AppendUse(ctx, &cache.Use{
		PipelineRunID: runID, CacheEntryID: firstID, CacheLayer: "global", Outcome: "hit",
	}); err != nil {
		t.Fatal(err)
	}
	secondID, err := db.Cache.AppendEntry(ctx, &cache.Entry{
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
		recorded, err := db.Cache.FindEntry(ctx, runID, "global", cache.Key{Provider: "crossref", Namespace: "works", RequestFingerprint: "replay-key", ExtractorVersion: "1"})
		if err != nil || recorded == nil || recorded.ID != firstID || recorded.ResponseStatus != 200 {
			t.Fatalf("replayed FindEntry = %+v err=%v, want original entry %d", recorded, err, firstID)
		}
		anyEntry, err := db.Cache.FindAnyEntry(ctx, runID, cache.Key{Provider: "crossref", Namespace: "works", RequestFingerprint: "replay-key", ExtractorVersion: "1"})
		if err != nil || anyEntry == nil || anyEntry.ID != firstID {
			t.Fatalf("replayed FindAnyEntry = %+v err=%v, want original entry %d", anyEntry, err, firstID)
		}
	}
	latest, err := db.Cache.LatestEntry(ctx, cache.Key{Provider: "crossref", Namespace: "works", RequestFingerprint: "replay-key", ExtractorVersion: "1"})
	if err != nil || latest == nil || latest.ID != secondID || latest.ResponseStatus != 404 {
		t.Fatalf("latest cache entry = %+v err=%v, want %d", latest, err, secondID)
	}
	uses, err := db.Cache.ListUsesByRun(ctx, runID)
	if err != nil || len(uses) != 1 || uses[0].CacheEntryID != firstID {
		t.Fatalf("run cache uses = %+v err=%v, want one original use", uses, err)
	}
}
