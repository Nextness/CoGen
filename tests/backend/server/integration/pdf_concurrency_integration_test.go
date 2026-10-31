// pdf_concurrency_integration_test.go verifies PDF mutations, reads, anchors,
// and audit delivery against independent server connections.
package server_test

import (
	serverhelpers "analysis/tests/support/go/backend/server"
	"analysis/tests/support/go/pdffixture"

	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// pdfHTTPResult is one non-fatal mutation result used by concurrent callers.
type pdfHTTPResult struct {
	code int
	body map[string]any
	err  error
}

// callPDFMutation invokes one PDF mutation without failing a goroutine.
func callPDFMutation(handler http.Handler, method string, runID, revisionID int64, data []byte, etag string) pdfHTTPResult {
	path := fmt.Sprintf("/api/runs/%d/articles/%d/pdf", runID, revisionID)
	request := httptest.NewRequest(method, path, bytes.NewReader(data))
	request.Header.Set("Content-Type", "application/pdf")
	request.Header.Set("X-PDF-Size", fmt.Sprint(len(data)))
	request.Header.Set("X-PDF-SHA256", pdfDigest(data))
	if etag != "" {
		request.Header.Set("If-Match", etag)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		return pdfHTTPResult{code: recorder.Code, err: err}
	}
	return pdfHTTPResult{code: recorder.Code, body: body}
}

// callAnchorVersion invokes one anchor-version mutation without failing a goroutine.
func callAnchorVersion(handler http.Handler, runID int64, anchorID, payload string) pdfHTTPResult {
	path := fmt.Sprintf("/api/runs/%d/anchors/%s/versions", runID, anchorID)
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(payload))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		return pdfHTTPResult{code: recorder.Code, err: err}
	}
	return pdfHTTPResult{code: recorder.Code, body: body}
}

// callAnchorCreate invokes one anchor creation without failing a goroutine.
func callAnchorCreate(handler http.Handler, runID, revisionID int64, label, hash string) pdfHTTPResult {
	path := fmt.Sprintf("/api/runs/%d/articles/%d/anchors", runID, revisionID)
	payload := fmt.Sprintf(`{"label":%q,"page":1,"selected_text":"Concurrent methods","rectangles":[{"x":0.1,"y":0.2,"width":0.3,"height":0.1}],"pdf_content_hash":%q}`, label, hash)
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(payload))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		return pdfHTTPResult{code: recorder.Code, err: err}
	}
	return pdfHTTPResult{code: recorder.Code, body: body}
}

