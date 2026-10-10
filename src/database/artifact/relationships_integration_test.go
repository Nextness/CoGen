// relationships_integration_test.go exercises artifact-primary relationship
// reads across run roles, step inputs and outputs, cache payloads, and
// identity-candidate payloads against a production-migrated database.
//go:build integration

package artifact_test

import (
	"context"
	"testing"

	"analysis/database"
	"analysis/database/artifact"
	"analysis/database/cache"
	"analysis/database/run"
)

// TestStoreListRunArtifactsCoversEveryRelationshipKind verifies the family
// returns artifact application types joined from other families.
func TestStoreListRunArtifactsCoversEveryRelationshipKind(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "artifact-relationships")

	runRoleID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "relationship-run-role", ContentType: "application/json", ByteSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Link(ctx, artifact.LinkInput{PipelineRunID: runID, ArtifactID: runRoleID, Role: artifact.RunArtifactWorkspaceConfig}); err != nil {
		t.Fatal(err)
	}

	stepInputID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "relationship-step-input", ContentType: "application/json", ByteSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	stepOutputID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "relationship-step-output", ContentType: "application/json", ByteSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	stepID, err := db.Run.CreateStep(ctx, runID, "enrich")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Run.LinkStepInputArtifact(ctx, run.StepArtifactInput{StepID: stepID, ArtifactID: stepInputID}); err != nil {
		t.Fatal(err)
	}
	if err := db.Run.LinkStepOutputArtifact(ctx, run.StepArtifactInput{StepID: stepID, ArtifactID: stepOutputID}); err != nil {
		t.Fatal(err)
	}

	cachePayloadID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "relationship-cache-payload", ContentType: "application/json", ByteSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	entryID, err := db.Cache.AppendEntry(ctx, &cache.Entry{
		Provider: "crossref", Namespace: "works", RequestFingerprint: "fingerprint",
		ResponseStatus: 200, PayloadArtifactID: &cachePayloadID, FetchedAt: "2026-01-02T03:04:05Z", ExtractorVersion: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Cache.AppendUse(ctx, &cache.Use{PipelineRunID: runID, CacheEntryID: entryID, CacheLayer: "global", Outcome: "hit"}); err != nil {
		t.Fatal(err)
	}

	candidatePayloadID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "relationship-candidate-payload", ContentType: "application/json", ByteSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	occurrenceID, err := db.AuthorOccs.Create(&database.AuthorOccurrence{CitationName: "Test Author"})
	if err != nil {
		t.Fatal(err)
	}
	resolutionID, err := db.IdentityResolutions.Create(&database.AuthorIdentityResolution{
		PipelineRunID: runID, AuthorOccurrenceID: occurrenceID, Status: database.AuthorIdentityStatusORCIDUnclear,
		Provider: "orcid", QueriedCitationName: "Test Author", ResolvedAt: "2026-01-02T03:04:05Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.IdentityCandidates.Create(&database.AuthorIdentityCandidate{
		IdentityResolutionID: resolutionID, CandidateORCID: "0000-0002-1825-0097",
		QueryURL: "https://orcid.org/0000-0002-1825-0097", PayloadArtifactID: candidatePayloadID, ProviderRank: 1,
	}); err != nil {
		t.Fatal(err)
	}

	artifacts, err := store.ListRunArtifacts(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	want := []artifact.RelatedArtifact{
		{Artifact: artifact.Artifact{ID: runRoleID, ContentHash: "relationship-run-role", ByteSize: 1, ContentType: "application/json"}, Role: "run_role", Detail: artifact.RunArtifactWorkspaceConfig},
		{Artifact: artifact.Artifact{ID: stepInputID, ContentHash: "relationship-step-input", ByteSize: 1, ContentType: "application/json"}, Role: "step_input", Detail: "enrich"},
		{Artifact: artifact.Artifact{ID: stepOutputID, ContentHash: "relationship-step-output", ByteSize: 1, ContentType: "application/json"}, Role: "step_output", Detail: "enrich"},
		{Artifact: artifact.Artifact{ID: cachePayloadID, ContentHash: "relationship-cache-payload", ByteSize: 1, ContentType: "application/json"}, Role: "cache_payload", Detail: "crossref:works"},
		{Artifact: artifact.Artifact{ID: candidatePayloadID, ContentHash: "relationship-candidate-payload", ByteSize: 1, ContentType: "application/json"}, Role: "identity_candidate_payload", Detail: "orcid"},
	}
	if len(artifacts) != len(want) {
		t.Fatalf("artifact count = %d, want %d: %+v", len(artifacts), len(want), artifacts)
	}
	for index := range want {
		got := artifacts[index]
		if got.ID != want[index].ID || got.ContentHash != want[index].ContentHash ||
			got.ByteSize != want[index].ByteSize || got.ContentType != want[index].ContentType ||
			got.Role != want[index].Role || got.Detail != want[index].Detail || got.CreatedAt == "" {
			t.Fatalf("artifact %d = %+v, want %+v", index, got, want[index])
		}
	}

	empty, err := store.ListRunArtifacts(ctx, 99999)
	if err != nil || empty != nil {
		t.Fatalf("empty artifact list = %+v err=%v, want nil without error", empty, err)
	}
}
