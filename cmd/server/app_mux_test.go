package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// The watchlist app is deployed as its own public Lambda. Broker-file parsing
// and the portfolio JSON belong to brokers-sync only; exposing them there would
// widen the attack surface of an app that has no use for them.
func TestWatchlistAppOmitsPortfolioRoutes(t *testing.T) {
	dir := t.TempDir()
	mux := newMux(appWatchlist, dir, dir, nil)

	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/upload/zip"},
		{http.MethodGet, "/data/result.json"},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: status = %d, want 404", tc.method, tc.path, rec.Code)
		}
	}
}

// The watchlist UI looks symbols up through the market-data routes, so both
// apps must serve them or the standalone watchlist cannot add or show tickers.
func TestBothAppsServeMarketDataRoutes(t *testing.T) {
	dir := t.TempDir()
	for _, app := range []string{appBrokersSync, appWatchlist} {
		mux := newMux(app, dir, dir, nil)
		for _, path := range []string{"/api/ticker/", "/api/search", "/api/history/", "/api/news/"} {
			rec := httptest.NewRecorder()
			// POST is rejected by every market-data handler before any upstream
			// call, which proves the route exists without hitting Yahoo.
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: status = %d, want 405 (route missing?)", app, path, rec.Code)
			}
		}
	}
}

// Each app falls back to its own HTML entry for client routes. Serving the
// portfolio shell from the watchlist app (or the reverse) would boot the
// wrong UI on a refresh.
func TestEachAppFallsBackToItsOwnEntry(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{"index.html": "PORTFOLIO", "watchlist.html": "WATCHLIST"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for app, want := range map[string]string{appBrokersSync: "PORTFOLIO", appWatchlist: "WATCHLIST"} {
		rec := httptest.NewRecorder()
		newMux(app, dir, dir, nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/some/client/route", nil))
		if got := rec.Body.String(); got != want {
			t.Errorf("%s: body = %q, want %q", app, got, want)
		}
	}
}
