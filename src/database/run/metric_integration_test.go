// metric_integration_test.go exercises the run-metric store against a database
// created by the production migration runner.
//go:build integration

package run_test

import (
	"context"
	"testing"

	"analysis/database/run"
)

// TestStoreMetricSetAndReadPreservesUpsertAndOrdering verifies replace-by-key semantics, ordering, whole-run sources, and absent-read identity.
func TestStoreMetricSetAndReadPreservesUpsertAndOrdering(t *testing.T) {
	store, _ := openFamilyStore(t)
	ctx := context.Background()
	runID, err := store.StartRun(ctx, "metrics", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := store.SetMetric(ctx, run.MetricInput{RunID: runID, Metric: "records", Source: "scopus", Value: 4}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetMetric(ctx, run.MetricInput{RunID: runID, Metric: "records", Source: "scopus", Value: 7}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetMetric(ctx, run.MetricInput{RunID: runID, Metric: "enabled", Source: "", Value: 1}); err != nil {
		t.Fatal(err)
	}

	metric, err := store.GetMetric(ctx, runID, "records", "scopus")
	if err != nil || metric == nil {
		t.Fatalf("get metric = %+v err=%v", metric, err)
	}
	if metric.Value != 7 || metric.Source != "scopus" {
		t.Fatalf("replaced metric = %+v, want value 7", metric)
	}

	wholeRun, err := store.GetMetric(ctx, runID, "enabled", "")
	if err != nil || wholeRun == nil || wholeRun.Value != 1 {
		t.Fatalf("whole-run metric = %+v err=%v", wholeRun, err)
	}

	missing, err := store.GetMetric(ctx, runID, "absent", "")
	if err != nil || missing != nil {
		t.Fatalf("missing metric = %+v err=%v, want nil without error", missing, err)
	}
	empty, err := store.ListMetricsByRun(ctx, 999)
	if err != nil || empty != nil {
		t.Fatalf("empty metric list = %+v err=%v, want nil without error", empty, err)
	}

	metrics, err := store.ListMetricsByRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 2 {
		t.Fatalf("metric count = %d, want 2", len(metrics))
	}
	if metrics[0].Metric != "enabled" || metrics[1].Metric != "records" {
		t.Fatalf("metric order = %+v, want enabled then records", metrics)
	}
}
