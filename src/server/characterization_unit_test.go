// characterization_unit_test.go pins query-budget boundary semantics that the
// migration must preserve when the budget driver wrappers are reorganized.
//go:build unit

package server

import (
	"context"
	"testing"
)

// TestCharacterizationQueryBudgetUnattachedAndDisabledContext verifies a
// missing budget context is free and a zero limit disables accounting entirely.
func TestCharacterizationQueryBudgetUnattachedAndDisabledContext(t *testing.T) {
	if err := consumeQuery(context.Background()); err != nil {
		t.Fatalf("consumeQuery without a budget = %v, want nil", err)
	}
	ctx, state := withQueryBudget(context.Background(), 0)
	for attempt := 0; attempt < 3; attempt++ {
		if err := consumeQuery(ctx); err != nil {
			t.Fatalf("consumeQuery with a disabled budget = %v, want nil", err)
		}
	}
	if used := state.used.Load(); used != 0 {
		t.Fatalf("disabled budget used=%d, want 0", used)
	}
	if state.exceeded.Load() {
		t.Fatal("disabled budget reported exceeded")
	}
}
