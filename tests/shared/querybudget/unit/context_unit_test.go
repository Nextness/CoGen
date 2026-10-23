// characterization_unit_test.go verifies query-budget boundary semantics for
// requests without a budget and requests with disabled accounting.

package querybudget_test

import "analysis/internal/querybudget"

import (
	"context"
	"testing"
)

// TestCharacterizationQueryBudgetUnattachedAndDisabledContext verifies a
// missing budget context is free and a zero limit disables accounting entirely.
func TestCharacterizationQueryBudgetUnattachedAndDisabledContext(t *testing.T) {
	if err := querybudget.Consume(context.Background()); err != nil {
		t.Fatalf("consumeQuery without a budget = %v, want nil", err)
	}
	ctx, state := querybudget.With(context.Background(), 0)
	for attempt := 0; attempt < 3; attempt++ {
		if err := querybudget.Consume(ctx); err != nil {
			t.Fatalf("consumeQuery with a disabled budget = %v, want nil", err)
		}
	}
	if used := state.Used(); used != 0 {
		t.Fatalf("disabled budget used=%d, want 0", used)
	}
	if state.Exceeded() {
		t.Fatal("disabled budget reported exceeded")
	}
}
