// metric.go provides the handwritten run metric data access layer over the
// generated statements. Values are upserted by run, metric, and source so a
// repeated snapshot replaces the prior value for that scope.
package run

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	generated "analysis/database/run/internal/sql"
)

// SetMetric inserts or replaces a metric value for a given run, metric name,
// and source. If source is empty, it records a whole-run metric.
func (s *Store) SetMetric(ctx context.Context, input MetricInput) error {
	if err := s.queries.UpsertRunMetric(ctx, generated.UpsertRunMetricParams{
		PipelineRunID: input.RunID,
		Metric:        input.Metric,
		Source:        input.Source,
		Value:         int64(input.Value),
	}); err != nil {
		return fmt.Errorf("set metric: %w", err)
	}
	return nil
}

// GetMetric returns a single metric for a given run, metric name, and source.
// Returns nil, nil if the metric is not recorded for this run.
func (s *Store) GetMetric(ctx context.Context, runID int64, metric, source string) (*Metric, error) {
	row, err := s.queries.GetRunMetric(ctx, generated.GetRunMetricParams{
		PipelineRunID: runID,
		Metric:        metric,
		Source:        source,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return metricFromGenerated(row), nil
}

// ListMetricsByRun returns all metrics for a given pipeline run, ordered by
// metric name then source.
func (s *Store) ListMetricsByRun(ctx context.Context, runID int64) ([]*Metric, error) {
	rows, err := s.queries.ListRunMetricsByRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	var metrics []*Metric
	for _, row := range rows {
		metrics = append(metrics, metricFromGenerated(row))
	}
	return metrics, nil
}

// metricFromGenerated maps one generated metric row into an application metric.
func metricFromGenerated(row generated.PipelineRunMetric) *Metric {
	return &Metric{
		PipelineRunID: row.PipelineRunID,
		Metric:        row.Metric,
		Source:        row.Source,
		Value:         int(row.Value),
	}
}
