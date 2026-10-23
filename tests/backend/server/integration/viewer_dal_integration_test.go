// viewer_dal_integration_test.go verifies that every viewer-facing family
// data-access layer is bound to the server's statement-budgeted connections
// and that handler contexts flow into its calls.
//

package server_test

import testhelpers "analysis/tests/support/go/backend/server"

import (
	"analysis/internal/querybudget"
	"context"
	"errors"
	"testing"

	"analysis/backend/database/audit"
	"analysis/backend/pdfstore"
)

// viewerDALRead names one single-statement viewer-facing family read used to
// prove connection binding and context propagation.
type viewerDALRead struct {
	name string
	read func(context.Context) error
}

// viewerDALReads returns one single-statement read per viewer-facing family
// store over the fixture run.
func viewerDALReads(viewer *testhelpers.Viewer, runID int64) []viewerDALRead {
	return []viewerDALRead{
		{"search", func(ctx context.Context) error {
			_, err := viewer.ReadStores.Search.GetSearchByID(ctx, 1)
			return err
		}},
		{"run", func(ctx context.Context) error {
			_, err := viewer.ReadStores.Run.GetByID(ctx, runID)
			return err
		}},
		{"source", func(ctx context.Context) error {
			_, err := viewer.ReadStores.Source.ListSourcesByRun(ctx, runID)
			return err
		}},
		{"work", func(ctx context.Context) error {
			_, err := viewer.ReadStores.Work.GetWorkByID(ctx, 1)
			return err
		}},
		{"author", func(ctx context.Context) error {
			_, err := viewer.ReadStores.Author.GetOccurrenceByID(ctx, 1)
			return err
		}},
		{"review", func(ctx context.Context) error {
			_, err := viewer.ReadStores.Review.GetContextByRun(ctx, runID)
			return err
		}},
		{"cache", func(ctx context.Context) error {
			_, err := viewer.ReadStores.Cache.ListUsesByRun(ctx, runID)
			return err
		}},
		{"audit", func(ctx context.Context) error {
			_, err := viewer.ReadStores.Audit.List(ctx, audit.Filter{Limit: 1})
			return err
		}},
		{"artifact", func(ctx context.Context) error {
			_, err := viewer.ReadStores.Artifact.GetByID(ctx, 1)
			return err
		}},
	}
}

// TestFamilyConstructorsBindBudgetedReadOnlyConnection verifies every viewer-facing
// family constructor preserves the supplied statement-budgeted connection.
// HTTP query-count tests separately verify the viewer's production wiring.
func TestFamilyConstructorsBindBudgetedReadOnlyConnection(t *testing.T) {
	path, runID, _, _ := testhelpers.ViewerFixture(t)
	viewer, err := testhelpers.OpenViewer(t, path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()

	for _, read := range viewerDALReads(viewer, runID) {
		t.Run(read.name, func(t *testing.T) {
			ctx, budget := querybudget.With(context.Background(), 1)
			if err := read.read(ctx); err != nil {
				t.Fatalf("first %s read: %v", read.name, err)
			}
			if used := budget.Used(); used != 1 {
				t.Fatalf("%s read used %d budgeted statements, want 1; the DAL is not bound to the supplied budgeted connection", read.name, used)
			}
			if err := read.read(ctx); !errors.Is(err, querybudget.ErrExceeded) {
				t.Fatalf("second %s read error=%v, want the query budget rejection", read.name, err)
			}
			if !budget.Exceeded() {
				t.Fatalf("%s read did not record the exceeded statement budget", read.name)
			}
		})
	}
}

// TestFamilyConstructorsPropagateCancellation verifies every viewer-facing
// family DAL passes the handler context into its call so canceled requests
// stop before touching the database.
func TestFamilyConstructorsPropagateCancellation(t *testing.T) {
	path, runID, _, _ := testhelpers.ViewerFixture(t)
	viewer, err := testhelpers.OpenViewer(t, path)
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

// TestCompanionConstructorBindsBudgetedReadOnlyConnection verifies the viewer's
// companion PDF constructor preserves a supplied budgeted read-only pool and request context.
func TestCompanionConstructorBindsBudgetedReadOnlyConnection(t *testing.T) {
	fixture := testhelpers.NewPDFViewerFixture(t)
	if pdfstore.New(fixture.Server.PDF) == nil {
		t.Fatal("viewer did not construct a companion PDF DAL for the bound store")
	}

	ctx, budget := querybudget.With(context.Background(), 1)
	document, err := pdfstore.New(fixture.Server.PDF).Document(ctx, "10.1000/viewer-available")
	if err != nil {
		t.Fatalf("first companion read: %v", err)
	}
	if document == nil || document.Status != pdfstore.StatusAvailable {
		t.Fatalf("companion document = %+v", document)
	}
	if used := budget.Used(); used != 1 {
		t.Fatalf("companion read used %d budgeted statements, want 1; the DAL is not bound to the supplied budgeted PDF connection", used)
	}
	if _, err := pdfstore.New(fixture.Server.PDF).Document(ctx, "10.1000/viewer-available"); !errors.Is(err, querybudget.ErrExceeded) {
		t.Fatalf("second companion read error=%v, want the query budget rejection", err)
	}
	if !budget.Exceeded() {
		t.Fatal("companion read did not record the exceeded statement budget")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := pdfstore.New(fixture.Server.PDF).Document(canceled, "10.1000/viewer-available"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled companion read error=%v, want context.Canceled", err)
	}
}
