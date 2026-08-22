// fixture_integration_test.go generates a workspace fixture database used by the
// dev server and Playwright integration tests. It is not a
// conventional test and is skipped when the fixture already exists.
//
//go:build integration

package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"analysis/database"
	"analysis/pdfstore"
	"analysis/searchterms"
)

// viewerFixtureContractVersion identifies the generated data contract expected by frontend browser tests.
const viewerFixtureContractVersion = 1

// TestGenerateFixture creates a workspace fixture database at
// src/server/testdata/workspace.fixture.db. It is used by the dev server
// and by Playwright tests. Run it with:
//
//	cd src && FORCE_FIXTURE=1 go test ./server -run TestGenerateFixture -count=1
//
// The FORCE_FIXTURE variable is required when the fixture already exists;
// the test skips automatically when the output file is present.
func TestGenerateFixture(t *testing.T) {
	outputPath := filepath.Join("testdata", "workspace.fixture.db")
	outputPDFPath := filepath.Join("testdata", "workspace.fixture.pdf.db")

	// Skip unless explicitly forced or the fixture does not exist.
	if os.Getenv("FORCE_FIXTURE") != "1" {
		if _, err := os.Stat(outputPath); err == nil {
			if _, pdfErr := os.Stat(outputPDFPath); pdfErr == nil {
				t.Skip("fixtures exist; set FORCE_FIXTURE=1 to regenerate")
			}
		}
	}

	// Create a temporary database, run migrations, and insert fixture data.
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "workspace.db")
	db, err := database.Open(tmpPath, filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}

	exec := func(query string, args ...any) {
		if _, err := db.DB.Exec(query, args...); err != nil {
			t.Fatalf("exec %q: %v", query[:min(len(query), 80)], err)
		}
	}

	// 1. Searches (3+)
	exec("INSERT INTO searches (search_id) VALUES ('deep-learning-nlp')")
	exec("INSERT INTO searches (search_id) VALUES ('quantum-computing')")
	exec("INSERT INTO searches (search_id) VALUES ('crispr-gene-editing')")

	// 2. Search revisions (search-1 has 3+ revisions)
	exec(`INSERT INTO search_revisions (search_id, revision_label, config_artifact_hash, resolved_manifest_hash) VALUES (1, 'r1', 'config-hash-dl-v1', 'manifest-hash-dl-v1')`)
	exec(`INSERT INTO search_revisions (search_id, revision_label, config_artifact_hash, resolved_manifest_hash) VALUES (1, 'r2', 'config-hash-dl-v2', 'manifest-hash-dl-v2')`)
	exec(`INSERT INTO search_revisions (search_id, revision_label, config_artifact_hash, resolved_manifest_hash) VALUES (1, 'r3', 'config-hash-dl-v3', 'manifest-hash-dl-v3')`)
	exec(`INSERT INTO search_revisions (search_id, revision_label, config_artifact_hash, resolved_manifest_hash) VALUES (2, 'r1', 'config-hash-qc-v1', 'manifest-hash-qc-v1')`)
	exec(`INSERT INTO search_revisions (search_id, revision_label, config_artifact_hash, resolved_manifest_hash) VALUES (3, 'r1', 'config-hash-cr-v1', 'manifest-hash-cr-v1')`)

	// 3. Execution plans (3+ with distinct fingerprints)
	exec(`INSERT INTO execution_plans (search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash, enrichment_enabled) VALUES (1, 'fp-deep-learning-v1', 'manifest-hash-dl-v1', 'input-dl-v1', 1)`)
	exec(`INSERT INTO execution_plans (search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash, enrichment_enabled) VALUES (2, 'fp-deep-learning-v2', 'manifest-hash-dl-v2', 'input-dl-v2', 1)`)
	exec(`INSERT INTO execution_plans (search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash, enrichment_enabled) VALUES (3, 'fp-deep-learning-v3', 'manifest-hash-dl-v3', 'input-dl-v3', 0)`)
	exec(`INSERT INTO execution_plans (search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash, enrichment_enabled) VALUES (4, 'fp-quantum-v1', 'manifest-hash-qc-v1', 'input-qc-v1', 1)`)
	exec(`INSERT INTO execution_plans (search_revision_id, execution_fingerprint, resolved_manifest_hash, input_manifest_hash, enrichment_enabled) VALUES (5, 'fp-crispr-v1', 'manifest-hash-cr-v1', 'input-cr-v1', 1)`)

	// 4. Pipeline runs (5+)
	//   Run 1: completed, full metrics, enrichment enabled
	//   Run 2: failed
	//   Run 3: trashed
	//   Run 4: completed, enrichment disabled
	//   Run 5: very recent, completed
	//   Run 6: completed
	exec(`INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number, summary) VALUES ('workspace', '2024-01-20T10:00:00Z', '2024-01-20T10:30:00Z', 'completed', 1, 1, 'Full pipeline run with enrichment')`)
	exec(`INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number, summary) VALUES ('workspace', '2024-02-15T08:00:00Z', '2024-02-15T08:15:00Z', 'failed', 1, 2, 'Failed during enrichment stage')`)
	exec(`INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number, summary, visibility_state, trashed_at, trash_reason) VALUES ('workspace', '2024-03-10T14:00:00Z', '2024-03-10T14:25:00Z', 'completed', 2, 1, 'Trashed after review', 'trashed', '2024-03-15T09:00:00Z', 'Incorrect deduplication settings')`)
	exec(`INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number, summary) VALUES ('workspace', '2024-04-05T11:00:00Z', '2024-04-05T11:20:00Z', 'completed', 3, 1, 'No enrichment pipeline run')`)
	exec(`INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number, summary) VALUES ('workspace', '2025-07-20T09:00:00Z', '2025-07-20T09:35:00Z', 'completed', 4, 1, 'Recent quantum computing pipeline')`)
	exec(`INSERT INTO pipeline_runs (step, started_at, finished_at, status, execution_plan_id, attempt_number, summary) VALUES ('workspace', '2024-05-12T16:00:00Z', '2024-05-12T16:15:00Z', 'completed', 5, 1, 'CRISPR pipeline run')`)

	// 5. Run sources and source records
	exec("INSERT INTO run_sources (pipeline_run_id, source_name, source_type, expected_file, query, expected_result_count, observed_result_count, result_count_comparison) VALUES (1, 'scopus-export', 'csv', 'scopus.csv', ?, 4, 2, 'below')", `TITLE-ABS-KEY(("transformer" OR "deep learning" OR "neural" OR "attention") AND ("reinforcement" OR "convolutional"))`)
	exec("INSERT INTO run_sources (pipeline_run_id, source_name, source_type, expected_file, query, expected_result_count, observed_result_count, result_count_comparison) VALUES (1, 'ieee-export', 'csv', 'ieee.csv', ?, 1, 1, 'match')", `("Document Title": ("transformer" OR "deep learning")) OR ("documentAbstract": ("neural")) OR ("authorTerms": ("attention"))`)
	exec("INSERT INTO run_sources (pipeline_run_id, source_name, source_type, expected_file) VALUES (2, 'scopus-export', 'csv', 'scopus.csv')")
	exec("INSERT INTO run_sources (pipeline_run_id, source_name, source_type, expected_file) VALUES (3, 'scopus-export', 'csv', 'scopus.csv')")
	exec("INSERT INTO run_sources (pipeline_run_id, source_name, source_type, expected_file) VALUES (4, 'scopus-export', 'csv', 'scopus.csv')")
	exec("INSERT INTO run_sources (pipeline_run_id, source_name, source_type, expected_file) VALUES (5, 'scopus-export', 'csv', 'scopus.csv')")
	exec("INSERT INTO run_sources (pipeline_run_id, source_name, source_type, expected_file) VALUES (6, 'scopus-export', 'csv', 'scopus.csv')")

	exec("INSERT INTO source_records (run_source_id, record_index, raw_payload, content_hash, parse_status) VALUES (1, 1, '{}', 'hash-scopus-1', 'parsed')")
	exec("INSERT INTO source_records (run_source_id, record_index, raw_payload, content_hash, parse_status) VALUES (1, 2, '{}', 'hash-scopus-2', 'parsed')")
	exec("INSERT INTO source_records (run_source_id, record_index, raw_payload, content_hash, parse_status) VALUES (2, 1, '{}', 'hash-ieee-1', 'parsed')")
	exec("INSERT INTO source_records (run_source_id, record_index, raw_payload, content_hash, parse_status) VALUES (3, 1, '{}', 'hash-failed-1', 'parse_failed')")

	// 6. Artifacts (for run steps)
	exec("INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES ('artifact-config-1', 1024, 'application/json')")
	exec("INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES ('artifact-input-1', 2048, 'application/json')")
	exec("INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES ('artifact-output-1', 4096, 'application/json')")
	exec("INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES ('artifact-config-4', 1024, 'application/json')")
	exec("INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES ('artifact-input-4', 2048, 'application/json')")
	exec("INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES ('artifact-output-4', 3072, 'application/json')")
	exec("INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES ('artifact-input-5', 2048, 'application/json')")
	exec("INSERT INTO artifacts (content_hash, byte_size, content_type) VALUES ('artifact-resolved-1', 3072, 'application/json')")

	exec("INSERT INTO artifact_blobs (artifact_id, pipeline_run_id, data) VALUES (1, 1, 'artifact-config-1')")
	exec("INSERT INTO artifact_blobs (artifact_id, pipeline_run_id, data) VALUES (2, 1, 'artifact-input-1')")
	exec("INSERT INTO artifact_blobs (artifact_id, pipeline_run_id, data) VALUES (3, 1, 'artifact-output-1')")
	exec("INSERT INTO artifact_blobs (artifact_id, pipeline_run_id, data) VALUES (4, 4, 'artifact-config-4')")
	exec("INSERT INTO artifact_blobs (artifact_id, pipeline_run_id, data) VALUES (5, 4, 'artifact-input-4')")
	exec("INSERT INTO artifact_blobs (artifact_id, pipeline_run_id, data) VALUES (6, 4, 'artifact-output-4')")
	exec("INSERT INTO artifact_blobs (artifact_id, pipeline_run_id, data) VALUES (7, 5, 'artifact-input-5')")
	exec("INSERT INTO artifact_blobs (artifact_id, pipeline_run_id, data) VALUES (8, 1, 'artifact-resolved-1')")

	// The configuration evidence is attached directly to the attempt so it
	// remains discoverable even though only two artifact slots exist on a step.
	exec("INSERT INTO run_artifacts (pipeline_run_id, artifact_id, artifact_role) VALUES (1, 1, 'workspace_config')")
	exec("INSERT INTO run_artifacts (pipeline_run_id, artifact_id, artifact_role) VALUES (1, 8, 'resolved_manifest')")
	exec("INSERT INTO run_artifacts (pipeline_run_id, artifact_id, artifact_role) VALUES (1, 2, 'input_manifest')")

	// 7. Run steps (for run 1, 4, 5)
	exec("INSERT INTO run_steps (pipeline_run_id, step_name, step_status, input_artifact_id, output_artifact_id, started_at, finished_at, input_fingerprint, output_fingerprint) VALUES (1, 'preflight', 'completed', 1, 2, '2024-01-20T10:00:00Z', '2024-01-20T10:01:00Z', 'fp-preflight-in', 'fp-preflight-out')")
	exec("INSERT INTO run_steps (pipeline_run_id, step_name, step_status, input_artifact_id, output_artifact_id, started_at, finished_at, input_fingerprint, output_fingerprint) VALUES (1, 'parse', 'completed', 2, 3, '2024-01-20T10:01:00Z', '2024-01-20T10:05:00Z', 'fp-parse-in', 'fp-parse-out')")
	exec("INSERT INTO run_steps (pipeline_run_id, step_name, step_status, input_artifact_id, output_artifact_id, started_at, finished_at, input_fingerprint, output_fingerprint) VALUES (1, 'enrich', 'completed', 3, 3, '2024-01-20T10:05:00Z', '2024-01-20T10:20:00Z', 'fp-enrich-in', 'fp-enrich-out')")
	exec("INSERT INTO run_steps (pipeline_run_id, step_name, step_status, input_artifact_id, output_artifact_id, started_at, finished_at, input_fingerprint, output_fingerprint) VALUES (4, 'preflight', 'completed', 4, 5, '2024-04-05T11:00:00Z', '2024-04-05T11:01:00Z', 'fp-preflight-in-v3', 'fp-preflight-out-v3')")
	exec("INSERT INTO run_steps (pipeline_run_id, step_name, step_status, input_artifact_id, output_artifact_id, started_at, finished_at, input_fingerprint, output_fingerprint) VALUES (4, 'parse', 'completed', 5, 6, '2024-04-05T11:01:00Z', '2024-04-05T11:05:00Z', 'fp-parse-in-v3', 'fp-parse-out-v3')")
	exec("INSERT INTO run_steps (pipeline_run_id, step_name, step_status, input_artifact_id, output_artifact_id, started_at, finished_at, input_fingerprint, output_fingerprint) VALUES (5, 'preflight', 'completed', 7, 7, '2025-07-20T09:00:00Z', '2025-07-20T09:01:00Z', 'fp-preflight-in-v4', 'fp-preflight-out-v4')")

	// 8. Works (27 DOIs for 27 work revisions)
	for i := 1; i <= 27; i++ {
		exec("INSERT INTO works (doi) VALUES (?)", sprintf("10.1000/%d", i))
	}

	// 9. Work revisions (27, distributed across the 6 runs)
	// Helper: work_revision records
	// [work_id, pipeline_run_id, payload_hash, title, year, journal, source, citation_count, reference_count, producer_stage, extension_data, abstract, keywords, keywords_plus]
	type rev struct {
		workID, runID int
		hash, title   string
		year          int
		journal       string
		source        string
		citations     int
		refs          int
		stage         string
		extension     string
		abstract      string
		keywords      string
		keywordsPlus  string
	}

	revisions := []rev{
		// Run 1 (completed, enrichment enabled) — 10 revisions
		{1, 1, "ph-dl-attn", "Attention Mechanisms in Transformer Models", 2024, "Nature", "scopus", 150, 45, "normalize", `{"validation_status":"valid"}`, "We study attention mechanisms in transformer models for deep learning.", `["transformer models","attention"]`, `["deep learning","nlp"]`},
		{2, 1, "ph-dl-rl", "Deep Reinforcement Learning for Robotics", 2023, "Science", "ieee", 89, 32, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{3, 1, "ph-dl-cnn", "Convolutional Neural Networks for Image Recognition", 2022, "IEEE Access", "scopus", 234, 67, "normalize", `{"validation_status":"valid"}`, "Convolutional neural networks for image recognition.", `["convolutional neural networks"]`, `["image recognition"]`},
		{4, 1, "ph-dl-gnn", "Graph Neural Networks: A Comprehensive Survey", 2024, "PLOS ONE", "wos", 45, 78, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{5, 1, "ph-dl-gan", "Generative Adversarial Networks: Advances and Challenges", 2021, "Nature", "scopus", 312, 56, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{6, 1, "ph-dl-tl", "Transfer Learning in Natural Language Processing", 2023, "ACL", "scopus", 67, 23, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{7, 1, "ph-dl-ssl", "Self-Supervised Learning: A Review", 2024, "IEEE TPAMI", "ieee", 23, 89, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{8, 1, "ph-dl-snn", "Spiking Neural Networks for Edge Computing", 2020, "Frontiers", "wos", 12, 34, "validate", `{"validation_status":"discarded","validation_reasons":["insufficient data"]}`, "", "", ""},
		{9, 1, "ph-dl-fl", "Federated Learning: Privacy-Preserving Machine Learning", 2022, "Cell", "scopus", 78, 41, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{10, 1, "ph-dl-nas", "Neural Architecture Search: Methods and Applications", 2023, "ACM Computing Surveys", "scopus", 56, 67, "normalize", `{"validation_status":"valid"}`, "", "", ""},

		// Run 2 (failed) — 3 revisions
		{11, 2, "ph-dl-bayes", "Bayesian Deep Learning: A Review", 2023, "JMLR", "scopus", 34, 28, "enrich", `{"validation_status":"valid"}`, "", "", ""},
		{12, 2, "ph-dl-vae", "Variational Autoencoders in Medical Imaging", 2021, "Medical Image Analysis", "ieee", 45, 19, "enrich", `{"validation_status":"valid"}`, "", "", ""},
		{13, 2, "ph-dl-xai", "Explainable AI: Techniques and Challenges", 2024, "AI Ethics", "scopus", 67, 31, "enrich", `{"validation_status":"valid"}`, "", "", ""},

		// Run 3 (trashed) — 3 revisions
		{14, 3, "ph-qc-overview", "Quantum Machine Learning: An Overview", 2022, "Quantum", "scopus", 89, 42, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{15, 3, "ph-qc-ecc", "Quantum Error Correction Codes", 2023, "Physical Review", "scopus", 23, 56, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{16, 3, "ph-qc-anneal", "Quantum Annealing for Optimization", 2021, "Nature Physics", "wos", 45, 34, "validate", `{"validation_status":"discarded","validation_reasons":["out of scope"]}`, "", "", ""},

		// Run 4 (enrichment disabled) — 5 revisions
		{17, 4, "ph-cr-cas9", "CRISPR-Cas9: A Decade of Progress", 2024, "Science", "scopus", 156, 78, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{18, 4, "ph-cr-embryo", "Gene Editing in Human Embryos", 2023, "Nature", "wos", 234, 45, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{19, 4, "ph-cr-base", "Base Editing: A New Frontier", 2022, "Cell", "scopus", 89, 34, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{20, 4, "ph-cr-prime", "Prime Editing: Methods and Applications", 2023, "Nature Biotechnology", "scopus", 67, 23, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{21, 4, "ph-cr-ethics", "Ethical Considerations in Gene Editing", 2024, "Bioethics", "scopus", 12, 56, "normalize", `{"validation_status":"valid"}`, "", "", ""},

		// Run 5 (very recent) — 4 revisions
		{22, 5, "ph-llm-survey", "Large Language Models: A Comprehensive Survey", 2025, "Nature", "scopus", 456, 89, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{23, 5, "ph-multimodal", "Multimodal AI Systems", 2025, "Science", "ieee", 234, 67, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{24, 5, "ph-ai-safety", "AI Safety and Alignment Research", 2025, "AI Magazine", "scopus", 89, 45, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{25, 5, "ph-foundation-robotics", "Foundation Models in Robotics", 2025, "IEEE Robotics", "ieee", 67, 34, "normalize", `{"validation_status":"valid"}`, "", "", ""},

		// Run 6 (search-3) — 2 revisions
		{26, 6, "ph-cr-screening", "CRISPR Screening in Cancer Research", 2024, "Cancer Cell", "scopus", 34, 23, "normalize", `{"validation_status":"valid"}`, "", "", ""},
		{27, 6, "ph-cr-epi", "Epigenetic Editing: Tools and Applications", 2023, "Nature Reviews", "wos", 45, 12, "normalize", `{"validation_status":"valid"}`, "", "", ""},
	}

	for _, r := range revisions {
		exec(`INSERT INTO work_revisions (work_id, pipeline_run_id, payload_hash, title, year, journal, source, citation_count, reference_count, producer_stage, extension_data, abstract, keywords, keywords_plus, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '2024-01-01T00:00:00Z')`,
			r.workID, r.runID, r.hash, r.title, r.year, r.journal, r.source, r.citations, r.refs, r.stage, r.extension, r.abstract, r.keywords, r.keywordsPlus)
	}

	// Populate the derived per-run term inventory and revision matches for run 1
	// through the same searchterms and repository code paths the pipeline uses.
	populateFixtureTermMatches(t, db, 1)

	// 10. Run work stages (validate, enrich, normalize for revisions in each run)
	// Run 1 stages
	for _, w := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} {
		outcome := "valid"
		if w == 8 {
			outcome = "discarded"
		}
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (1, ?, 'validate', ?)", w, outcome)
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (1, ?, 'enrich', 'enriched')", w)
		if outcome == "valid" {
			exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (1, ?, 'normalize', 'normalized')", w)
		}
	}
	// Run 2 stages (failed after enrich)
	for _, w := range []int{11, 12, 13} {
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (2, ?, 'validate', 'valid')", w)
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (2, ?, 'enrich', 'enriched')", w)
	}
	// Run 3 stages (trashed, valid)
	for _, w := range []int{14, 15, 16} {
		outcome := "valid"
		if w == 16 {
			outcome = "discarded"
		}
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (3, ?, 'validate', ?)", w, outcome)
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (3, ?, 'enrich', 'enriched')", w)
		if outcome == "valid" {
			exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (3, ?, 'normalize', 'normalized')", w)
		}
	}
	// Run 4 stages (enrichment disabled — skip enrich)
	for _, w := range []int{17, 18, 19, 20, 21} {
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (4, ?, 'validate', 'valid')", w)
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (4, ?, 'normalize', 'normalized')", w)
	}
	// Run 5 stages (recent)
	for _, w := range []int{22, 23, 24, 25} {
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (5, ?, 'validate', 'valid')", w)
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (5, ?, 'enrich', 'enriched')", w)
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (5, ?, 'normalize', 'normalized')", w)
	}
	// Run 6 stages
	for _, w := range []int{26, 27} {
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (6, ?, 'validate', 'valid')", w)
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (6, ?, 'enrich', 'enriched')", w)
		exec("INSERT INTO run_work_stages (pipeline_run_id, work_id, stage_name, outcome) VALUES (6, ?, 'normalize', 'normalized')", w)
	}

	// 11. People (6 with validated ORCIDs)
	exec("INSERT INTO people (orcid) VALUES ('0000-0002-1825-0097')") // Ada Lovelace
	exec("INSERT INTO people (orcid) VALUES ('0000-0003-1234-5678')") // Alan Turing
	exec("INSERT INTO people (orcid) VALUES ('0000-0001-2345-6789')") // John von Neumann
	exec("INSERT INTO people (orcid) VALUES ('0000-0002-3333-4444')") // Ada Lovelace (duplicate name)
	exec("INSERT INTO people (orcid) VALUES ('0000-0002-5555-6666')") // Marie Curie
	exec("INSERT INTO people (orcid) VALUES ('0000-0002-7777-8888')") // Nikola Tesla

	// 12. Author occurrences (11)
	exec("INSERT INTO author_occurrences (person_id, citation_name, first_name, last_name, orcid) VALUES (1, 'Ada Lovelace', 'Ada', 'Lovelace', '0000-0002-1825-0097')")
	exec("INSERT INTO author_occurrences (person_id, citation_name, first_name, last_name) VALUES (NULL, 'Charles Babbage', 'Charles', 'Babbage')")
	exec("INSERT INTO author_occurrences (person_id, citation_name, first_name, last_name, orcid) VALUES (2, 'Alan Turing', 'Alan', 'Turing', '0000-0003-1234-5678')")
	exec("INSERT INTO author_occurrences (person_id, citation_name, first_name, last_name) VALUES (NULL, 'Grace Hopper', 'Grace', 'Hopper')")
	exec("INSERT INTO author_occurrences (person_id, citation_name, first_name, last_name, orcid) VALUES (3, 'John von Neumann', 'John', 'von Neumann', '0000-0001-2345-6789')")
	exec("INSERT INTO author_occurrences (person_id, citation_name, first_name, last_name) VALUES (NULL, 'Claude Shannon', 'Claude', 'Shannon')")
	exec("INSERT INTO author_occurrences (person_id, citation_name, first_name, last_name, orcid) VALUES (4, 'Ada Lovelace', 'Ada', 'Lovelace', '0000-0002-3333-4444')") // duplicate name, different ORCID
	exec("INSERT INTO author_occurrences (person_id, citation_name, first_name, last_name, orcid) VALUES (5, 'Marie Curie', 'Marie', 'Curie', '0000-0002-5555-6666')")
	exec("INSERT INTO author_occurrences (person_id, citation_name, first_name, last_name) VALUES (NULL, 'Albert Einstein', 'Albert', 'Einstein')")
	exec("INSERT INTO author_occurrences (person_id, citation_name, first_name, last_name) VALUES (NULL, 'Rosalind Franklin', 'Rosalind', 'Franklin')")
	exec("INSERT INTO author_occurrences (person_id, citation_name, first_name, last_name, orcid) VALUES (6, 'Nikola Tesla', 'Nikola', 'Tesla', '0000-0002-7777-8888')")

	// 13. Authorships (15+, linking authors to revisions)
	// Revision 1-10 (Run 1): Ada Lovelace, Charles Babbage, Alan Turing
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (1, 1, 1, 'Analytical Society')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (1, 2, 2, 'Babbage Institute')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (2, 3, 1, 'Cambridge University')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (2, 4, 2, 'Harvard University')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (3, 5, 1, 'Institute for Advanced Study')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (3, 6, 2, 'Bell Labs')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (4, 1, 1, 'Analytical Society')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (4, 7, 2, 'Different University')") // duplicate Ada Lovelace
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (5, 8, 1, 'Sorbonne University')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (5, 9, 2, 'ETH Zurich')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (6, 10, 1, 'King''s College London')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (6, 11, 2, 'Tesla实验室')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (7, 3, 1, 'Cambridge University')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (7, 4, 2, 'Harvard University')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (8, 5, 1, 'Institute for Advanced Study')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (9, 1, 1, 'Analytical Society')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (10, 2, 1, 'Babbage Institute')")
	exec("INSERT INTO author_identity_resolutions (pipeline_run_id, author_occurrence_id, status, provider, queried_citation_name, resolved_at) VALUES (1, 2, 'orcid_is_unclear', 'orcid', 'Charles Babbage', '2024-01-20T10:15:00Z')")
	exec("INSERT INTO author_identity_candidates (identity_resolution_id, candidate_orcid, query_url, payload_artifact_id, provider_rank) VALUES (1, '0000-0001-2345-6789', 'https://orcid.example/search?q=Charles+Babbage', 1, 1), (1, '0000-0002-1825-0097', 'https://orcid.example/search?q=Charles+Babbage', 1, 2)")

	// Revisions 17-21 (Run 4, enrichment disabled): Marie Curie, Rosalind Franklin
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (17, 8, 1, 'Institut du Radium')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (17, 10, 2, 'Birkbeck College')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (18, 8, 1, 'Institut du Radium')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (19, 10, 1, 'Birkbeck College')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (20, 8, 1, 'Institut du Radium')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (21, 9, 1, 'ETH Zurich')")

	// Revisions 22-25 (Run 5, very recent): Nikola Tesla, Grace Hopper
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (22, 11, 1, 'Tesla实验室')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (22, 4, 2, 'Harvard University')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (23, 11, 1, 'Tesla实验室')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (24, 4, 1, 'Harvard University')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (25, 11, 1, 'Tesla实验室')")

	// Revisions 26-27 (Run 6): Albert Einstein
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (26, 9, 1, 'ETH Zurich')")
	exec("INSERT INTO authorships (work_revision_id, author_occurrence_id, author_order, affiliation) VALUES (27, 9, 1, 'ETH Zurich')")

	// 14. Reference mentions (20+)
	// Revision 1 (Attention Mechanisms) — 4 mentions
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (1, 1, '10.1000/2', 'Deep Reinforcement Learning for Robotics', 'Alan Turing')")
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (1, 2, '10.1000/3', 'Convolutional Neural Networks for Image Recognition', 'John von Neumann')")
	exec("INSERT INTO reference_mentions (work_revision_id, resolved_work_id, mention_order, doi, title, author) VALUES (1, 4, 3, '10.1000/4', 'Graph Neural Networks: A Comprehensive Survey', 'Ada Lovelace')")
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (1, 4, '10.external/vaswani2017', 'Attention Is All You Need', 'Vaswani et al.')")

	// Revision 2 (Deep RL) — 3 mentions
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (2, 1, '10.external/mnih2015', 'Human-level control through deep reinforcement learning', 'Mnih et al.')")
	exec("INSERT INTO reference_mentions (work_revision_id, resolved_work_id, mention_order, doi, title, author) VALUES (2, 1, 2, '10.1000/1', 'Attention Mechanisms in Transformer Models', 'Ada Lovelace')")
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (2, 3, '10.external/silver2016', 'Mastering the game of Go with deep neural networks', 'Silver et al.')")

	// Revision 3 (CNN) — 3 mentions
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (3, 1, '10.external/krizhevsky2012', 'ImageNet Classification with Deep Convolutional Neural Networks', 'Krizhevsky et al.')")
	exec("INSERT INTO reference_mentions (work_revision_id, resolved_work_id, mention_order, doi, title, author) VALUES (3, 2, 2, '10.1000/2', 'Deep Reinforcement Learning for Robotics', 'Alan Turing')")
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (3, 3, '10.external/he2016', 'Deep Residual Learning for Image Recognition', 'He et al.')")

	// Revision 5 (GAN) — 3 mentions
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (5, 1, '10.external/goodfellow2014', 'Generative Adversarial Nets', 'Goodfellow et al.')")
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (5, 2, '10.external/radford2015', 'Unsupervised Representation Learning with Deep Convolutional GANs', 'Radford et al.')")
	exec("INSERT INTO reference_mentions (work_revision_id, resolved_work_id, mention_order, doi, title, author) VALUES (5, 3, 3, '10.1000/3', 'Convolutional Neural Networks for Image Recognition', 'John von Neumann')")

	// Revision 17 (CRISPR-Cas9) — 3 mentions
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (17, 1, '10.external/jinek2012', 'A programmable dual-RNA-guided DNA endonuclease in adaptive bacterial immunity', 'Jinek et al.')")
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (17, 2, '10.external/cong2013', 'Multiplex genome engineering using CRISPR/Cas systems', 'Cong et al.')")
	exec("INSERT INTO reference_mentions (work_revision_id, resolved_work_id, mention_order, doi, title, author) VALUES (17, 19, 3, '10.1000/19', 'Base Editing: A New Frontier', 'Marie Curie')")

	// Revision 22 (LLM Survey) — 3 mentions
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (22, 1, '10.external/devlin2018', 'BERT: Pre-training of Deep Bidirectional Transformers', 'Devlin et al.')")
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (22, 2, '10.external/brown2020', 'Language Models are Few-Shot Learners', 'Brown et al.')")
	exec("INSERT INTO reference_mentions (work_revision_id, resolved_work_id, mention_order, doi, title, author) VALUES (22, 1, 3, '10.1000/1', 'Attention Mechanisms in Transformer Models', 'Ada Lovelace')")

	// Revision 26 (CRISPR Screening) — 2 mentions
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (26, 1, '10.external/shalem2014', 'Genome-scale CRISPR-Cas9 knockout screening in human cells', 'Shalem et al.')")
	exec("INSERT INTO reference_mentions (work_revision_id, resolved_work_id, mention_order, doi, title, author) VALUES (26, 17, 2, '10.1000/17', 'CRISPR-Cas9: A Decade of Progress', 'Marie Curie')")

	// Revision 27 (Epigenetic Editing) — 2 mentions
	exec("INSERT INTO reference_mentions (work_revision_id, mention_order, doi, title, author) VALUES (27, 1, '10.external/liu2016', 'Targeted genome editing', 'Liu et al.')")
	exec("INSERT INTO reference_mentions (work_revision_id, resolved_work_id, mention_order, doi, title, author) VALUES (27, 19, 2, '10.1000/19', 'Base Editing: A New Frontier', 'Marie Curie')")

	// Total: 4+3+3+3+3+3+2+2 = 23 reference mentions

	// 15. Pipeline run metrics
	// Run 1: full metrics
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'input_records', '', 12)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'parsed_articles', '', 10)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'deduplicated_articles', '', 10)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'duplicate_articles', '', 2)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'enrichment_candidates', '', 10)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'enriched_article_updates', '', 8)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'valid_articles', '', 9)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'discarded_articles', '', 1)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'normalized_articles_processed', '', 9)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'normalization_fields_processed', '', 48)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'normalization_fields_changed', '', 6)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'normalization_fields_already_canonical', '', 42)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'normalization_fields_unavailable', '', 0)")
	for _, field := range []struct {
		name                                 string
		processed, changed, canonical, empty int
	}{
		{"publisher", 9, 0, 9, 0},
		{"journal", 9, 1, 8, 0},
		{"author_name", 15, 0, 15, 0},
		{"affiliation", 15, 5, 10, 0},
	} {
		exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'normalization_fields_processed', ?, ?)", field.name, field.processed)
		exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'normalization_fields_changed', ?, ?)", field.name, field.changed)
		exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'normalization_fields_already_canonical', ?, ?)", field.name, field.canonical)
		exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'normalization_fields_unavailable', ?, ?)", field.name, field.empty)
	}
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'input_records', 'scopus', 8)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'input_records', 'ieee', 4)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'parsed_articles', 'scopus', 7)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'parsed_articles', 'ieee', 3)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'cache_hits', 'crossref', 5)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'cache_misses', 'crossref', 3)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'cache_hits', 'openalex', 4)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'cache_misses', 'openalex', 2)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'cache_network_fetches', 'crossref', 3)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'enriched_fields_total', '', 10)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'enriched_fields_title', '', 2)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'enriched_fields_abstract', '', 2)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'enriched_fields_publisher', '', 2)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'enriched_fields_citation_count', '', 1)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'enriched_fields_references', '', 1)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'enriched_fields_authors', '', 2)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'enriched_fields', 'crossref', 5)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (1, 'enriched_fields', 'openalex', 5)")

	// Run 2: partial metrics (failed before completion)
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (2, 'input_records', '', 3)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (2, 'parsed_articles', '', 3)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (2, 'deduplicated_articles', '', 3)")

	// Run 4: enrichment disabled metrics
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (4, 'input_records', '', 5)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (4, 'parsed_articles', '', 5)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (4, 'deduplicated_articles', '', 5)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (4, 'enrichment_skipped', '', 1)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (4, 'valid_articles', '', 5)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (4, 'normalized_articles_processed', '', 5)")

	// Run 5: recent metrics
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (5, 'input_records', '', 4)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (5, 'parsed_articles', '', 4)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (5, 'deduplicated_articles', '', 4)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (5, 'valid_articles', '', 4)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (5, 'normalized_articles_processed', '', 4)")

	// Run 6: metrics
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (6, 'input_records', '', 2)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (6, 'parsed_articles', '', 2)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (6, 'deduplicated_articles', '', 2)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (6, 'valid_articles', '', 2)")
	exec("INSERT INTO pipeline_run_metrics (pipeline_run_id, metric, source, value) VALUES (6, 'normalized_articles_processed', '', 2)")

	// 16. Cache entries (provider responses)
	exec("INSERT INTO cache_entries (provider, namespace, request_fingerprint, response_status, payload_artifact_id, fetched_at, expires_at, extractor_version) VALUES ('crossref', 'doi', 'req-crossref-1', 200, 3, '2024-01-20T10:10:00Z', '2024-04-20T10:10:00Z', 'v1')")
	exec("INSERT INTO cache_entries (provider, namespace, request_fingerprint, response_status, payload_artifact_id, fetched_at, expires_at, extractor_version) VALUES ('crossref', 'doi', 'req-crossref-2', 200, 3, '2024-01-20T10:10:00Z', '2024-04-20T10:10:00Z', 'v1')")
	exec("INSERT INTO cache_entries (provider, namespace, request_fingerprint, response_status, fetched_at, expires_at, extractor_version) VALUES ('crossref', 'doi', 'req-crossref-miss-1', 404, '2024-01-20T10:15:00Z', '2024-01-27T10:15:00Z', 'v1')")
	exec("INSERT INTO cache_entries (provider, namespace, request_fingerprint, response_status, payload_artifact_id, fetched_at, expires_at, extractor_version) VALUES ('openalex', 'doi', 'req-openalex-1', 200, 3, '2024-01-20T10:20:00Z', '2024-04-20T10:20:00Z', 'v1')")
	exec("INSERT INTO cache_entries (provider, namespace, request_fingerprint, response_status, payload_artifact_id, fetched_at, expires_at, extractor_version) VALUES ('openalex', 'doi', 'req-openalex-2', 200, 3, '2024-01-20T10:20:00Z', '2024-04-20T10:20:00Z', 'v1')")
	exec("INSERT INTO cache_entries (provider, namespace, request_fingerprint, response_status, fetched_at, expires_at, extractor_version) VALUES ('openalex', 'doi', 'req-openalex-stale', 200, '2024-01-20T10:20:00Z', '2023-12-20T10:20:00Z', 'v1')") // expired

	// 17. Run cache uses (provenance)
	exec("INSERT INTO run_cache_uses (pipeline_run_id, cache_entry_id, cache_layer, outcome) VALUES (1, 1, 'active_run', 'hit')")
	exec("INSERT INTO run_cache_uses (pipeline_run_id, cache_entry_id, cache_layer, outcome) VALUES (1, 2, 'active_run', 'hit')")
	exec("INSERT INTO run_cache_uses (pipeline_run_id, cache_entry_id, cache_layer, outcome) VALUES (1, 3, 'active_run', 'negative')")
	exec("INSERT INTO run_cache_uses (pipeline_run_id, cache_entry_id, cache_layer, outcome) VALUES (1, 4, 'global', 'hit')")
	exec("INSERT INTO run_cache_uses (pipeline_run_id, cache_entry_id, cache_layer, outcome) VALUES (1, 5, 'global', 'hit')")
	exec("INSERT INTO run_cache_uses (pipeline_run_id, cache_entry_id, cache_layer, outcome) VALUES (1, 6, 'global', 'stale')")
	exec("INSERT INTO run_cache_uses (pipeline_run_id, cache_entry_id, cache_layer, outcome) VALUES (5, 1, 'global', 'hit')")
	exec("INSERT INTO run_cache_uses (pipeline_run_id, cache_entry_id, cache_layer, outcome) VALUES (5, 4, 'global', 'hit')")

	// 18. Audit events (for multiple entity types)
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2024-01-20T10:00:00Z', 'pipeline', 1, 'pipeline_run', '1', 'pipeline_started', '{\"search_id\":\"deep-learning-nlp\",\"revision\":\"r1\"}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2024-01-20T10:30:00Z', 'pipeline', 1, 'pipeline_run', '1', 'pipeline_completed', '{\"status\":\"completed\"}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2024-01-20T10:10:00Z', 'crossref', 1, 'work_revision', '1', 'field_enriched', '{\"field\":\"title\",\"provider\":\"crossref\"}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2024-01-20T10:10:00Z', 'crossref', 1, 'work_revision', '1', 'field_enriched', '{\"field\":\"abstract\",\"provider\":\"crossref\"}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2024-01-20T10:10:00Z', 'crossref', 1, 'work_revision', '1', 'field_enriched', '{\"field\":\"publisher\",\"provider\":\"crossref\"}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2024-01-20T10:10:00Z', 'openalex', 1, 'work_revision', '2', 'field_enriched', '{\"field\":\"authors\",\"provider\":\"openalex\"}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2024-01-20T10:10:00Z', 'openalex', 1, 'work_revision', '2', 'field_enriched', '{\"field\":\"citation_count\",\"provider\":\"openalex\"}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2024-01-20T10:10:00Z', 'crossref', 1, 'work_revision', '3', 'field_enriched', '{\"field\":\"title\",\"provider\":\"crossref\"}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2024-01-20T10:10:00Z', 'crossref', 1, 'work_revision', '3', 'field_enriched', '{\"field\":\"references\",\"provider\":\"crossref\"}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2024-01-20T10:10:00Z', 'openalex', 1, 'work_revision', '4', 'field_enriched', '{\"field\":\"abstract\",\"provider\":\"openalex\"}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2024-01-20T10:10:00Z', 'openalex', 1, 'work_revision', '5', 'field_enriched', '{\"field\":\"authors\",\"provider\":\"openalex\"}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2024-01-20T10:10:00Z', 'openalex', 1, 'work_revision', '5', 'field_enriched', '{\"field\":\"publisher\",\"provider\":\"openalex\"}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2024-01-20T10:15:00Z', 'pipeline', 1, 'work_revision', '8', 'validation_discarded', '{\"reasons\":[\"insufficient data\"]}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2024-02-15T08:15:00Z', 'pipeline', 2, 'pipeline_run', '2', 'pipeline_failed', '{\"error\":\"enrichment timeout\"}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2024-03-15T09:00:00Z', 'user', 3, 'pipeline_run', '3', 'pipeline_trashed', '{\"reason\":\"Incorrect deduplication settings\"}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json, correlation_id) VALUES ('2024-04-05T11:00:00Z', 'pipeline', 4, 'pipeline_run', '4', 'pipeline_started', '{\"search_id\":\"deep-learning-nlp\",\"revision\":\"r3\",\"enrichment_enabled\":false}', 'corr-4-start')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json, correlation_id) VALUES ('2024-04-05T11:20:00Z', 'pipeline', 4, 'pipeline_run', '4', 'pipeline_completed', '{\"status\":\"completed\",\"enrichment_enabled\":false}', 'corr-4-end')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json) VALUES ('2025-07-20T09:00:00Z', 'pipeline', 5, 'pipeline_run', '5', 'pipeline_started', '{\"search_id\":\"quantum-computing\",\"revision\":\"r1\"}')")
	exec("INSERT INTO audit_events (occurred_at, actor, pipeline_run_id, entity_type, entity_id, action, metadata_json, correlation_id) VALUES ('2024-05-12T16:00:00Z', 'pipeline', 6, 'pipeline_run', '6', 'pipeline_started', '{\"search_id\":\"crispr-gene-editing\",\"revision\":\"r1\"}', 'corr-6-start')")

	// 19. Bound normalized PDF inventory and mirrored audit evidence
	ctx := context.Background()
	if err := pdfstore.BindStore(ctx, db.DB, "workspace.fixture.pdf.db"); err != nil {
		t.Fatal(err)
	}
	tmpPDFPath := filepath.Join(tmpDir, "workspace.fixture.pdf.db")
	pdfs, err := pdfstore.Open(tmpPDFPath, filepath.Join("..", "..", "config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.DB.Query(`SELECT w.doi, w.id, wr.pipeline_run_id
		FROM works w JOIN work_revisions wr ON wr.work_id=w.id
		WHERE wr.producer_stage='normalize' ORDER BY w.id`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var doi string
		var workID, pipelineRunID int64
		if err := rows.Scan(&doi, &workID, &pipelineRunID); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if _, err := pdfs.Register(ctx, doi, workID, pipelineRunID); err != nil {
			rows.Close()
			t.Fatal(err)
		}
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := pdfs.Add(ctx, "10.1000/1", 1, deterministicFixturePDF("Selectable fixture methods on page one", "Selectable fixture conclusions on page two")); err != nil {
		t.Fatal(err)
	}
	if _, err := pdfs.DB.Exec(`UPDATE pdf_documents
		SET inventoried_at='2024-01-20T10:25:00Z', updated_at='2024-01-20T10:25:00Z'
		WHERE doi='10.1000/1';
		UPDATE pdf_audit_outbox SET occurred_at='2024-01-20T10:25:00Z',
			event_key='fixture-pdf-' || action || '-' || entity_id,
			correlation_id='fixture-pdf-' || action || '-' || entity_id`); err != nil {
		t.Fatal(err)
	}
	if _, err := pdfs.FlushAuditOutbox(ctx, db.DB); err != nil {
		t.Fatal(err)
	}
	normalizeFixtureTimestamps(t, pdfs.DB)
	if _, err := pdfs.DB.Exec(sprintf("PRAGMA user_version=%d", viewerFixtureContractVersion)); err != nil {
		t.Fatalf("set PDF fixture contract version: %v", err)
	}
	if _, err := pdfs.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatalf("PDF WAL checkpoint: %v", err)
	}
	if err := pdfs.Close(); err != nil {
		t.Fatalf("close PDF database: %v", err)
	}

	// Normalize migration and default timestamps so browser output does not depend on fixture generation time.
	normalizeFixtureTimestamps(t, db.DB)
	if _, err := db.DB.Exec(sprintf("PRAGMA user_version=%d", viewerFixtureContractVersion)); err != nil {
		t.Fatalf("set metadata fixture contract version: %v", err)
	}

	// Checkpoint WAL and close
	if _, err := db.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatalf("wal checkpoint: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	// Copy the temporary databases to their output paths.
	os.MkdirAll(filepath.Dir(outputPath), 0755)
	copyFixture := func(source, destination string) int {
		input, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("read temp fixture: %v", err)
		}
		if err := os.WriteFile(destination, input, 0644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		return len(input)
	}
	metadataBytes := copyFixture(tmpPath, outputPath)
	pdfBytes := copyFixture(tmpPDFPath, outputPDFPath)
	t.Logf("fixtures written to %s (%d bytes) and %s (%d bytes)", outputPath, metadataBytes, outputPDFPath, pdfBytes)
}

// normalizeFixtureTimestamps replaces SQLite-generated timestamps with one fixed instant while preserving explicit fixture evidence.
func normalizeFixtureTimestamps(t *testing.T, db *sql.DB) {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("list fixture tables: %v", err)
	}
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			rows.Close()
			t.Fatalf("scan fixture table: %v", err)
		}
		if !validFixtureIdentifier(table) {
			rows.Close()
			t.Fatalf("unsafe fixture table identifier %q", table)
		}
		tables = append(tables, table)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close fixture table rows: %v", err)
	}
	for _, table := range tables {
		var protected int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master
			WHERE type='trigger' AND tbl_name=? AND lower(sql) LIKE '%before update%'`, table).Scan(&protected); err != nil {
			t.Fatalf("inspect update protection for %s: %v", table, err)
		}
		if protected > 0 {
			continue
		}
		columns, err := db.Query(`SELECT name FROM pragma_table_info(?)
			WHERE name IN ('applied_at', 'created_at', 'updated_at', 'enriched_at', 'used_at') ORDER BY cid`, table)
		if err != nil {
			t.Fatalf("list timestamp columns for %s: %v", table, err)
		}
		var names []string
		for columns.Next() {
			var name string
			if err := columns.Scan(&name); err != nil {
				columns.Close()
				t.Fatalf("scan timestamp column for %s: %v", table, err)
			}
			names = append(names, name)
		}
		if err := columns.Close(); err != nil {
			t.Fatalf("close timestamp columns for %s: %v", table, err)
		}
		for _, name := range names {
			query := sprintf(`UPDATE "%s" SET "%s"='2024-01-01T00:00:00Z'
				WHERE "%s" IS NOT NULL AND instr("%s", 'T')=0`, table, name, name, name)
			if _, err := db.Exec(query); err != nil {
				t.Fatalf("normalize %s.%s: %v", table, name, err)
			}
		}
	}
}

