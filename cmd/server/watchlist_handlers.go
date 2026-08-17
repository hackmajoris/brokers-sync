package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"brokers-sync/internal/watchlist"
)

// maxWatchlistBody caps request bodies; entries are a symbol, a short note and a
// price, so anything larger is abuse.
const maxWatchlistBody = 8 << 10

type watchlistHandler struct {
	store *watchlist.Store
}

// newWatchlistHandler builds the handler from the AWS default config. It returns
// nil when table is empty so local runs work without AWS credentials.
func newWatchlistHandler(ctx context.Context, table string) *watchlistHandler {
	if table == "" {
		return nil
	}
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Printf("watchlist disabled: %v", err)
		return nil
	}
	return &watchlistHandler{store: watchlist.NewStore(dynamodb.NewFromConfig(cfg), table)}
}

// register wires the watchlist routes into mux.
func (h *watchlistHandler) register(mux *http.ServeMux) {
	mux.HandleFunc("/api/watchlist/new", h.handleNew)
	mux.HandleFunc("/api/watchlist", h.handle)
}

// handleNew generates a portfolio code. This is the only watchlist route that
// does not require a code.
func (h *watchlistHandler) handleNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	code, err := watchlist.NewCode()
	if err != nil {
		http.Error(w, "could not generate code", http.StatusInternalServerError)
		return
	}
	normalized, ok := watchlist.Normalize(code)
	if !ok {
		http.Error(w, "could not generate code", http.StatusInternalServerError)
		return
	}
	if err := h.store.EnsureMeta(r.Context(), watchlist.HashKey(normalized)); err != nil {
		http.Error(w, "could not create portfolio", http.StatusBadGateway)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code})
}

func (h *watchlistHandler) handle(w http.ResponseWriter, r *http.Request) {
	pk, ok := portfolioKey(r)
	if !ok {
		notFound(w)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.list(w, r, pk)
	case http.MethodPut:
		h.upsert(w, r, pk)
	case http.MethodDelete:
		h.delete(w, r, pk)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *watchlistHandler) list(w http.ResponseWriter, r *http.Request, pk string) {
	items, err := h.store.List(r.Context(), pk)
	if err != nil {
		h.fail(w, err)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}

func (h *watchlistHandler) upsert(w http.ResponseWriter, r *http.Request, pk string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxWatchlistBody)
	var item watchlist.Item
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := h.store.Upsert(r.Context(), pk, item); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *watchlistHandler) delete(w http.ResponseWriter, r *http.Request, pk string) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		http.Error(w, "missing symbol", http.StatusBadRequest)
		return
	}
	if err := h.store.Delete(r.Context(), pk, symbol); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// fail maps store errors to responses. An unknown portfolio must look exactly
// like a missing or malformed code.
func (h *watchlistHandler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, watchlist.ErrNotFound):
		notFound(w)
	case errors.Is(err, watchlist.ErrInvalid), errors.Is(err, watchlist.ErrTooMany):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, "watchlist unavailable", http.StatusBadGateway)
	}
}

// portfolioKey reads the code from the request header. The code never travels
// in a URL, where it would leak into access logs, history and Referer headers.
func portfolioKey(r *http.Request) (string, bool) {
	normalized, ok := watchlist.Normalize(r.Header.Get("X-Portfolio-Code"))
	if !ok {
		return "", false
	}
	return watchlist.HashKey(normalized), true
}

// notFound is the single response for a missing, malformed or unknown code.
// Distinguishing them would confirm which codes exist.
func notFound(w http.ResponseWriter) {
	http.Error(w, "not found", http.StatusNotFound)
}
