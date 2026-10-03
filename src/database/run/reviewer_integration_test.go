// reviewer_integration_test.go exercises the per-run reviewer store against a
// database created by the production migration runner.
//go:build integration

package run_test

import (
	"context"
	"strings"
	"testing"

	"analysis/database/run"
)

// TestStoreReviewerInsertAndGetPreservesIdentity verifies trimming, immutable projection, absent identity, and input validation.
func TestStoreReviewerInsertAndGetPreservesIdentity(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()
	runID, err := store.StartRun(ctx, "reviewer", "")
	if err != nil {
		t.Fatal(err)
	}

	missing, err := store.GetReviewer(ctx, runID)
	if err != nil || missing != nil {
		t.Fatalf("missing reviewer = %+v err=%v, want nil without error", missing, err)
	}

	if err := store.InsertReviewer(ctx, run.ReviewerInput{RunID: runID, Username: "  Researcher  ", Email: "  researcher@example.test  "}); err != nil {
		t.Fatal(err)
	}
	reviewer, err := store.GetReviewer(ctx, runID)
	if err != nil || reviewer == nil {
		t.Fatalf("get reviewer = %+v err=%v", reviewer, err)
	}
	if reviewer.PipelineRunID != runID || reviewer.Username != "Researcher" || reviewer.Email != "researcher@example.test" || reviewer.CreatedAt == "" {
		t.Fatalf("reviewer = %+v", reviewer)
	}

	cases := []struct {
		name  string
		input run.ReviewerInput
	}{
		{"non-positive run", run.ReviewerInput{RunID: 0, Username: "name"}},
		{"long username", run.ReviewerInput{RunID: runID, Username: strings.Repeat("a", 201)}},
		{"long email", run.ReviewerInput{RunID: runID, Email: strings.Repeat("a", 321)}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := store.InsertReviewer(ctx, test.input); err == nil {
				t.Fatal("invalid reviewer input was accepted")
			}
		})
	}
}
