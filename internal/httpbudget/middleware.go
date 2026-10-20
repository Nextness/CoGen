// Package httpbudget bounds request SQL work and buffered response sizes without owning application error envelopes.
package httpbudget

import (
	"analysis/internal/querybudget"
	"bytes"
	"net/http"
	"strconv"
)

// Limits defines one route's serialized response and attempted-statement ceilings; zero disables that ceiling.
type Limits struct{ Bytes, Queries int }

// responseBudgetWriter buffers one bounded JSON response before it is committed.
type responseBudgetWriter struct {
	header   http.Header
	body     bytes.Buffer
	status   int
	limit    int
	exceeded bool
}

// Header implements http.ResponseWriter.
func (w *responseBudgetWriter) Header() http.Header { return w.header }

// WriteHeader records the first response status without committing it.
func (w *responseBudgetWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

// Write retains at most the configured byte budget and reports a complete write.
func (w *responseBudgetWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	remaining := w.limit - w.body.Len()
	if remaining < len(data) {
		w.exceeded = true
		if remaining > 0 {
			_, _ = w.body.Write(data[:remaining])
		}
		return len(data), nil
	}
	_, _ = w.body.Write(data)
	return len(data), nil
}

// Middleware bounds serialized responses and SQL work according to the supplied route policy.
func Middleware(next http.Handler, policy func(string) Limits, fail func(http.ResponseWriter, int, string, string, any)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limits := policy(r.URL.Path)
		limit, queryLimit := limits.Bytes, limits.Queries
		if limit == 0 && queryLimit == 0 {
			next.ServeHTTP(w, r)
			return
		}
		ctx, queryState := querybudget.With(r.Context(), queryLimit)
		r = r.WithContext(ctx)
		if limit == 0 {
			w.Header().Set("X-Query-Count-Limit", strconv.Itoa(queryLimit))
			w.Header().Add("Trailer", "X-Query-Count-Used")
			next.ServeHTTP(w, r)
			w.Header().Set("X-Query-Count-Used", strconv.FormatInt(queryState.Used(), 10))
			return
		}
		buffered := &responseBudgetWriter{header: make(http.Header), limit: limit}
		next.ServeHTTP(buffered, r)
		if limit > 0 {
			w.Header().Set("X-Response-Byte-Limit", strconv.Itoa(limit))
		}
		w.Header().Set("X-Query-Count-Limit", strconv.Itoa(queryLimit))
		w.Header().Set("X-Query-Count-Used", strconv.FormatInt(queryState.Used(), 10))
		if queryState.Exceeded() {
			fail(w, http.StatusInternalServerError, "query_budget_exceeded", "the request exceeded its SQL statement budget", map[string]any{"query_limit": queryLimit})
			return
		}
		if buffered.exceeded {
			fail(w, http.StatusInternalServerError, "response_budget_exceeded", "the bounded response exceeded its serialized byte limit", map[string]any{"byte_limit": limit})
			return
		}
		for key, values := range buffered.header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		status := buffered.status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write(buffered.body.Bytes())
	})
}
