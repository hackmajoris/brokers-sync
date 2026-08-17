package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"brokers-sync/internal/watchlist"
)

func newTestServer() (*httptest.Server, *watchlist.FakeAPI) {
	db := watchlist.NewFakeAPI()
	h := &watchlistHandler{store: watchlist.NewStore(db, "test-table")}
	mux := http.NewServeMux()
	h.register(mux)
	return httptest.NewServer(mux), db
}

// createCode exercises the real creation path so tests use a genuine code.
func createCode(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	resp, err := http.Post(srv.URL+"/api/watchlist/new", "application/json", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Code == "" {
		t.Fatal("empty code returned")
	}
	return body.Code
}

func do(t *testing.T, srv *httptest.Server, method, path, code, body string) *http.Response {
	t.Helper()
	var r *http.Request
	var err error
	if body == "" {
		r, err = http.NewRequest(method, srv.URL+path, nil)
	} else {
		r, err = http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	}
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if code != "" {
		r.Header.Set("X-Portfolio-Code", code)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	return resp
}

func TestWatchlistLifecycle(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	code := createCode(t, srv)

	resp := do(t, srv, http.MethodPut, "/api/watchlist", code, `{"symbol":"aapl","note":"earnings","targetPrice":150}`)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT status = %d, want 204", resp.StatusCode)
	}

	resp = do(t, srv, http.MethodGet, "/api/watchlist", code, "")
	var listed struct {
		Items []watchlist.Item `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	_ = resp.Body.Close()
	if len(listed.Items) != 1 || listed.Items[0].Symbol != "AAPL" {
		t.Fatalf("list = %+v, want one AAPL item", listed.Items)
	}
	if listed.Items[0].TargetPrice != 150 {
		t.Errorf("targetPrice = %v, want 150", listed.Items[0].TargetPrice)
	}

	resp = do(t, srv, http.MethodDelete, "/api/watchlist?symbol=AAPL", code, "")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", resp.StatusCode)
	}

	resp = do(t, srv, http.MethodGet, "/api/watchlist", code, "")
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	_ = resp.Body.Close()
	if len(listed.Items) != 0 {
		t.Errorf("list after delete = %+v, want empty", listed.Items)
	}
}

// Two portfolios must never see each other's entries. This is the whole
// security model: the code is the only thing separating them.
func TestWatchlistIsolatedPerCode(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	a := createCode(t, srv)
	b := createCode(t, srv)

	_ = do(t, srv, http.MethodPut, "/api/watchlist", a, `{"symbol":"AAPL"}`).Body.Close()

	resp := do(t, srv, http.MethodGet, "/api/watchlist", b, "")
	var listed struct {
		Items []watchlist.Item `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	_ = resp.Body.Close()
	if len(listed.Items) != 0 {
		t.Fatalf("portfolio B sees %+v, want empty", listed.Items)
	}
}

// Missing, malformed and unknown-but-well-formed codes must be byte-identical.
// Any difference tells an attacker which codes exist. This test fails the
// moment someone adds a "helpful" error message to one of these paths.
func TestUnauthorizedResponsesAreIndistinguishable(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	unknown, err := watchlist.NewCode()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		code string
	}{
		{"missing code", ""},
		{"malformed code", "not-a-code"},
		{"wrong length", "K7M2-9QRF-3XVB"},
		{"unknown but well-formed", unknown},
	}

	type response struct {
		status int
		body   string
	}
	var want response
	for i, tc := range cases {
		resp := do(t, srv, http.MethodGet, "/api/watchlist", tc.code, "")
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		got := response{resp.StatusCode, string(raw)}

		if got.status != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", tc.name, got.status)
		}
		if i == 0 {
			want = got
			continue
		}
		if got != want {
			t.Errorf("%s: response %+v differs from %+v", tc.name, got, want)
		}
	}
}

func TestUpsertRejectsOversizedBody(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	code := createCode(t, srv)

	huge := `{"symbol":"AAPL","note":"` + strings.Repeat("x", maxWatchlistBody+1) + `"}`
	resp := do(t, srv, http.MethodPut, "/api/watchlist", code, huge)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUpsertRejectsOverlongNote(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	code := createCode(t, srv)

	body := `{"symbol":"AAPL","note":"` + strings.Repeat("x", watchlist.MaxNoteLen+1) + `"}`
	resp := do(t, srv, http.MethodPut, "/api/watchlist", code, body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	code := createCode(t, srv)

	resp := do(t, srv, http.MethodPatch, "/api/watchlist", code, "")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("PATCH status = %d, want 405", resp.StatusCode)
	}

	resp = do(t, srv, http.MethodGet, "/api/watchlist/new", "", "")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /new status = %d, want 405", resp.StatusCode)
	}
}