// validFixtureIdentifier reports whether a discovered SQLite identifier is safe to quote in fixture-only SQL.
func validFixtureIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && char != '_' {
			return false
		}
	}
	return true
}

// populateFixtureTermMatches computes and stores the derived per-run term
// inventory and revision matches for one run through the same searchterms and
// TermMatches code paths the pipeline uses.
func populateFixtureTermMatches(t *testing.T, db *database.Database, runID int64) {
	t.Helper()
	rows, err := db.DB.Query(`SELECT source_name, query FROM run_sources
		WHERE pipeline_run_id=? AND query IS NOT NULL AND trim(query)<>'' ORDER BY id`, runID)
	if err != nil {
		t.Fatal(err)
	}
	queries := map[string]string{}
	for rows.Next() {
		var source, query string
		if err := rows.Scan(&source, &query); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		queries[source] = query
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	terms := searchterms.ParseSources(queries)
	termsBySource := map[string][]string{}
	for _, term := range terms {
		for _, source := range term.Sources {
			termsBySource[source] = append(termsBySource[source], term.Text)
		}
	}
	revRows, err := db.DB.Query(`SELECT id, title, abstract, keywords, keywords_plus
		FROM work_revisions WHERE pipeline_run_id=? AND producer_stage='normalize' ORDER BY id`, runID)
	if err != nil {
		t.Fatal(err)
	}
	matches := map[int64]map[string][]string{}
	for revRows.Next() {
		var revisionID int64
		var title, abstract, keywords, keywordsPlus sql.NullString
		if err := revRows.Scan(&revisionID, &title, &abstract, &keywords, &keywordsPlus); err != nil {
			revRows.Close()
			t.Fatal(err)
		}
		fieldMatches := searchterms.MatchFields(title.String, abstract.String, fixtureKeywordArray(keywords), fixtureKeywordArray(keywordsPlus), terms)
		hasMatch := false
		for _, list := range fieldMatches {
			if len(list) > 0 {
				hasMatch = true
				break
			}
		}
		if hasMatch {
			matches[revisionID] = fieldMatches
		}
	}
	if err := revRows.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.TermMatches.ReplaceRunTermData(runID, termsBySource, matches); err != nil {
		t.Fatal(err)
	}
}

// fixtureKeywordArray decodes a stored keyword TEXT value into an array with a
// raw-text fallback, mirroring the pipeline's stored-value handling.
func fixtureKeywordArray(raw sql.NullString) []string {
	if !raw.Valid || raw.String == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw.String), &arr); err == nil {
		return arr
	}
	return []string{raw.String}
}

// sprintf is a convenience wrapper for fmt.Sprintf used in the fixture generator.
func sprintf(format string, args ...any) string {
	// Use fmt.Sprintf via the testing package's mechanism
	return fmt.Sprintf(format, args...)
}
