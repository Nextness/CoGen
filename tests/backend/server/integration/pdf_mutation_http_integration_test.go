// pdf_mutation_http_integration_test.go exercises the public PDF mutation
// boundary through a real HTTPServer, including authority, admission,
// cancellation, and rejection behavior.
package server_test

import (
	serverhelpers "analysis/tests/support/go/backend/server"
	"analysis/tests/support/go/pdffixture"

	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// startPDFHTTPServer starts the configured loopback server on an OS-assigned port.
func startPDFHTTPServer(t *testing.T, fixture serverhelpers.PdfViewerFixture) (*http.Client, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpServer := fixture.Server.HTTPServer(listener.Addr().String())
	done := make(chan struct{})
	go func() {
		_ = httpServer.Serve(listener)
		close(done)
	}()
	t.Cleanup(func() {
		_ = httpServer.Close()
		<-done
	})
	return &http.Client{Timeout: 5 * time.Second}, "http://" + listener.Addr().String()
}

// pdfHTTPHeaders returns one valid raw upload header set.
func pdfHTTPHeaders(data []byte, etag string) http.Header {
	header := http.Header{}
	header.Set("Content-Type", "application/pdf")
	header.Set("X-PDF-Size", fmt.Sprint(len(data)))
	header.Set("X-PDF-SHA256", pdfDigest(data))
	if etag != "" {
		header.Set("If-Match", etag)
	}
	return header
}

// doPDFHTTP performs one real HTTP request and returns its status and body.
func doPDFHTTP(t *testing.T, client *http.Client, request *http.Request) (int, []byte, http.Header) {
	t.Helper()
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("HTTP request: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read HTTP response: %v", err)
	}
	return response.StatusCode, body, response.Header
}

