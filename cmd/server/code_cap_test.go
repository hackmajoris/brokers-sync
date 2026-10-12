package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"brokers-sync/internal/watchlist"
)

// Once the cap is used up, creation must fail with a status the client can
// explain to the user, and must not leave a portfolio behind: a refused create
// that still wrote a META item would make the cap meaningless.
func TestCreateRefusedAtCap(t *testing.T) {
	srv, db := newCappedTestServer(2)
	defer srv.Close()

	createCode(t, srv)
	createCode(t, srv)
	portfolios := len(db.Items) - 1 // minus the counter item

	resp, err := http.Post(srv.URL+"/api/watchlist/new", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("create past cap: status = %d, want 503", resp.StatusCode)
	}
	if got := len(db.Items) - 1; got != portfolios {
		t.Errorf("portfolios after refused create = %d, want %d", got, portfolios)
	}
}

// brokers-sync keeps read/write access for its lookup modal but must not mint
// codes: the watchlist app is the single place creation is counted and guarded.
func TestOnlyWatchlistAppCreatesCodes(t *testing.T) {
	for app, want := range map[string]int{appBrokersSync: http.StatusNotFound, appWatchlist: http.StatusOK} {
		wl := &watchlistHandler{store: watchlist.NewStore(watchlist.NewFakeAPI(), "test-table"), maxCodes: defaultMaxCodes, human: fakeHuman{}}
		dir := t.TempDir()
		mux := newMux(app, dir, dir, wl)

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/watchlist/new", nil))
		if rec.Code != want {
			t.Errorf("%s: POST /api/watchlist/new status = %d, want %d", app, rec.Code, want)
		}

		// Both apps still serve existing lists; without a code this is the
		// handler's own 404, so check the route exists via a disallowed method.
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/api/watchlist", nil))
		if rec.Code == http.StatusNotFound && rec.Body.String() == "404 page not found\n" {
			t.Errorf("%s: /api/watchlist route missing", app)
		}
	}
}

// A typo in MAX_CODES must stop the server instead of silently moving the cap.
func TestMaxCodesFromEnv(t *testing.T) {
	if n, err := maxCodesFromEnv(""); err != nil || n != defaultMaxCodes {
		t.Errorf("unset: got %d, %v; want %d", n, err, defaultMaxCodes)
	}
	if n, err := maxCodesFromEnv("250"); err != nil || n != 250 {
		t.Errorf("250: got %d, %v", n, err)
	}
	for _, bad := range []string{"0", "-5", "1k", "abc"} {
		if _, err := maxCodesFromEnv(bad); err == nil {
			t.Errorf("%q: want an error", bad)
		}
	}
}

// The creation alarm counts these lines. A refused create must not emit one,
// and every real create must, or the alarm under- or over-reports an attack.
func TestCreateEmitsOneMetricPerCode(t *testing.T) {
	var buf bytes.Buffer
	h := &watchlistHandler{store: watchlist.NewStore(watchlist.NewFakeAPI(), "test-table"), maxCodes: 2, metrics: &buf, human: fakeHuman{}}
	mux := http.NewServeMux()
	h.registerCreate(mux)

	for range 3 { // the third is refused by the cap
		mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/watchlist/new", nil))
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("metric lines = %d, want 2 (one per created code)", len(lines))
	}
	var emf struct {
		AWS struct {
			CloudWatchMetrics []struct {
				Namespace string
				Metrics   []struct{ Name string }
			}
		} `json:"_aws"`
		App          string
		CodesCreated int
	}
	if err := json.Unmarshal([]byte(lines[0]), &emf); err != nil {
		t.Fatalf("metric line is not JSON: %v", err)
	}
	cw := emf.AWS.CloudWatchMetrics
	if len(cw) != 1 || cw[0].Namespace != metricNamespace || len(cw[0].Metrics) != 1 || cw[0].Metrics[0].Name != metricCodesCreated {
		t.Errorf("EMF metadata = %+v, want namespace %s metric %s", cw, metricNamespace, metricCodesCreated)
	}
	if emf.App != appWatchlist || emf.CodesCreated != 1 {
		t.Errorf("EMF values = App %q CodesCreated %d, want %q 1", emf.App, emf.CodesCreated, appWatchlist)
	}
}
