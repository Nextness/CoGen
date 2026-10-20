// plans_integration_test.go exercises the execution plan store against a
// database created by the production migration runner.
//go:build integration

package search_test

import (
	"context"
	"testing"

	"analysis/database/search"
)

// createTestRevision creates one search and revision pair and returns the revision ID.
func createTestRevision(t *testing.T, store *search.Store, label string) int64 {
	t.Helper()
	searchID := createTestSearch(t, store, "plan-search-"+label)
	revisionID, _, err := store.UpsertRevision(context.Background(), search.RevisionInput{
		SearchID: searchID, RevisionLabel: label,
		ConfigArtifactHash: "config-" + label, ResolvedManifestHash: "manifest-" + label,
	})
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}
	return revisionID
}

// TestStoreFindOrCreatePlanPreservesFingerprintChecks verifies identical fingerprints reuse one ID while differing manifests and inputs are rejected.
func TestStoreFindOrCreatePlanPreservesFingerprintChecks(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()
	revisionID := createTestRevision(t, store, "v1")

	planID, err := store.FindOrCreatePlan(ctx, search.PlanInput{
		SearchRevisionID: revisionID, ExecutionFingerprint: "fp-abc123", ResolvedManifestHash: "manifest-xyz",
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if planID == 0 {
		t.Fatal("expected non-zero plan id")
	}

	got, err := store.GetPlanByID(ctx, planID)
	if err != nil || got == nil {
		t.Fatalf("get by id = %+v err=%v", got, err)
	}
	if got.ExecutionFingerprint != "fp-abc123" || got.ResolvedManifestHash != "manifest-xyz" || got.EnrichmentEnabled {
		t.Fatalf("plan by id = %+v", got)
	}
	byFingerprint, err := store.GetPlanByFingerprint(ctx, revisionID, "fp-abc123")
	if err != nil || byFingerprint == nil || byFingerprint.ID != planID {
		t.Fatalf("get by fingerprint = %+v err=%v", byFingerprint, err)
	}

	reusedID, err := store.FindOrCreatePlan(ctx, search.PlanInput{
		SearchRevisionID: revisionID, ExecutionFingerprint: "fp-abc123", ResolvedManifestHash: "manifest-xyz",
	})
	if err != nil {
		t.Fatalf("identical plan reuse: %v", err)
	}
	if reusedID != planID {
		t.Fatalf("identical plan reuse returned %d, want %d", reusedID, planID)
	}
	if _, err := store.FindOrCreatePlan(ctx, search.PlanInput{
		SearchRevisionID: revisionID, ExecutionFingerprint: "fp-abc123", ResolvedManifestHash: "manifest-other",
	}); err == nil {
		t.Fatal("duplicate fingerprint with a different manifest hash was accepted")
	}
}

// TestStoreFindOrCreatePlanWithInputManifestPreservesPolicy verifies the frozen input manifest and enrichment policy are stored and compared.
func TestStoreFindOrCreatePlanWithInputManifestPreservesPolicy(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()
	revisionID := createTestRevision(t, store, "v2")

	planID, err := store.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
		SearchRevisionID: revisionID, ExecutionFingerprint: "fp-policy",
		ResolvedManifestHash: "manifest", InputManifestHash: "input-manifest-hash", EnrichmentEnabled: true,
	})
	if err != nil {
		t.Fatalf("create policy plan: %v", err)
	}
	plan, err := store.GetPlanByID(ctx, planID)
	if err != nil || plan == nil {
		t.Fatalf("get policy plan = %+v err=%v", plan, err)
	}
	if plan.InputManifestHash != "input-manifest-hash" || !plan.EnrichmentEnabled {
		t.Fatalf("policy plan = %+v", plan)
	}

	reusedID, err := store.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
		SearchRevisionID: revisionID, ExecutionFingerprint: "fp-policy",
		ResolvedManifestHash: "manifest", InputManifestHash: "input-manifest-hash", EnrichmentEnabled: true,
	})
	if err != nil || reusedID != planID {
		t.Fatalf("identical policy plan returned %d err=%v, want %d", reusedID, err, planID)
	}
	if _, err := store.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
		SearchRevisionID: revisionID, ExecutionFingerprint: "fp-policy",
		ResolvedManifestHash: "manifest", InputManifestHash: "input-manifest-hash", EnrichmentEnabled: false,
	}); err == nil {
		t.Fatal("duplicate fingerprint with a different enrichment policy was accepted")
	}
	if _, err := store.FindOrCreatePlanWithInputManifest(ctx, search.PlanInput{
		SearchRevisionID: revisionID, ExecutionFingerprint: "fp-missing-input", ResolvedManifestHash: "manifest",
	}); err == nil {
		t.Fatal("missing input manifest hash was accepted")
	}
}

// TestStorePlansOrderAndEmptyIdentity verifies distinct revisions may share a fingerprint, lists follow ORDER BY id, and absent reads stay nil.
func TestStorePlansOrderAndEmptyIdentity(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()
	revisionID := createTestRevision(t, store, "v3")

	missing, err := store.GetPlanByID(ctx, 999)
	if err != nil || missing != nil {
		t.Fatalf("missing plan = %+v err=%v, want nil without error", missing, err)
	}
	missing, err = store.GetPlanByFingerprint(ctx, 999, "absent")
	if err != nil || missing != nil {
		t.Fatalf("missing plan by fingerprint = %+v err=%v, want nil without error", missing, err)
	}
	empty, err := store.ListPlansBySearchRevision(ctx, 999)
	if err != nil || empty != nil {
		t.Fatalf("empty plan list = %+v err=%v, want nil without error", empty, err)
	}

	first, err := store.FindOrCreatePlan(ctx, search.PlanInput{
		SearchRevisionID: revisionID, ExecutionFingerprint: "fp-a", ResolvedManifestHash: "manifest-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.FindOrCreatePlan(ctx, search.PlanInput{
		SearchRevisionID: revisionID, ExecutionFingerprint: "fp-b", ResolvedManifestHash: "manifest-b",
	})
	if err != nil {
		t.Fatal(err)
	}
	plans, err := store.ListPlansBySearchRevision(ctx, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 || plans[0].ID != first || plans[1].ID != second {
		t.Fatalf("plan list = %+v, want ids [%d %d]", plans, first, second)
	}
}