// TestPDFMutationConcurrentServersExactlyOneWins verifies two independent
// server connections cannot both commit the same document counter.
func TestPDFMutationConcurrentServersExactlyOneWins(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	second, err := serverhelpers.OpenViewer(t, fixture.MetadataPath)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	status := pdfStatusForWork(t, fixture.Server.Handler(), fixture.AvailableID)
	etag := status["mutation_etag"].(string)
	if warm := pdfContentRequest(t, second.Handler(), http.MethodGet, fmt.Sprintf("/api/pdf/%d", fixture.AvailableID), nil); warm.Code != http.StatusOK {
		t.Fatalf("second server cache warm = %d", warm.Code)
	}
	firstBytes := pdffixture.Bytes("First concurrent winner")
	secondBytes := pdffixture.Bytes("Second concurrent winner")
	start := make(chan struct{})
	results := make(chan pdfHTTPResult, 2)
	go func() {
		<-start
		results <- callPDFMutation(fixture.Server.Handler(), http.MethodPut, fixture.RunID, fixture.RevisionID, firstBytes, etag)
	}()
	go func() {
		<-start
		results <- callPDFMutation(second.Handler(), http.MethodPut, fixture.RunID, fixture.RevisionID, secondBytes, etag)
	}()
	close(start)
	var succeeded, conflicted int
	var winner []byte
	for range 2 {
		select {
		case result := <-results:
			if result.err != nil {
				t.Fatal(result.err)
			}
			switch result.code {
			case http.StatusOK:
				succeeded++
				hash := result.body["pdf_status"].(map[string]any)["content_hash"].(string)
				if hash == pdfDigest(firstBytes) {
					winner = firstBytes
				} else if hash == pdfDigest(secondBytes) {
					winner = secondBytes
				}
			case http.StatusPreconditionFailed:
				conflicted++
			default:
				t.Fatalf("unexpected concurrent mutation = %d %v", result.code, result.body)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent replacement deadlocked")
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent replacement succeeded=%d conflicted=%d", succeeded, conflicted)
	}
	for name, handler := range map[string]http.Handler{"first": fixture.Server.Handler(), "second": second.Handler()} {
		response := pdfContentRequest(t, handler, http.MethodGet, fmt.Sprintf("/api/pdf/%d", fixture.AvailableID), nil)
		if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), winner) {
			t.Fatalf("%s server read after replacement = %d bytes=%d", name, response.Code, response.Body.Len())
		}
	}

	current := pdfStatusForWork(t, fixture.Server.Handler(), fixture.AvailableID)
	currentETag := current["mutation_etag"].(string)
	raceReplacement := pdffixture.Bytes("Remove and replace race")
	removalResult := make(chan pdfHTTPResult, 1)
	replacementResult := make(chan pdfHTTPResult, 1)
	go func() {
		removalResult <- callPDFMutation(second.Handler(), http.MethodDelete, fixture.RunID, fixture.RevisionID, nil, currentETag)
	}()
	go func() {
		replacementResult <- callPDFMutation(fixture.Server.Handler(), http.MethodPut, fixture.RunID, fixture.RevisionID, raceReplacement, currentETag)
	}()
	removal := <-removalResult
	replacement := <-replacementResult
	if removal.err != nil || replacement.err != nil {
		t.Fatalf("concurrent remove/replace errors = %v %v", removal.err, replacement.err)
	}
	successes := 0
	for _, result := range []pdfHTTPResult{removal, replacement} {
		if result.code == http.StatusOK {
			successes++
		} else if result.code != http.StatusPreconditionFailed {
			t.Fatalf("unexpected remove/replace result = %d %v", result.code, result.body)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent remove/replace successes = %d", successes)
	}
	finalFirst := pdfStatusForWork(t, fixture.Server.Handler(), fixture.AvailableID)
	finalSecond := pdfStatusForWork(t, second.Handler(), fixture.AvailableID)
	if finalFirst["status"] != finalSecond["status"] || finalFirst["change_counter"] != finalSecond["change_counter"] {
		t.Fatalf("servers disagree after remove/replace: first=%v second=%v", finalFirst, finalSecond)
	}
}

// TestPDFAnchorRaceWithReplacementKeepsHashesBound verifies concurrent anchor
// creation and replacement never commits an anchor with a different hash.
func TestPDFAnchorRaceWithReplacementKeepsHashesBound(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	second, err := serverhelpers.OpenViewer(t, fixture.MetadataPath)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if status, _ := mutationJSON(t, fixture.Server.Handler(), http.MethodPost, fmt.Sprintf("/api/runs/%d/review-context", fixture.RunID), `{"parent_context_id":null}`, ""); status != http.StatusCreated {
		t.Fatalf("create context status=%d", status)
	}
	knownHashes := map[string]bool{}
	for index := 0; index < 6; index++ {
		status := pdfStatusForWork(t, fixture.Server.Handler(), fixture.AvailableID)
		hash := status["content_hash"].(string)
		etag := status["mutation_etag"].(string)
		knownHashes[hash] = true
		replacement := pdffixture.Bytes(fmt.Sprintf("Race replacement %d", index))
		knownHashes[pdfDigest(replacement)] = true
		start := make(chan struct{})
		anchorResult := make(chan pdfHTTPResult, 1)
		replaceResult := make(chan pdfHTTPResult, 1)
		go func() {
			<-start
			anchorResult <- callAnchorCreate(fixture.Server.Handler(), fixture.RunID, fixture.RevisionID, fmt.Sprintf("race-anchor-%d", index), hash)
		}()
		go func() {
			<-start
			replaceResult <- callPDFMutation(second.Handler(), http.MethodPut, fixture.RunID, fixture.RevisionID, replacement, etag)
		}()
		close(start)
		anchor := <-anchorResult
		replace := <-replaceResult
		if anchor.err != nil || replace.err != nil {
			t.Fatalf("race %d errors = %v %v", index, anchor.err, replace.err)
		}
		if anchor.code == http.StatusCreated {
			version := anchor.body["anchor"].(map[string]any)["version"].(map[string]any)
			if version["pdf_content_hash"] != hash {
				t.Fatalf("race %d anchor hash = %v want %s", index, version["pdf_content_hash"], hash)
			}
		} else if anchor.code != http.StatusConflict {
			t.Fatalf("race %d anchor result = %d %v", index, anchor.code, anchor.body)
		}
		if replace.code != http.StatusOK && replace.code != http.StatusPreconditionFailed {
			t.Fatalf("race %d replacement result = %d %v", index, replace.code, replace.body)
		}
	}
	finalStatus := pdfStatusForWork(t, fixture.Server.Handler(), fixture.AvailableID)
	finalHash := finalStatus["content_hash"].(string)
	finalReplacement := pdffixture.Bytes("Final race replacement")
	finalResult := callPDFMutation(second.Handler(), http.MethodPut, fixture.RunID, fixture.RevisionID, finalReplacement, finalStatus["mutation_etag"].(string))
	if finalResult.err != nil || finalResult.code != http.StatusOK {
		t.Fatalf("final replacement = %d %v err=%v", finalResult.code, finalResult.body, finalResult.err)
	}
	stalePin := pdfContentRequest(t, second.Handler(), http.MethodGet, fmt.Sprintf("/api/pdf/%d?content_hash=%s", fixture.AvailableID, finalHash), nil)
	if stalePin.Code != http.StatusConflict {
		t.Fatalf("stale historical pin result = %d", stalePin.Code)
	}
	newHash := pdfDigest(finalReplacement)
	newPin := pdfContentRequest(t, second.Handler(), http.MethodGet, fmt.Sprintf("/api/pdf/%d?content_hash=%s", fixture.AvailableID, newHash), map[string]string{"Range": "bytes=0-4"})
	if newPin.Code != http.StatusPartialContent || newPin.Body.String() != "%PDF-" {
		t.Fatalf("new pinned range = %d %q", newPin.Code, newPin.Body.String())
	}
	knownHashes[newHash] = true
	status, anchors := serverhelpers.RequestJSON(t, fixture.Server.Handler(), fmt.Sprintf("/api/runs/%d/articles/%d/anchors?limit=50", fixture.RunID, fixture.RevisionID))
	if status != http.StatusOK {
		t.Fatalf("anchors status=%d", status)
	}
	for _, raw := range anchors["items"].([]any) {
		hash := raw.(map[string]any)["version"].(map[string]any)["pdf_content_hash"].(string)
		if !knownHashes[hash] {
			t.Fatalf("anchor committed with unknown hash %s", hash)
		}
	}

	restoreStatus := pdfStatusForWork(t, fixture.Server.Handler(), fixture.AvailableID)
	restoreHash := restoreStatus["content_hash"].(string)
	restoreETag := restoreStatus["mutation_etag"].(string)
	createPath := fmt.Sprintf("/api/runs/%d/articles/%d/anchors", fixture.RunID, fixture.RevisionID)
	code, created := mutationJSON(t, fixture.Server.Handler(), http.MethodPost, createPath, fmt.Sprintf(`{"label":"restore-race","page":1,"selected_text":"Restore race","rectangles":[{"x":0.1,"y":0.2,"width":0.3,"height":0.1}],"pdf_content_hash":"%s"}`, restoreHash), "")
	if code != http.StatusCreated {
		t.Fatalf("create restore race anchor = %d %v", code, created)
	}
	restoreAnchor := created["anchor"].(map[string]any)
	restoreID := restoreAnchor["id"].(string)
	restoreVersionID := int64(restoreAnchor["version"].(map[string]any)["id"].(float64))
	code, tombstoned := mutationJSON(t, fixture.Server.Handler(), http.MethodPost, fmt.Sprintf("/api/runs/%d/anchors/%s/versions", fixture.RunID, restoreID), fmt.Sprintf(`{"expected_version_id":%d,"state":"deleted","page":0,"selected_text":"","rectangles":[]}`, restoreVersionID), "")
	if code != http.StatusCreated {
		t.Fatalf("tombstone restore race anchor = %d %v", code, tombstoned)
	}
	tombstoneID := int64(tombstoned["anchor"].(map[string]any)["version"].(map[string]any)["id"].(float64))
	restorePayload := fmt.Sprintf(`{"expected_version_id":%d,"state":"active","restore_from_version_id":%d,"page":0,"selected_text":"","rectangles":[],"pdf_content_hash":"%s"}`, tombstoneID, restoreVersionID, restoreHash)
	restoreResult := make(chan pdfHTTPResult, 1)
	removeResult := make(chan pdfHTTPResult, 1)
	go func() {
		restoreResult <- callAnchorVersion(fixture.Server.Handler(), fixture.RunID, restoreID, restorePayload)
	}()
	go func() {
		removeResult <- callPDFMutation(second.Handler(), http.MethodDelete, fixture.RunID, fixture.RevisionID, nil, restoreETag)
	}()
	restored := <-restoreResult
	removed := <-removeResult
	if restored.err != nil || removed.err != nil {
		t.Fatalf("restore race errors = %v %v", restored.err, removed.err)
	}
	if restored.code == http.StatusCreated {
		version := restored.body["anchor"].(map[string]any)["version"].(map[string]any)
		if version["pdf_content_hash"] != restoreHash {
			t.Fatalf("restored anchor hash = %v want %s", version["pdf_content_hash"], restoreHash)
		}
	} else if restored.code != http.StatusConflict {
		t.Fatalf("restore race result = %d %v", restored.code, restored.body)
	}
	if removed.code != http.StatusOK && removed.code != http.StatusPreconditionFailed {
		t.Fatalf("restore/remove race removal = %d %v", removed.code, removed.body)
	}
}

// TestPDFAuditRelayRunsWithConcurrentMutations verifies two relays and
// alternating mutations drain without deadlock or duplicate delivery errors.
func TestPDFAuditRelayRunsWithConcurrentMutations(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	second, err := serverhelpers.OpenViewer(t, fixture.MetadataPath)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	fixture.Server.StartAuditRelay(t.Context())
	defer fixture.Server.StopAuditRelay()
	second.StartAuditRelay(t.Context())
	defer second.StopAuditRelay()
	handlers := []http.Handler{fixture.Server.Handler(), second.Handler()}
	for index := 0; index < 6; index++ {
		status := pdfStatusForWork(t, handlers[0], fixture.AvailableID)
		data := pdffixture.Bytes(fmt.Sprintf("Relay mutation %d", index))
		result := callPDFMutation(handlers[index%2], http.MethodPut, fixture.RunID, fixture.RevisionID, data, status["mutation_etag"].(string))
		if result.err != nil || result.code != http.StatusOK {
			t.Fatalf("relay mutation %d = %d %v err=%v", index, result.code, result.body, result.err)
		}
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		firstStatus, firstHealth := serverhelpers.RequestJSON(t, handlers[0], "/api/health")
		secondStatus, secondHealth := serverhelpers.RequestJSON(t, handlers[1], "/api/health")
		if firstStatus == http.StatusOK && secondStatus == http.StatusOK && firstHealth["pdf_audit_pending"] == float64(0) && secondHealth["pdf_audit_pending"] == float64(0) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("audit relay did not drain: first=%v second=%v", firstHealth, secondHealth)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
