// scenario_test.go runs identical scenarios against independently built old
// and current code. The parent test copies it as main.go into each snapshot.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"analysis/pdfstore"
	"analysis/server"
)

// bindings adapts names and inputs only; each closure calls its snapshot's
// original implementation rather than a shared implementation of persistence.
type bindings struct {
	raw          *sql.DB
	close        func() error
	start        func(bool) (int64, int, error)
	finish       func(int64, string) error
	step         func(int64, string) (int64, error)
	stepStatus   func(int64, string) error
	stepLinks    func(int64, int64, int64) error
	readSteps    func(int64) (any, error)
	readRun      func(int64) (any, error)
	readRuns     func() (any, error)
	purge        func(int64) (any, error)
	reviewer     func(bool) (any, error)
	recover      func(int64) error
	artifact     func(string, string, int64, []byte) (int64, error)
	readArtifact func(int64) (any, error)
	link         func(int64, string) error
	appendCache  func(int, *int64, string) (int64, error)
	useCache     func(int64, int64, string, string) error
	findCache    func(int64, bool) (any, error)
	latestCache  func(bool) (any, error)
	flush        func(*pdfstore.Store) (int, error)
}

// must stops a scenario if an operation expected to succeed fails.
func must(err error) {
	if err != nil {
		panic(err)
	}
}

