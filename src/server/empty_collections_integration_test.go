// empty_collections_integration_test.go pins the empty-collection JSON shape of
// the viewer reads that moved from server-owned row maps into table families.
// The original handlers returned a non-nil empty slice for an empty result set,
// which marshals as [], and the frontend relies on that array shape.
//
//go:build integration

package server

import (
	"net/http"
	"testing"
)

// TestMovedViewerReadsPreserveEmptyCollectionShapes verifies a filtered-empty
// page still marshals its collection key as an empty array rather than null.
func TestMovedViewerReadsPreserveEmptyCollectionShapes(t *testing.T) {
	path, runID, _, _ := viewerFixture(t)
	viewer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer viewer.Close()
	handler := viewer.Handler()
	run := stringID(runID)

	work, err := viewer.writeDB.DB.Exec("INSERT INTO works (doi) VALUES ('10.1000/empty-collections')")
	if err != nil {
		t.Fatal(err)
	}
	workID, err := work.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	revision, err := viewer.writeDB.DB.Exec(`INSERT INTO work_revisions (work_id, pipeline_run_id, payload_hash, producer_stage)
		VALUES (?, ?, 'empty-collections', 'normalize')`, workID, runID)
	if err != nil {
		t.Fatal(err)
	}
	emptyRevisionID, err := revision.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := viewer.writeDB.DB.Exec(`INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome)
		VALUES (?, ?, 'validate', 'valid')`, runID, workID); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		path string
		key  string
	}{
		{"/api/runs/" + run + "/artifacts?role=no-such-role", "artifacts"},
		{"/api/runs/" + run + "/cache-uses?q=no-match", "rows"},
		{"/api/runs/" + run + "/cache-uses?q=no-match", "cache_uses"},
		{"/api/runs/" + run + "/identity-evidence?q=no-match", "rows"},
		{"/api/runs/" + run + "/corpus/sources?q=no-match", "rows"},
		{"/api/articles/" + stringID(emptyRevisionID) + "/collections/audit?run_id=" + run + "&limit=1", "items"},
	}
	for _, test := range cases {
		status, body := requestJSON(t, handler, test.path)
		if status != http.StatusOK {
			t.Fatalf("GET %s: status=%d body=%v", test.path, status, body)
		}
		value, ok := body[test.key]
		if !ok {
			t.Errorf("GET %s: missing %s", test.path, test.key)
			continue
		}
		items, ok := value.([]any)
		if !ok || len(items) != 0 {
			t.Errorf("GET %s: %s = %#v, want []", test.path, test.key, value)
		}
	}
}
