package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Client-side routes such as /watchlist have no file behind them. Without a
// fallback the server 404s and a refresh or a pasted link breaks, which is what
// CloudFront's 404-to-index.html mapping already prevents in production.
func TestSPAHandlerServesClientRoutes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("INDEX"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("BUNDLE"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := spaHandler(dir)
	tests := []struct {
		name string
		path string
		want string
	}{
		{"client route", "/watchlist", "INDEX"},
		{"another client route", "/positions", "INDEX"},
		{"root", "/", "INDEX"},
		{"real asset is not swallowed by the fallback", "/assets/app.js", "BUNDLE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Body.String(); got != tt.want {
				t.Errorf("body = %q, want %q", got, tt.want)
			}
		})
	}
}