// check enforces expected behavior independently of the cross-version comparison.
func check(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

// execute runs controlled fixture or failure-injection SQL.
func execute(db *sql.DB, query string, args ...any) { _, err := db.Exec(query, args...); must(err) }

// digest identifies deterministic fixture bytes.
func digest(data []byte) string { value := sha256.Sum256(data); return hex.EncodeToString(value[:]) }

// main seeds a production-migrated fixture or executes one cloned-fixture case.
func main() {
	root, scenario := os.Args[1], os.Args[2]
	db, err := openBindings(filepath.Join(root, "metadata.db"), "config/database.something")
	must(err)
	defer db.close()
	pdf, err := pdfstore.Open(filepath.Join(root, "corpus.pdf.db"), "config/database.something")
	must(err)
	defer pdf.Close()
	if scenario == "seed" {
		seed(db.raw, pdf.DB)
		return
	}
	results := map[string]any{}
	switch scenario {
	case "run":
		runLifecycle(db, results)
	case "run-concurrency":
		runConcurrency(db, results)
	case "artifact":
		artifactIntegrity(db, results)
	case "cache":
		cacheReplay(db, results)
	case "review":
		reviewFlow(root, db, results)
	case "review-rollback":
		reviewRollback(root, db, results)
	case "review-concurrency":
		reviewConcurrency(root, results)
	case "pdf":
		pdfFlow(db, pdf, results)
	case "pdf-savepoint":
		pdfSavepoints(db, pdf, results)
	case "pdf-crash":
		pdfCrash(db, pdf, results)
	default:
		panic("unknown scenario")
	}
	results["metadata"] = snapshot(db.raw, []string{"pipeline_runs", "run_steps", "artifacts", "artifact_blobs", "run_artifacts", "cache_entries", "run_cache_uses", "review_contexts", "review_context_work_heads", "work_review_versions", "work_review_version_substatuses", "review_notes", "review_note_versions", "review_note_links", "review_context_note_heads", "review_anchors", "review_anchor_versions", "review_context_anchor_heads", "audit_events", "pdf_audit_links"})
	results["pdf"] = snapshot(pdf.DB, []string{"pdf_documents", "pdf_blobs", "pdf_audit_outbox"})
	identities := map[string]string{}
	rows, err := db.raw.Query("SELECT id, label FROM review_anchors ORDER BY rowid")
	must(err)
	for rows.Next() {
		var id, label string
		must(rows.Scan(&id, &label))
		identities[id] = "anchor:" + label
	}
	must(rows.Err())
	must(rows.Close())
	rows, err = pdf.DB.Query("SELECT event_key, correlation_id FROM pdf_audit_outbox ORDER BY rowid")
	must(err)
	index := 0
	for rows.Next() {
		var key, correlation string
		must(rows.Scan(&key, &correlation))
		index++
		identities[key] = fmt.Sprint("event:", index)
		identities[correlation] = fmt.Sprint("correlation:", index)
	}
	must(rows.Err())
	must(rows.Close())
	data, err := json.Marshal(results)
	must(err)
	var decoded any
	must(json.Unmarshal(data, &decoded))
	normalized := normalize(decoded, "", identities)
	file, err := os.Create(filepath.Join(root, "result.json"))
	must(err)
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	must(encoder.Encode(normalized))
}

// seed creates deterministic rows using schemas created by production migrations.
func seed(db, pdf *sql.DB) {
	execute(db, "INSERT INTO searches(id,search_id) VALUES(1,'parity')")
	execute(db, "INSERT INTO search_revisions(id,search_id,revision_label,config_artifact_hash,resolved_manifest_hash) VALUES(1,1,'r1','config','manifest')")
	execute(db, "INSERT INTO execution_plans(id,search_revision_id,execution_fingerprint,resolved_manifest_hash,input_manifest_hash) VALUES(1,1,'fingerprint','manifest','input')")
	for id := 1; id <= 4; id++ {
		status := "completed"
		if id == 3 {
			status = "running"
		}
		execute(db, "INSERT INTO pipeline_runs(id,step,execution_plan_id,attempt_number,started_at,status) VALUES(?,'parity',1,?,?,?)", id, id, fmt.Sprintf("2026-01-%02dT00:00:00Z", id), status)
	}
	execute(db, "INSERT INTO works(id,doi) VALUES(1,'10.1000/parity')")
	for id := 1; id <= 2; id++ {
		execute(db, "INSERT INTO work_revisions(id,work_id,pipeline_run_id,payload_hash,producer_stage,title) VALUES(?,1,?,'payload','normalize','Parity article')", id, id)
		execute(db, "INSERT INTO run_work_stages(pipeline_run_id,work_id,stage_name,outcome) VALUES(?,1,'validate','valid')", id)
	}
	execute(db, "INSERT INTO pdf_store_binding(id,relative_path,configured_at,config_fingerprint) VALUES(1,'corpus.pdf.db','2026-01-01T00:00:00Z',?)", strings.Repeat("0", 64))
	data := []byte("%PDF-1.7\nparity fixture")
	execute(pdf, "INSERT INTO pdf_blobs(content_hash,byte_size,data,created_at) VALUES(?,?,?,'2026-01-01T00:00:00Z')", digest(data), len(data), data)
	execute(pdf, "INSERT INTO pdf_documents(doi,status,content_hash,inventoried_at,updated_at) VALUES('10.1000/parity','available',?,'2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')", digest(data))
}

// errorText requires failure and retains exact error text for comparison.
func errorText(err error) string { check(err != nil, "expected failure"); return err.Error() }

// runLifecycle compares attempt numbering, step timestamps, recovery, and rollback.
func runLifecycle(db *bindings, result map[string]any) {
	_, _, err := db.start(true)
	result["active"] = errorText(err)
	must(db.finish(3, "completed"))
	id, attempt, err := db.start(true)
	must(err)
	check(attempt == 5, "attempt number")
	result["attempt"] = []int64{id, int64(attempt)}
	step, err := db.step(id, "parse")
	must(err)
	must(db.stepStatus(step, "running"))
	must(db.stepStatus(step, "pending"))
	value, err := db.readSteps(id)
	must(err)
	result["pending_steps"] = value
	must(db.stepStatus(step, "running"))
	data := []byte("step artifact")
	artifact, err := db.artifact(digest(data), "text/plain", int64(len(data)), data)
	must(err)
	must(db.stepLinks(step, 1, artifact))
	value, err = db.purge(1)
	must(err)
	result["shared_purge"] = value
	value, err = db.reviewer(false)
	must(err)
	result["absent_reviewer"] = value
	value, err = db.reviewer(true)
	must(err)
	result["reviewer"] = value
	result["invalid_status"] = errorText(db.finish(id, "invalid"))
	execute(db.raw, "CREATE TRIGGER reject_recovery BEFORE INSERT ON audit_events BEGIN SELECT RAISE(ABORT,'parity audit failure'); END")
	result["rollback"] = errorText(db.recover(id))
	result["after_failure"] = snapshot(db.raw, []string{"pipeline_runs", "run_steps", "audit_events"})
	execute(db.raw, "DROP TRIGGER reject_recovery")
	must(db.recover(id))
	result["terminal"] = errorText(db.recover(id))
	value, err = db.readRun(id)
	must(err)
	result["recovered_run"] = value
	value, err = db.readRun(999)
	must(err)
	result["absent_run"] = value
	value, err = db.readRuns()
	must(err)
	result["run_order"] = value
	value, err = db.readSteps(id)
	must(err)
	result["recovered_steps"] = value
	value, err = db.purge(id)
	must(err)
	result["purge"] = value
}

// runConcurrency compares competing active-attempt exclusion without depending
// on which goroutine wins the write lock.
func runConcurrency(db *bindings, result map[string]any) {
	must(db.finish(3, "completed"))
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes, conflicts := 0, 0
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, attempt, err := db.start(true)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				check(attempt == 5, "concurrent attempt number")
				successes++
			} else {
				check(strings.Contains(err.Error(), "already has running attempt"), err.Error())
				conflicts++
			}
		}()
	}
	wg.Wait()
	check(successes == 1 && conflicts == 5, "active attempt exclusion")
	result["successes"] = successes
	result["conflicts"] = conflicts
}

