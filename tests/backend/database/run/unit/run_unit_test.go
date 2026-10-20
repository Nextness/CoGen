// run_unit_test.go verifies retry, parameter mapping, and timestamp contracts through public store writes.
package run_test

import (
	"database/sql/driver"
	"errors"
	"regexp"
	"strings"
	"testing"

	"analysis/backend/database/run"
	"analysis/tests/support/go/sqlprobe"
)

// TestStartAttemptClassifiesTransientSQLiteFailures verifies retryable failures retry and permanent failures stop immediately.
func TestStartAttemptClassifiesTransientSQLiteFailures(t *testing.T) {
	for _, test := range []struct {
		name    string
		failure error
		retry   bool
	}{
		{"nil", nil, false},
		{"unique constraint", errors.New("UNIQUE constraint failed: pipeline_runs.execution_plan_id, pipeline_runs.attempt_number"), true},
		{"database locked", errors.New("database is locked (5) (SQLITE_BUSY)"), true},
		{"sqlite busy", errors.New("SQLITE_BUSY: database is locked"), true},
		{"permanent", errors.New("no such table: pipeline_runs"), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			lookups := 0
			d := &sqlprobe.Driver{}
			d.Query = func(call sqlprobe.Call) (driver.Rows, error) {
				if strings.Contains(call.SQL, "FindRunningAttemptByPlan") {
					lookups++
					if lookups == 1 && test.failure != nil {
						return nil, test.failure
					}
					return &sqlprobe.Rows{Names: []string{"id"}}, nil
				}
				return &sqlprobe.Rows{Names: []string{"max_attempt_number"}, Values: [][]driver.Value{{int64(0)}}}, nil
			}
			d.Exec = func(sqlprobe.Call) (driver.Result, error) { return sqlprobe.Result{ID: 11, Affected: 1}, nil }
			db := d.OpenDB()
			defer db.Close()
			id, attempt, err := run.New(db).StartAttemptIfIdle(t.Context(), run.StartAttemptInput{ExecutionPlanID: 7, Step: "parse"})
			wantLookups := 1
			if test.retry {
				wantLookups = 2
			}
			if lookups != wantLookups || d.Begins != wantLookups {
				t.Fatalf("lookups=%d begins=%d, want %d", lookups, d.Begins, wantLookups)
			}
			if test.failure != nil && !test.retry {
				if !errors.Is(err, test.failure) || d.Rollbacks != 1 || d.Commits != 0 {
					t.Fatalf("permanent error=%v rollback=%d commit=%d", err, d.Rollbacks, d.Commits)
				}
			} else if err != nil || id != 11 || attempt != 1 || d.Commits != 1 {
				t.Fatalf("start=(%d,%d,%v), commits=%d", id, attempt, err, d.Commits)
			}
			if test.retry && d.Rollbacks != 1 {
				t.Fatalf("retry failed to roll back: %d", d.Rollbacks)
			}
		})
	}
}

// TestStoreWritesPreserveNullabilityAndTimestampPrecision verifies actual bindings sent by run and step writes.
func TestStoreWritesPreserveNullabilityAndTimestampPrecision(t *testing.T) {
	d := &sqlprobe.Driver{}
	var calls []sqlprobe.Call
	d.Exec = func(call sqlprobe.Call) (driver.Result, error) {
		calls = append(calls, call)
		return sqlprobe.Result{ID: 42, Affected: 1}, nil
	}
	d.Query = func(call sqlprobe.Call) (driver.Rows, error) {
		if strings.Contains(call.SQL, "FindRunningAttemptByPlan") {
			return &sqlprobe.Rows{Names: []string{"id"}}, nil
		}
		return &sqlprobe.Rows{Names: []string{"max_attempt_number"}, Values: [][]driver.Value{{int64(0)}}}, nil
	}
	db := d.OpenDB()
	defer db.Close()
	store := run.New(db)
	for _, text := range []string{"", "text"} {
		if _, _, err := store.StartAttemptIfIdle(t.Context(), run.StartAttemptInput{ExecutionPlanID: 42, Step: "parse", SearchQuery: text}); err != nil {
			t.Fatal(err)
		}
		args := calls[len(calls)-1].Args
		if len(args) != 5 || args[3].Value != int64(42) {
			t.Fatalf("attempt bindings=%v", args)
		}
		var want any = text
		if text == "" {
			want = nil
		}
		if args[2].Value != want {
			t.Fatalf("query=%v, want %v", args[2].Value, want)
		}
		if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`).MatchString(args[1].Value.(string)) {
			t.Fatalf("run timestamp=%v", args[1].Value)
		}
	}
	if _, err := store.CreateStep(t.Context(), 42, "parse"); err != nil {
		t.Fatal(err)
	}
	args := calls[len(calls)-1].Args
	if args[0].Value != int64(42) || !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$`).MatchString(args[2].Value.(string)) {
		t.Fatalf("step bindings=%v", args)
	}
}

// TestAttemptAlreadyRunningErrorReportsPlanAndRun verifies the typed conflict message.
func TestAttemptAlreadyRunningErrorReportsPlanAndRun(t *testing.T) {
	err := &run.AttemptAlreadyRunningError{ExecutionPlanID: 7, PipelineRunID: 11}
	if err.Error() != "execution plan 7 already has running attempt 11" {
		t.Fatalf("error=%q", err.Error())
	}
}
