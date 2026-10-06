package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func originTestHandler(secret string) http.Handler {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return requireOriginHeader(ok, secret)
}

// Direct calls to the execute-api URL skip the CloudFront WAF and share the
// stage throttle with real users, so anything without CloudFront's secret must
// be refused before it reaches a handler.
func TestRequireOriginHeaderRejectsDirectCalls(t *testing.T) {
	h := originTestHandler("s3cret")
	for name, header := range map[string]string{"missing": "", "wrong": "guess", "prefix": "s3cre"} {
		req := httptest.NewRequest(http.MethodPost, "/api/upload/zip", nil)
		if header != "" {
			req.Header.Set("X-Origin-Verify", header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s header: status = %d, want 403", name, rec.Code)
		}
	}
}

func TestRequireOriginHeaderAllowsCloudFront(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=aapl", nil)
	req.Header.Set("X-Origin-Verify", "s3cret")
	rec := httptest.NewRecorder()
	originTestHandler("s3cret").ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// The Lambda Web Adapter readiness probe calls GET / without the header. If it
// got 403 the function could fail to become ready, taking the whole API down.
func TestRequireOriginHeaderAllowsReadinessProbe(t *testing.T) {
	rec := httptest.NewRecorder()
	originTestHandler("s3cret").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("GET /: status = %d, want 200", rec.Code)
	}
}

// Local runs have no CloudFront in front, so no secret means no check.
func TestRequireOriginHeaderDisabledWithoutSecret(t *testing.T) {
	rec := httptest.NewRecorder()
	originTestHandler("").ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/upload/zip", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