// artifactIntegrity compares idempotency, metadata/byte conflicts, atomic
// rollback, and immutable run-role assignment.
func artifactIntegrity(db *bindings, result map[string]any) {
	data := []byte("artifact bytes")
	hash := digest(data)
	id, err := db.artifact(hash, "text/plain", int64(len(data)), data)
	must(err)
	again, err := db.artifact(hash, "text/plain", int64(len(data)), data)
	must(err)
	check(again == id, "artifact replay identity")
	value, err := db.readArtifact(id)
	must(err)
	result["identity"] = value
	value, err = db.readArtifact(999)
	must(err)
	result["absent"] = value
	_, err = db.artifact(hash, "application/json", int64(len(data)), data)
	result["media_conflict"] = errorText(err)
	_, err = db.artifact(hash, "text/plain", int64(len(data)), []byte("different data"))
	result["byte_conflict"] = errorText(err)
	_, err = db.artifact(digest([]byte("size")), "text/plain", 100, []byte("size"))
	result["size_conflict"] = errorText(err)
	must(db.link(id, "workspace_config"))
	must(db.link(id, "workspace_config"))
	other, err := db.artifact(digest([]byte("other")), "text/plain", 5, []byte("other"))
	must(err)
	result["role_conflict"] = errorText(db.link(other, "workspace_config"))
	execute(db.raw, "CREATE TRIGGER reject_blob BEFORE INSERT ON artifact_blobs BEGIN SELECT RAISE(ABORT,'parity blob failure'); END")
	_, err = db.artifact(digest([]byte("rollback")), "text/plain", 8, []byte("rollback"))
	result["rollback"] = errorText(err)
	execute(db.raw, "DROP TRIGGER reject_blob")
}

// cacheReplay compares immutable response history, layer selection, replay,
// nullable payload/expiry, and append-only enforcement.
func cacheReplay(db *bindings, result map[string]any) {
	data := []byte("response")
	artifact, err := db.artifact(digest(data), "application/json", int64(len(data)), data)
	must(err)
	first, err := db.appendCache(200, &artifact, "2030-01-01T00:00:00Z")
	must(err)
	must(db.useCache(1, first, "active_run", "hit"))
	must(db.useCache(1, first, "global", "hit"))
	second, err := db.appendCache(404, nil, "")
	must(err)
	check(second != first, "cache versions must append")
	for _, global := range []bool{false, true} {
		value, err := db.latestCache(global)
		must(err)
		result[fmt.Sprint("latest_", global)] = value
	}
	for _, anyLayer := range []bool{false, true} {
		value, err := db.findCache(1, anyLayer)
		must(err)
		result[fmt.Sprint("replay_", anyLayer)] = value
	}
	_, err = db.raw.Exec("UPDATE cache_entries SET response_status=201 WHERE id=?", first)
	result["immutable"] = errorText(err)
	result["invalid_use"] = errorText(db.useCache(1, second, "active_run", "invalid"))
}