// TestPDFMutationHTTPAuthorityAndRejections verifies the real server enforces
// Host, Origin, method, media, and precondition boundaries while old bytes stay readable.
func TestPDFMutationHTTPAuthorityAndRejections(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	client, baseURL := startPDFHTTPServer(t, fixture)
	original := pdfBytes(t, fixture.Server.Handler(), fixture.AvailableID)
	status := pdfStatusForWork(t, fixture.Server.Handler(), fixture.AvailableID)
	etag := status["mutation_etag"].(string)
	data := pdffixture.Bytes("Real HTTP rejection")
	path := fmt.Sprintf("/api/runs/%d/articles/%d/pdf", fixture.RunID, fixture.RevisionID)

	hostRequest, _ := http.NewRequest(http.MethodGet, baseURL+"/api/health", nil)
	hostRequest.Host = "localhost" + strings.TrimPrefix(baseURL, "http://127.0.0.1")
	if code, body, _ := doPDFHTTP(t, client, hostRequest); code != http.StatusBadRequest || !strings.Contains(string(body), "invalid_host") {
		t.Fatalf("Host rejection = %d %s", code, body)
	}

	originRequest, _ := http.NewRequest(http.MethodPut, baseURL+path, bytes.NewReader(data))
	originRequest.Header = pdfHTTPHeaders(data, etag)
	originRequest.Header.Set("Origin", "http://evil.test")
	if code, body, _ := doPDFHTTP(t, client, originRequest); code != http.StatusForbidden || !strings.Contains(string(body), "origin_rejected") {
		t.Fatalf("Origin rejection = %d %s", code, body)
	}
	assertPDFHTTPBytes(t, client, baseURL, fixture.AvailableID, original)

	methodRequest, _ := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if code, body, _ := doPDFHTTP(t, client, methodRequest); code != http.StatusMethodNotAllowed {
		t.Fatalf("method rejection = %d %s", code, body)
	}

	mediaRequest, _ := http.NewRequest(http.MethodPut, baseURL+path, bytes.NewReader(data))
	mediaRequest.Header = pdfHTTPHeaders(data, etag)
	mediaRequest.Header.Set("Content-Type", "application/json")
	if code, body, _ := doPDFHTTP(t, client, mediaRequest); code != http.StatusUnsupportedMediaType || !strings.Contains(string(body), "unsupported_media_type") {
		t.Fatalf("media rejection = %d %s", code, body)
	}

	missingRequest, _ := http.NewRequest(http.MethodPut, baseURL+path, bytes.NewReader(data))
	missingRequest.Header = pdfHTTPHeaders(data, "")
	if code, body, _ := doPDFHTTP(t, client, missingRequest); code != http.StatusPreconditionRequired || !strings.Contains(string(body), "pdf_precondition_required") {
		t.Fatalf("missing precondition = %d %s", code, body)
	}

	staleRequest, _ := http.NewRequest(http.MethodPut, baseURL+path, bytes.NewReader(data))
	staleRequest.Header = pdfHTTPHeaders(data, fmt.Sprintf("\"pdf-%d-%x\"", fixture.AvailableID, int64(1)))
	if code, body, _ := doPDFHTTP(t, client, staleRequest); code != http.StatusPreconditionFailed || !strings.Contains(string(body), "pdf_state_conflict") {
		t.Fatalf("stale precondition = %d %s", code, body)
	}
	assertPDFHTTPBytes(t, client, baseURL, fixture.AvailableID, original)

	truncatedRequest, _ := http.NewRequest(http.MethodPut, baseURL+path, bytes.NewReader(data[:len(data)-16]))
	truncatedRequest.Header = pdfHTTPHeaders(data, etag)
	if code, body, _ := doPDFHTTP(t, client, truncatedRequest); code != http.StatusUnprocessableEntity || !strings.Contains(string(body), "pdf_upload_mismatch") {
		t.Fatalf("truncated upload = %d %s", code, body)
	}
	assertPDFHTTPBytes(t, client, baseURL, fixture.AvailableID, original)

	mismatchRequest, _ := http.NewRequest(http.MethodPut, baseURL+path, bytes.NewReader(data))
	mismatchRequest.Header = pdfHTTPHeaders(data, etag)
	mismatchRequest.Header.Set("X-PDF-SHA256", strings.Repeat("0", 64))
	if code, body, _ := doPDFHTTP(t, client, mismatchRequest); code != http.StatusUnprocessableEntity || !strings.Contains(string(body), "pdf_upload_mismatch") {
		t.Fatalf("digest mismatch = %d %s", code, body)
	}
	assertPDFHTTPBytes(t, client, baseURL, fixture.AvailableID, original)
}

