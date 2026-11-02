package handler

import (
	"net/http"

	"example.com/pdfreplacement/store"
)

// Handler serves PDF replacement requests.
type Handler struct {
	store *store.Store
}

// New creates a Handler for the given store.
func New(s *store.Store) *Handler {
	return &Handler{store: s}
}

func saveArticlePDF(h *Handler, w http.ResponseWriter, _ *http.Request) {
	_ = h.store.Save("pdf-id")
	w.WriteHeader(http.StatusOK)
}

// NewServer maps HTTP routes to handler functions.
func NewServer() *http.ServeMux {
	mux := http.NewServeMux()
	h := New(&store.Store{})
	mux.HandleFunc("PUT /upload", func(w http.ResponseWriter, r *http.Request) {
		saveArticlePDF(h, w, r)
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}