// request runs a real handler without requiring network sockets and retains
// status, response body, and exact statement count in the comparison.
func request(viewer *server.Server, result map[string]any, name, method, path string, data any, status int) map[string]any {
	encoded, err := json.Marshal(data)
	must(err)
	req := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	viewer.Handler().ServeHTTP(response, req)
	check(response.Code == status, fmt.Sprintf("%s: status=%d want=%d body=%s", name, response.Code, status, response.Body.String()))
	var body map[string]any
	must(json.Unmarshal(response.Body.Bytes(), &body))
	result[name] = map[string]any{"status": response.Code, "body": body, "queries": response.Header().Get("X-Query-Count-Used")}
	return body
}

// reviewFlow compares decision/note/anchor version graphs, no-ops, stale
// conflicts, inheritance, tombstones, restore, and lifecycle audit.
func reviewFlow(root string, db *bindings, result map[string]any) {
	viewer, err := server.Open(filepath.Join(root, "metadata.db"))
	must(err)
	defer viewer.Close()
	call := func(name, method, path string, data any, status int) map[string]any {
		return request(viewer, result, name, method, path, data, status)
	}
	call("context", "POST", "/api/runs/1/review-context", map[string]any{"parent_context_id": nil}, 201)
	decision := map[string]any{"expected_version_id": nil, "status": "not_approved", "sub_statuses": []string{"duplicate"}, "reason": "parity"}
	first := call("decision", "PUT", "/api/runs/1/articles/1/review", decision, 200)["review"].(map[string]any)["version"].(map[string]any)["id"]
	decision["expected_version_id"] = first
	call("decision_noop", "PUT", "/api/runs/1/articles/1/review", decision, 200)
	decision["expected_version_id"] = nil
	call("decision_conflict", "PUT", "/api/runs/1/articles/1/review", decision, 409)
	note := call("note", "POST", "/api/runs/1/articles/1/notes", map[string]any{"body": "Parity [[article:10.1000/parity]] [[ext:https://example.test|x]]"}, 201)["note"].(map[string]any)
	noteID := int64(note["id"].(float64))
	noteVersion := note["version"].(map[string]any)["id"]
	anchor := call("anchor", "POST", "/api/runs/1/articles/1/anchors", map[string]any{"label": "methods", "page": 1, "selected_text": "parity", "rectangles": []map[string]float64{{"x": .1, "y": .2, "width": .3, "height": .1}}}, 201)["anchor"].(map[string]any)
	anchorID := anchor["id"].(string)
	anchorVersion := anchor["version"].(map[string]any)["id"]
	call("anchor_noop", "POST", "/api/runs/1/anchors/"+anchorID+"/versions", map[string]any{"expected_version_id": anchorVersion, "state": "active", "page": 1, "selected_text": "parity", "rectangles": []map[string]float64{{"x": .1, "y": .2, "width": .3, "height": .1}}}, 200)
	call("note_noop", "POST", fmt.Sprintf("/api/runs/1/notes/%d/versions", noteID), map[string]any{"expected_version_id": noteVersion, "state": "active", "body": "Parity [[article:10.1000/parity]] [[ext:https://example.test|x]]"}, 200)
	call("child", "POST", "/api/runs/2/review-context", map[string]any{"parent_context_id": 1}, 201)
	call("parent_conflict", "POST", "/api/runs/2/review-context", map[string]any{"parent_context_id": nil}, 409)
	decision["expected_version_id"] = first
	decision["status"] = "approved"
	decision["sub_statuses"] = []string{}
	call("child_decision", "PUT", "/api/runs/2/articles/2/review", decision, 200)
	path := fmt.Sprintf("/api/runs/2/notes/%d/versions", noteID)
	edit := map[string]any{"expected_version_id": noteVersion, "state": "active", "body": "Child note"}
	note = call("child_note", "POST", path, edit, 201)["note"].(map[string]any)
	call("note_conflict", "POST", path, edit, 409)
	edit["expected_version_id"] = note["version"].(map[string]any)["id"]
	edit["state"] = "deleted"
	edit["body"] = nil
	note = call("delete_note", "POST", path, edit, 201)["note"].(map[string]any)
	edit["expected_version_id"] = note["version"].(map[string]any)["id"]
	edit["state"] = "active"
	edit["body"] = "Restored note"
	call("restore_note", "POST", path, edit, 201)
	path = "/api/runs/2/anchors/" + anchorID + "/versions"
	anchorEdit := map[string]any{"expected_version_id": anchorVersion, "state": "deleted"}
	deleted := call("delete_anchor", "POST", path, anchorEdit, 201)["anchor"].(map[string]any)
	call("anchor_conflict", "POST", path, anchorEdit, 409)
	anchorEdit = map[string]any{"expected_version_id": deleted["version"].(map[string]any)["id"], "state": "active", "page": 1, "selected_text": "restored", "rectangles": []map[string]float64{{"x": .2, "y": .3, "width": .3, "height": .1}}}
	call("restore_anchor", "POST", path, anchorEdit, 201)
	for _, item := range []struct{ name, path string }{{"parent_review", "/api/runs/1/articles/1/review"}, {"parent_notes", "/api/runs/1/articles/1/notes"}, {"parent_anchors", "/api/runs/1/articles/1/anchors"}, {"child_review_versions", "/api/runs/2/articles/2/review/versions"}, {"child_notes", "/api/runs/2/articles/2/notes"}, {"child_anchors", "/api/runs/2/articles/2/anchors"}} {
		call(item.name, "GET", item.path, nil, 200)
	}
	call("trash", "PUT", "/api/runs/4/visibility", map[string]any{"visibility_state": "trashed", "reason": "parity"}, 200)
	call("restore", "PUT", "/api/runs/4/visibility", map[string]any{"visibility_state": "active", "reason": ""}, 200)
	value, err := db.purge(1)
	must(err)
	result["review_lineage_purge"] = value
}

