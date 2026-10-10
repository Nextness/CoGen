// run_unit_test.go verifies the run family's retry classification, nullable
// mapping, timestamp formats, and typed conflict message without a database.
//go:build unit

package run

import (
	"errors"
	"regexp"
	"testing"
)

// TestIsRetryableErrorClassifiesTransientSQLiteFailures verifies only UNIQUE and lock failures are retried.
func TestIsRetryableErrorClassifiesTransientSQLiteFailures(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unique constraint", errors.New("UNIQUE constraint failed: pipeline_runs.execution_plan_id, pipeline_runs.attempt_number"), true},
		{"database locked", errors.New("database is locked (5) (SQLITE_BUSY)"), true},
		{"sqlite busy", errors.New("SQLITE_BUSY: database is locked"), true},
		{"permanent", errors.New("no such table: pipeline_runs"), false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := isRetryableError(test.err); got != test.want {
				t.Fatalf("isRetryableError(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}

// TestNullableMappingsPreserveEmptyAndRequiredValues verifies empty optional text becomes NULL and required IDs stay valid.
func TestNullableMappingsPreserveEmptyAndRequiredValues(t *testing.T) {
	if value := nullableString(""); value.Valid {
		t.Fatalf("empty string = %+v, want invalid", value)
	}
	if value := nullableString("text"); !value.Valid || value.String != "text" {
		t.Fatalf("text = %+v, want valid text", value)
	}
	if value := nullableInt64(42); !value.Valid || value.Int64 != 42 {
		t.Fatalf("required ID = %+v, want valid 42", value)
	}
}

// TestTimestampFormatsPreservePrecision verifies the run and run-step persisted formats.
func TestTimestampFormatsPreservePrecision(t *testing.T) {
	runPattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`)
	if !runPattern.MatchString(runTimestamp()) {
		t.Fatalf("run timestamp %q does not match the persisted second format", runTimestamp())
	}
	stepPattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$`)
	if !stepPattern.MatchString(runStepTimestamp()) {
		t.Fatalf("step timestamp %q does not match the persisted microsecond format", runStepTimestamp())
	}
}

// TestAttemptAlreadyRunningErrorReportsPlanAndRun verifies the typed conflict message.
func TestAttemptAlreadyRunningErrorReportsPlanAndRun(t *testing.T) {
	err := &AttemptAlreadyRunningError{ExecutionPlanID: 7, PipelineRunID: 11}
	if err.Error() != "execution plan 7 already has running attempt 11" {
		t.Fatalf("error = %q", err.Error())
	}
}
