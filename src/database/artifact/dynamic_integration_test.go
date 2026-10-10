// dynamic_integration_test.go verifies the retained run-artifact builder
// rejects injection attempts, follows its declared ordering, and caps its
// search, role, and page inputs.
//go:build integration

package artifact_test

import (
	"context"
	"strings"
	"testing"

	"analysis/database/artifact"
)

// TestStoreListRunArtifactEvidenceConstrainsDynamicInputs verifies the retained
// run-artifact builder binds search and role values, rejects invalid order
// values, follows its declared ordering, and caps oversized inputs.
func TestStoreListRunArtifactEvidenceConstrainsDynamicInputs(t *testing.T) {
	store, db := openFamilyStore(t)
	ctx := context.Background()
	runID := createTestRun(t, db, "artifact-evidence-dynamic")

	alphaID, err := store.Create(ctx, artifact.CreateInput{ContentHash: "alpha-evidence", ContentType: "application/json", ByteSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	betaID, err := store.Create(ctx, artifact.CreateInput{ContentHash: `x' OR '1'='1`, ContentType: "text/plain", ByteSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Link(ctx, artifact.LinkInput{PipelineRunID: runID, ArtifactID: alphaID, Role: artifact.RunArtifactWorkspaceConfig}); err != nil {
		t.Fatal(err)
	}
	if err := store.Link(ctx, artifact.LinkInput{PipelineRunID: runID, ArtifactID: betaID, Role: artifact.RunArtifactResolvedManifest}); err != nil {
		t.Fatal(err)
	}

	ascending, err := store.ListRunArtifactEvidence(ctx, artifact.RunArtifactFilter{RunID: runID, Order: "ASC", Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(ascending.Items) != 2 || ascending.Items[0].ID != alphaID || ascending.Items[1].ID != betaID {
		t.Fatalf("ascending artifact order = %+v", ascending.Items)
	}
	descending, err := store.ListRunArtifactEvidence(ctx, artifact.RunArtifactFilter{RunID: runID, Order: "DESC", Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(descending.Items) != 2 || descending.Items[0].ID != betaID || descending.Items[1].ID != alphaID {
		t.Fatalf("descending artifact order = %+v", descending.Items)
	}

	// A malicious search value is bound as a literal LIKE pattern and matches
	// only the artifact that stores the same text.
	literal, err := store.ListRunArtifactEvidence(ctx, artifact.RunArtifactFilter{RunID: runID, Query: `' OR '1'='1`, Order: "ASC", Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(literal.Items) != 1 || literal.Items[0].ID != betaID {
		t.Fatalf("literal search = %+v, want only the stored malicious text", literal.Items)
	}

	for _, test := range []struct {
		name   string
		filter artifact.RunArtifactFilter
	}{
		{"injected order", artifact.RunArtifactFilter{RunID: runID, Order: "ASC; DROP TABLE artifacts; --", Limit: 25}},
		{"oversized query", artifact.RunArtifactFilter{RunID: runID, Query: strings.Repeat("x", 201), Order: "ASC", Limit: 25}},
		{"oversized role", artifact.RunArtifactFilter{RunID: runID, Role: strings.Repeat("x", 101), Order: "ASC", Limit: 25}},
		{"oversized limit", artifact.RunArtifactFilter{RunID: runID, Order: "ASC", Limit: 1001}},
		{"oversized page size", artifact.RunArtifactFilter{RunID: runID, Order: "ASC", Page: 1, PerPage: 1001, PageMode: true, Limit: 25}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.ListRunArtifactEvidence(ctx, test.filter); err == nil {
				t.Fatal("expected the retained builder to reject the input")
			}
		})
	}
}