// reviewRollback compares complete rollback after an injected mutation-audit
// failure, including context heads, version children, and logical identities.
func reviewRollback(root string, db *bindings, result map[string]any) {
	viewer, err := server.Open(filepath.Join(root, "metadata.db"))
	must(err)
	defer viewer.Close()
	execute(db.raw, "CREATE TRIGGER reject_review_audit BEFORE INSERT ON audit_events BEGIN SELECT RAISE(ABORT,'parity review audit failure'); END")
	request(viewer, result, "context_rollback", "POST", "/api/runs/1/review-context", map[string]any{"parent_context_id": nil}, 500)
	execute(db.raw, "DROP TRIGGER reject_review_audit")
	request(viewer, result, "context", "POST", "/api/runs/1/review-context", map[string]any{"parent_context_id": nil}, 201)
	execute(db.raw, "CREATE TRIGGER reject_review_audit BEFORE INSERT ON audit_events BEGIN SELECT RAISE(ABORT,'parity review audit failure'); END")
	request(viewer, result, "visibility_rollback", "PUT", "/api/runs/4/visibility", map[string]any{"visibility_state": "trashed", "reason": "rollback"}, 500)
	request(viewer, result, "decision_rollback", "PUT", "/api/runs/1/articles/1/review", map[string]any{"expected_version_id": nil, "status": "not_approved", "sub_statuses": []string{"duplicate"}}, 500)
	request(viewer, result, "note_rollback", "POST", "/api/runs/1/articles/1/notes", map[string]any{"body": "Rollback [[ext:https://example.test|x]]"}, 500)
	request(viewer, result, "anchor_rollback", "POST", "/api/runs/1/articles/1/anchors", map[string]any{"label": "rollback", "page": 1, "selected_text": "rollback", "rectangles": []map[string]float64{{"x": .1, "y": .2, "width": .3, "height": .1}}}, 500)
	execute(db.raw, "DROP TRIGGER reject_review_audit")
}

