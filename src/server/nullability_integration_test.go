// nullability_integration_test.go preserves SQL NULL, empty strings, and zeroes
// independently through the family projections and the public viewer API.
//go:build integration

package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"

	"analysis/database"
	"analysis/internal/testfixture"
)

// TestViewerMetadataPreservesNullEmptyAndZero verifies optional article,
// authorship, reference, stage, and graph fields retain their database values.
func TestViewerMetadataPreservesNullEmptyAndZero(t *testing.T) {
	for _, test := range []struct {
		name   string
		text   any
		number any
	}{
		{name: "null"},
		{name: "empty and zero", text: "", number: float64(0)},
		{name: "recorded", text: "recorded", number: float64(2024)},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "metadata.db")
			metadata, err := database.Open(path, filepath.Join("..", "..", "config", "database.something"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { metadata.Close() })
			runID, err := testfixture.Run(context.Background(), metadata.RawDBForTest(), "nullability", "")
			if err != nil {
				t.Fatal(err)
			}
			if err := metadata.Run.FinishRun(context.Background(), runID, "completed", ""); err != nil {
				t.Fatal(err)
			}
			// Direct inserts distinguish valid historical SQL values that the
			// pipeline's optional-input conventions normally coalesce on writes.
			exec := func(query string, args ...any) int64 {
				t.Helper()
				result, err := metadata.RawDBForTest().Exec(query, args...)
				if err != nil {
					t.Fatal(err)
				}
				id, err := result.LastInsertId()
				if err != nil {
					t.Fatal(err)
				}
				return id
			}
			workID := exec("INSERT INTO works (doi) VALUES (?)", test.text)
			revisionID := exec(`INSERT INTO work_revisions
				(work_id, pipeline_run_id, payload_hash, producer_stage, title, abstract, year, journal, publisher, source,
				keywords, keywords_plus, citation_count, reference_count, extension_data)
				VALUES (?, ?, 'fixture', 'normalize', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				workID, runID, test.text, test.text, test.number, test.text, test.text, test.text,
				test.text, test.text, test.number, test.number, test.text)
			exec(`INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome, reason)
				VALUES (?, ?, 'validate', 'valid', ?)`, runID, workID, test.text)
			authorID := exec(`INSERT INTO author_occurrences (citation_name, first_name, last_name, orcid)
				VALUES ('Fixture author', ?, ?, ?)`, test.text, test.text, test.text)
			exec(`INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation)
				VALUES (?, ?, 1, ?)`, revisionID, authorID, test.text)
			referenceID := exec(`INSERT INTO reference_mentions
				(work_revision_id, mention_order, raw_reference, doi, title, author, year, source)
				VALUES (?, 1, ?, ?, ?, ?, ?, ?)`, revisionID, test.text, test.text, test.text, test.text, test.number, test.text)
			viewer, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer viewer.Close()
			get := func(path string) map[string]any {
				t.Helper()
				status, body := requestJSON(t, viewer.Handler(), path)
				if status != http.StatusOK {
					t.Fatalf("GET %s: status=%d body=%v", path, status, body)
				}
				return body
			}
			articlePath := fmt.Sprintf("/api/articles/%d", revisionID)
			article := get(fmt.Sprintf("%s?run_id=%d", articlePath, runID))
			assertMetadataFields(t, article["article"].(map[string]any), test.text,
				"title", "abstract", "journal", "publisher", "source", "keywords", "keywords_plus", "extension_data", "doi")
			assertMetadataFields(t, article["article"].(map[string]any), test.number, "year", "citation_count", "reference_count")
			for _, collection := range []string{"authors", "references", "stages"} {
				body := get(fmt.Sprintf("%s/collections/%s?run_id=%d", articlePath, collection, runID))
				row := body["items"].([]any)[0].(map[string]any)
				switch collection {
				case "authors":
					assertMetadataFields(t, row, test.text, "first_name", "last_name", "orcid", "affiliation")
				case "references":
					assertMetadataFields(t, row, test.text, "doi", "title", "author", "source")
					assertMetadataFields(t, row, test.number, "year")
				case "stages":
					assertMetadataFields(t, row, test.text, "reason")
				}
			}
			reference := get(fmt.Sprintf("/api/references/%d?run_id=%d", referenceID, runID))["reference"].(map[string]any)
			assertMetadataFields(t, reference, test.text, "raw_reference", "doi", "title", "author", "source", "citing_title")
			assertMetadataFields(t, reference, test.number, "year")
			for _, collection := range []string{"corpus/articles", "evaluation", "corpus/authors", "corpus/references", "stages"} {
				body := get(fmt.Sprintf("/api/runs/%d/%s", runID, collection))
				row := body["rows"].([]any)[0].(map[string]any)
				switch collection {
				case "corpus/articles", "evaluation":
					assertMetadataFields(t, row, test.text, "title", "journal", "publisher", "source", "doi", "abstract", "keywords", "keywords_plus")
					assertMetadataFields(t, row, test.number, "year", "citation_count", "reference_count")
				case "corpus/authors":
					assertMetadataFields(t, row, test.text, "first_name", "last_name", "orcid")
				case "corpus/references":
					assertMetadataFields(t, row, test.text, "doi", "title", "author", "source", "citing_title")
					assertMetadataFields(t, row, test.number, "year")
				case "stages":
					assertMetadataFields(t, row, test.text, "reason")
				}
			}
			for _, mode := range []string{"article_author", "article_reference", "research_network"} {
				body := get(fmt.Sprintf("/api/graph?run_id=%d&mode=%s", runID, mode))
				for _, item := range body["nodes"].([]any) {
					node := item.(map[string]any)
					switch node["type"] {
					case "author":
						assertMetadataFields(t, node, test.text, "orcid")
					case "reference":
						assertMetadataFields(t, node, test.text, "label", "doi", "author")
						assertMetadataFields(t, node, test.number, "year")
					}
				}
			}
		})
	}
}

// assertMetadataFields compares exact JSON values without coalescing null,
// empty strings, or zeroes.
func assertMetadataFields(t *testing.T, row map[string]any, expected any, fields ...string) {
	t.Helper()
	for _, field := range fields {
		actual, present := row[field]
		if !present || !reflect.DeepEqual(actual, expected) {
			t.Errorf("%s = %#v (present=%v), want %#v", field, actual, present, expected)
		}
	}
}