// TestPDFMutationHTTPCapacityAndReplacementRead verifies two retained uploads
// bound admission while the old PDF remains readable, then release capacity.
func TestPDFMutationHTTPCapacityAndReplacementRead(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	client, baseURL := startPDFHTTPServer(t, fixture)
	original := pdfBytes(t, fixture.Server.Handler(), fixture.AvailableID)
	status := pdfStatusForWork(t, fixture.Server.Handler(), fixture.AvailableID)
	etag := status["mutation_etag"].(string)
	path := fmt.Sprintf("/api/runs/%d/articles/%d/pdf", fixture.RunID, fixture.RevisionID)

	readers := make([]*io.PipeReader, 0, 2)
	writers := make([]*io.PipeWriter, 0, 2)
	results := make(chan int, 2)
	for range 2 {
		reader, writer := io.Pipe()
		request, _ := http.NewRequest(http.MethodPut, baseURL+path, reader)
		request.Header = pdfHTTPHeaders(make([]byte, 1024), etag)
		readers = append(readers, reader)
		writers = append(writers, writer)
		go func() {
			response, err := client.Do(request)
			if err != nil {
				results <- 0
				return
			}
			response.Body.Close()
			results <- response.StatusCode
		}()
		if _, err := writer.Write([]byte("%PDF-")); err != nil {
			t.Fatal(err)
		}
	}
	waitForPDFCapacity(t, client, baseURL, path, etag)
	assertPDFHTTPBytes(t, client, baseURL, fixture.AvailableID, original)
	for _, writer := range writers {
		_ = writer.Close()
	}
	for range readers {
		select {
		case code := <-results:
			if code != http.StatusUnprocessableEntity {
				t.Fatalf("truncated concurrent upload = %d", code)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("truncated upload did not release capacity")
		}
	}
	waitForPDFCapacityRelease(t, client, baseURL, path, etag, original)
	replacement := pdffixture.Bytes("Capacity release replacement")
	request, _ := http.NewRequest(http.MethodPut, baseURL+path, bytes.NewReader(replacement))
	request.Header = pdfHTTPHeaders(replacement, etag)
	code, body, _ := doPDFHTTP(t, client, request)
	if code != http.StatusOK {
		t.Fatalf("replacement after capacity release = %d %s", code, body)
	}
	assertPDFHTTPBytes(t, client, baseURL, fixture.AvailableID, replacement)
}

// TestPDFMutationHTTPLimitsAndCancellation verifies exact-limit, over-limit,
// chunked over-limit, and canceled uploads preserve the current selection.
func TestPDFMutationHTTPLimitsAndCancellation(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	client, baseURL := startPDFHTTPServer(t, fixture)
	original := pdfBytes(t, fixture.Server.Handler(), fixture.AvailableID)
	etag := pdfStatusForWork(t, fixture.Server.Handler(), fixture.AvailableID)["mutation_etag"].(string)
	path := fmt.Sprintf("/api/runs/%d/articles/%d/pdf", fixture.RunID, fixture.RevisionID)

	assertPDFHTTPBytes(t, client, baseURL, fixture.AvailableID, original)
	atLimit := make([]byte, 20_000_000)
	copy(atLimit, "%PDF-")
	atLimitRequest, _ := http.NewRequest(http.MethodPut, baseURL+path, bytes.NewReader(atLimit))
	atLimitRequest.Header = pdfHTTPHeaders(atLimit, etag)
	if code, body, _ := doPDFHTTP(t, client, atLimitRequest); code != http.StatusOK {
		t.Fatalf("exact-limit upload = %d %s", code, body)
	}
	updatedETag := pdfStatusForWork(t, fixture.Server.Handler(), fixture.AvailableID)["mutation_etag"].(string)

	overLimit := make([]byte, 20_000_001)
	copy(overLimit, "%PDF-")
	overRequest, _ := http.NewRequest(http.MethodPut, baseURL+path, bytes.NewReader(overLimit))
	overRequest.Header = pdfHTTPHeaders(overLimit, updatedETag)
	if code, body, _ := doPDFHTTP(t, client, overRequest); code != http.StatusRequestEntityTooLarge || !strings.Contains(string(body), "request_too_large") {
		t.Fatalf("over-limit upload = %d %s", code, body)
	}

	chunkedOver := make([]byte, 20_000_001)
	copy(chunkedOver, "%PDF-")
	chunkedRequest, _ := http.NewRequest(http.MethodPut, baseURL+path, io.NopCloser(bytes.NewReader(chunkedOver)))
	chunkedRequest.ContentLength = -1
	chunkedRequest.Header = pdfHTTPHeaders(chunkedOver, updatedETag)
	if code, body, _ := doPDFHTTP(t, client, chunkedRequest); code != http.StatusRequestEntityTooLarge || !strings.Contains(string(body), "request_too_large") {
		t.Fatalf("chunked over-limit upload = %d %s", code, body)
	}
	assertPDFHTTPBytes(t, client, baseURL, fixture.AvailableID, atLimit)

	cancelContext, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	reader, writer := io.Pipe()
	cancelRequest, _ := http.NewRequestWithContext(cancelContext, http.MethodPut, baseURL+path, reader)
	cancelRequest.Header = pdfHTTPHeaders(make([]byte, 1024), updatedETag)
	go func() {
		_, _ = writer.Write([]byte("%PDF-"))
		<-cancelContext.Done()
		_ = writer.Close()
	}()
	_, _ = client.Do(cancelRequest)
	assertPDFHTTPBytes(t, client, baseURL, fixture.AvailableID, atLimit)
}

// TestPDFUploadDeadlineWrapper verifies upload-specific deadlines are set before
// the buffering response wrapper.
func TestPDFUploadDeadlineWrapper(t *testing.T) {
	fixture := serverhelpers.NewPDFViewerFixture(t)
	writer := &deadlineResponseWriter{header: make(http.Header)}
	request := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/runs/%d/articles/%d/pdf", fixture.RunID, fixture.RevisionID), bytes.NewReader(nil))
	fixture.Server.Handler().ServeHTTP(writer, request)
	if writer.read.IsZero() || writer.write.IsZero() || writer.write.Sub(writer.read) <= 0 {
		t.Fatalf("upload deadlines = read %v write %v", writer.read, writer.write)
	}
}

// deadlineResponseWriter records the deadlines set by the upload middleware.
type deadlineResponseWriter struct {
	header http.Header
	read   time.Time
	write  time.Time
	status int
	body   bytes.Buffer
}

// Header returns the response header map.
func (w *deadlineResponseWriter) Header() http.Header { return w.header }

// WriteHeader records the response status.
func (w *deadlineResponseWriter) WriteHeader(status int) { w.status = status }

// Write records a response body.
func (w *deadlineResponseWriter) Write(data []byte) (int, error) { return w.body.Write(data) }

// SetReadDeadline records the request read deadline.
func (w *deadlineResponseWriter) SetReadDeadline(deadline time.Time) error {
	w.read = deadline
	return nil
}

// SetWriteDeadline records the response write deadline.
func (w *deadlineResponseWriter) SetWriteDeadline(deadline time.Time) error {
	w.write = deadline
	return nil
}

// waitForPDFCapacity polls an invalid-signature upload until admission is full.
func waitForPDFCapacity(t *testing.T, client *http.Client, baseURL, path, etag string) {
	t.Helper()
	data := make([]byte, 128)
	copy(data, "not-a-pdf")
	deadline := time.Now().Add(3 * time.Second)
	for {
		request, _ := http.NewRequest(http.MethodPut, baseURL+path, bytes.NewReader(data))
		request.Header = pdfHTTPHeaders(data, etag)
		code, body, _ := doPDFHTTP(t, client, request)
		if code == http.StatusServiceUnavailable && strings.Contains(string(body), "pdf_capacity_exhausted") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("capacity was not exhausted: %d %s", code, body)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitForPDFCapacityRelease polls a no-op upload until capacity is released.
func waitForPDFCapacityRelease(t *testing.T, client *http.Client, baseURL, path, etag string, data []byte) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		request, _ := http.NewRequest(http.MethodPut, baseURL+path, bytes.NewReader(data))
		request.Header = pdfHTTPHeaders(data, etag)
		code, body, _ := doPDFHTTP(t, client, request)
		if code == http.StatusOK {
			return
		}
		if code != http.StatusServiceUnavailable {
			t.Fatalf("capacity release probe = %d %s", code, body)
		}
		if time.Now().After(deadline) {
			t.Fatalf("capacity was not released: %d %s", code, body)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// assertPDFHTTPBytes verifies one evidence read still returns exact old bytes.
func assertPDFHTTPBytes(t *testing.T, client *http.Client, baseURL string, workID int64, want []byte) {
	t.Helper()
	request, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/pdf/%d", baseURL, workID), nil)
	code, body, _ := doPDFHTTP(t, client, request)
	if code != http.StatusOK || !bytes.Equal(body, want) {
		t.Fatalf("PDF read = %d bytes=%d want=%d", code, len(body), len(want))
	}
}