// reviewConcurrency requires one success and one optimistic conflict from
// competing decision, note, and anchor saves against the same expected head.
func reviewConcurrency(root string, result map[string]any) {
	viewer, err := server.Open(filepath.Join(root, "metadata.db"))
	must(err)
	defer viewer.Close()
	request(viewer, result, "context", "POST", "/api/runs/1/review-context", map[string]any{"parent_context_id": nil}, 201)
	decision := request(viewer, result, "decision", "PUT", "/api/runs/1/articles/1/review", map[string]any{"expected_version_id": nil, "status": "in_progress", "sub_statuses": []string{}}, 200)["review"].(map[string]any)["version"].(map[string]any)["id"]
	note := request(viewer, result, "note", "POST", "/api/runs/1/articles/1/notes", map[string]any{"body": "Initial note"}, 201)["note"].(map[string]any)
	anchor := request(viewer, result, "anchor", "POST", "/api/runs/1/articles/1/anchors", map[string]any{"label": "concurrent", "page": 1, "selected_text": "first", "rectangles": []map[string]float64{{"x": .1, "y": .2, "width": .3, "height": .1}}}, 201)["anchor"].(map[string]any)
	for _, test := range []struct {
		name, method, path string
		body               any
		success            int
	}{
		{"decision", "PUT", "/api/runs/1/articles/1/review", map[string]any{"expected_version_id": decision, "status": "approved", "sub_statuses": []string{}}, 200},
		{"note", "POST", fmt.Sprintf("/api/runs/1/notes/%.0f/versions", note["id"]), map[string]any{"expected_version_id": note["version"].(map[string]any)["id"], "state": "active", "body": "Competing edit"}, 201},
		{"anchor", "POST", "/api/runs/1/anchors/" + anchor["id"].(string) + "/versions", map[string]any{"expected_version_id": anchor["version"].(map[string]any)["id"], "state": "active", "page": 1, "selected_text": "competing edit", "rectangles": []map[string]float64{{"x": .1, "y": .2, "width": .3, "height": .1}}}, 201},
	} {
		statuses := make(chan int, 2)
		var group sync.WaitGroup
		for i := 0; i < 2; i++ {
			group.Add(1)
			go func() {
				defer group.Done()
				encoded, err := json.Marshal(test.body)
				must(err)
				req := httptest.NewRequest(test.method, test.path, bytes.NewReader(encoded))
				req.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				viewer.Handler().ServeHTTP(response, req)
				statuses <- response.Code
			}()
		}
		group.Wait()
		close(statuses)
		counts := map[int]int{}
		for status := range statuses {
			counts[status]++
		}
		check(counts[test.success] == 1 && counts[409] == 1, "competing "+test.name+" saves")
		result[test.name+"_competition"] = counts
	}
}

// pdfFlow compares registration, validated bytes, deduplication, immutable
// selection, mutation rollback, and outbox delivery on the original stores.
func pdfFlow(db *bindings, pdf *pdfstore.Store, result map[string]any) {
	ctx := context.Background()
	registered, err := pdf.Register(ctx, "10.1000/new", 1, 1)
	must(err)
	check(registered, "PDF registration")
	data := []byte("%PDF-1.7\nnew fixture")
	added, err := pdf.Add(ctx, "10.1000/new", 1, data)
	must(err)
	result["add"] = added
	added, err = pdf.Add(ctx, "10.1000/new", 1, []byte("%PDF-1.7\nreplacement"))
	must(err)
	result["replay"] = added
	execute(pdf.DB, "CREATE TRIGGER reject_pdf_outbox BEFORE INSERT ON pdf_audit_outbox BEGIN SELECT RAISE(ABORT,'parity outbox failure'); END")
	_, err = pdf.Register(ctx, "10.1000/rollback", 1, 1)
	result["rollback"] = errorText(err)
	execute(pdf.DB, "DROP TRIGGER reject_pdf_outbox")
	count, err := db.flush(pdf)
	must(err)
	result["flushed"] = count
	count, err = db.flush(pdf)
	must(err)
	check(count == 0, "PDF duplicate delivery")
}

// outboxEvent inserts fixed identities to isolate delivery behavior from clocks.
func outboxEvent(pdf *pdfstore.Store, key, actor string, run int64) {
	execute(pdf.DB, `INSERT INTO pdf_audit_outbox(event_key,occurred_at,actor,pipeline_run_id,entity_type,entity_id,action,metadata_json,correlation_id) VALUES(?,'2026-01-01T00:00:00Z',?,?,'work','1','pdf_inventory_registered','{}',?)`, key, actor, run, key)
}

