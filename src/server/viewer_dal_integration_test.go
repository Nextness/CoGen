// viewer_dal_integration_test.go verifies that every viewer-facing family
// data-access layer is bound to the server's statement-budgeted connections
// and that handler contexts flow into its calls.
//
//go:build integration

package server

import (
	"context"
	"errors"
	"testing"

	"analysis/database/audit"
	"analysis/pdfstore"
)

// viewerDALRead names one single-statement viewer-facing family read used to
// prove connection binding and context propagation.
type viewerDALRead struct {
	name string
	read func(context.Context) error
}

// viewerDALReads returns one single-statement read per viewer-facing family
// store over the fixture run.
func viewerDALReads(viewer *Server, runID int64) []viewerDALRead {
	return []viewerDALRead{
		{"search", func(ctx context.Context) error {
			_, err := viewer.searchStore.GetSearchByID(ctx, 1)
			return err
		}},
		{"run", func(ctx context.Context) error {
			_, err := viewer.runStore.GetByID(ctx, runID)
			return err
		}},
		{"source", func(ctx context.Context) error {
			_, err := viewer.sourceStore.ListSourcesByRun(ctx, runID)
			return err
		}},
		{"work", func(ctx context.Context) error {
			_, err := viewer.workStore.GetWorkByID(ctx, 1)
			return err
		}},
		{"author", func(ctx context.Context) error {
			_, err := viewer.authorStore.GetOccurrenceByID(ctx, 1)
			return err
		}},
		{"review", func(ctx context.Context) error {
			_, err := viewer.reviewStore.GetContextByRun(ctx, runID)
			return err
		}},
		{"cache", func(ctx context.Context) error {
			_, err := viewer.cacheStore.ListUsesByRun(ctx, runID)
			return err
		}},
		{"audit", func(ctx context.Context) error {
			_, err := viewer.auditStore.List(ctx, audit.Filter{Limit: 1})
			return err
		}},
		{"artifact", func(ctx context.Context) error {
			_, err := viewer.artifact.GetByID(ctx, 1)
			return err
		}},
	}
}

// TestViewerDALsBindBudgetedReadOnlyConnection verifies every viewer-facing
// family DAL executes through the server's statement-budgeted connection
// instead of an independently opened one.
func TestViewerDALsBindBudgetedReadOnlyConnection(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()

	for _, read := range viewerDALReads(viewer, runID) {
		t.Run(read.name, func(t *testing.T) {
			ctx, budget := withQueryBudget(context.Background(), 1)
			if err := read.read(ctx); err != nil {
				t.Fatalf("first %s read: %v", read.name, err)
			}
			if used := budget.used.Load(); used != 1 {
				t.Fatalf("%s read used %d budgeted statements, want 1; the DAL is not bound to the server's budgeted connection", read.name, used)
			}
			if err := read.read(ctx); !errors.Is(err, errQueryBudgetExceeded) {
				t.Fatalf("second %s read error=%v, want the query budget rejection", read.name, err)
			}
			if !budget.exceeded.Load() {
				t.Fatalf("%s read did not record the exceeded statement budget", read.name)
			}
		})
	}
}

// TestViewerDALsPropagateHandlerCancellation verifies every viewer-facing
// family DAL passes the handler context into its call so canceled requests
// stop before touching the database.
func TestViewerDALsPropagateHandlerCancellation(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()

	for _, read := range viewerDALReads(viewer, runID) {
		t.Run(read.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := read.read(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("%s canceled read error=%v, want context.Canceled", read.name, err)
			}
		})
	}
}

// TestViewerCompanionDALBindsBudgetedReadOnlyConnection verifies the viewer's
// companion PDF DAL executes through the server's statement-budgeted read-only
// PDF connection and receives handler contexts.
func TestViewerCompanionDALBindsBudgetedReadOnlyConnection(t *testing.T) {
	fixture := newPDFViewerFixture(t)
	if fixture.server.pdfStore == nil {
		t.Fatal("viewer did not construct a companion PDF DAL for the bound store")
	}

	ctx, budget := withQueryBudget(context.Background(), 1)
	document, err := fixture.server.pdfStore.Document(ctx, "10.1000/viewer-available")
	if err != nil {
		t.Fatalf("first companion read: %v", err)
	}
	if document == nil || document.Status != pdfstore.StatusAvailable {
		t.Fatalf("companion document = %+v", document)
	}
	if used := budget.used.Load(); used != 1 {
		t.Fatalf("companion read used %d budgeted statements, want 1; the DAL is not bound to the server's budgeted PDF connection", used)
	}
	if _, err := fixture.server.pdfStore.Document(ctx, "10.1000/viewer-available"); !errors.Is(err, errQueryBudgetExceeded) {
		t.Fatalf("second companion read error=%v, want the query budget rejection", err)
	}
	if !budget.exceeded.Load() {
		t.Fatal("companion read did not record the exceeded statement budget")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fixture.server.pdfStore.Document(canceled, "10.1000/viewer-available"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled companion read error=%v, want context.Canceled", err)
	}
}