// pdfSavepoints compares bad-event isolation, bounded multi-batch draining,
// stale run references, audit/link atomicity, and successful retries.
func pdfSavepoints(db *bindings, pdf *pdfstore.Store, result map[string]any) {
	for i := 0; i < 103; i++ {
		actor := "good"
		if i == 1 {
			actor = "bad"
		}
		outboxEvent(pdf, fmt.Sprintf("event-%03d", i), actor, 999)
	}
	execute(db.raw, "CREATE TRIGGER reject_pdf_link BEFORE INSERT ON pdf_audit_links WHEN NEW.event_key='event-001' BEGIN SELECT RAISE(ABORT,'parity link failure'); END")
	count, err := db.flush(pdf)
	result["first_count"] = count
	result["failure"] = errorText(err)
	check(count == 102, "failed event blocked batch draining")
	result["after_failure"] = snapshot(db.raw, []string{"audit_events", "pdf_audit_links"})
	execute(db.raw, "DROP TRIGGER reject_pdf_link")
	count, err = db.flush(pdf)
	must(err)
	check(count == 1, "failed event retry")
	result["retry_count"] = count
}

// pdfCrash compares the committed-but-unmarked crash window and verifies
// retry does not duplicate the already committed audit event.
func pdfCrash(db *bindings, pdf *pdfstore.Store, result map[string]any) {
	outboxEvent(pdf, "crash", "pipeline", 1)
	execute(pdf.DB, "CREATE TRIGGER reject_mark BEFORE UPDATE ON pdf_audit_outbox BEGIN SELECT RAISE(ABORT,'parity marking failure'); END")
	count, err := db.flush(pdf)
	check(count == 0, "marked during injected failure")
	result["failure"] = errorText(err)
	result["committed"] = snapshot(db.raw, []string{"audit_events", "pdf_audit_links"})
	execute(pdf.DB, "DROP TRIGGER reject_mark")
	count, err = db.flush(pdf)
	must(err)
	check(count == 1, "crash retry delivery")
	result["retry_count"] = count
}

// snapshot retains every persisted column, including nulls and relationships,
// in insertion order; normalization handles only volatile identities/clocks.
func snapshot(db *sql.DB, tables []string) map[string]any {
	result := map[string]any{}
	for _, table := range tables {
		rows, err := db.Query("SELECT * FROM " + table + " ORDER BY rowid")
		must(err)
		columns, err := rows.Columns()
		must(err)
		items := []map[string]any{}
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			must(rows.Scan(pointers...))
			row := map[string]any{}
			for i, name := range columns {
				value := values[i]
				if data, ok := value.([]byte); ok {
					value = hex.EncodeToString(data)
				}
				row[name] = value
			}
			items = append(items, row)
		}
		must(rows.Err())
		must(rows.Close())
		result[table] = items
	}
	return result
}

// normalize preserves values and nullability while replacing call-time clocks
// and generated IDs consistently across returned values and persisted graphs.
func normalize(value any, key string, identities map[string]string) any {
	switch value := value.(type) {
	case map[string]any:
		for name, item := range value {
			value[name] = normalize(item, name, identities)
		}
		return value
	case []any:
		for i, item := range value {
			value[i] = normalize(item, key, identities)
		}
		return value
	case string:
		if strings.HasSuffix(key, "_json") {
			var document any
			if json.Unmarshal([]byte(value), &document) == nil {
				encoded, err := json.Marshal(normalize(document, "", identities))
				must(err)
				return string(encoded)
			}
		}
		if strings.HasSuffix(strings.ToLower(key), "_at") {
			for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
				if _, err := time.Parse(layout, value); err == nil {
					if layout == time.RFC3339Nano {
						return "<rfc3339>"
					}
					return "<seconds>"
				}
			}
		}
		keys := make([]string, 0, len(identities))
		for identity := range identities {
			keys = append(keys, identity)
		}
		sort.Strings(keys)
		for _, identity := range keys {
			value = strings.ReplaceAll(value, identity, identities[identity])
		}
		return value
	default:
		return value
	}
}
