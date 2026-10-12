package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"brokers-sync/internal/watchlist"
)

// fakeHuman stands in for Turnstile. A zero value accepts every token.
type fakeHuman struct{ err error }

func (f fakeHuman) Verify(context.Context, string) error { return f.err }

func createWith(human humanVerifier) (*httptest.ResponseRecorder, *watchlist.FakeAPI) {
	db := watchlist.NewFakeAPI()
	h := &watchlistHandler{store: watchlist.NewStore(db, "test-table"), maxCodes: defaultMaxCodes, metrics: io.Discard, human: human}
	mux := http.NewServeMux()
	h.registerCreate(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/watchlist/new", nil))
	return rec, db
}

// The bot check exists to protect the code cap. A rejected or unverifiable
// request must create nothing and must not consume a slot, or bots could still
// drain the cap just by failing the check.
func TestBotCheckGuardsCreation(t *testing.T) {
	tests := []struct {
		name   string
		human  humanVerifier
		status int
	}{
		{"rejected token", fakeHuman{err: errNotHuman}, http.StatusForbidden},
		{"Cloudflare unreachable fails closed", fakeHuman{err: errors.New("dial tcp: timeout")}, http.StatusBadGateway},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, db := createWith(tt.human)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if len(db.Items) != 0 {
				t.Errorf("table has %d items after a failed check, want 0 (no slot, no portfolio)", len(db.Items))
			}
		})
	}

	rec, db := createWith(fakeHuman{})
	if rec.Code != http.StatusOK || len(db.Items) != 2 { // counter + portfolio
		t.Errorf("passing check: status %d, items %d; want 200 and a counter plus a portfolio", rec.Code, len(db.Items))
	}
}

// Creation without a bot check must be impossible to wire up by accident.
func TestRegisterCreateRequiresBotCheck(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("registerCreate without a verifier did not panic")
		}
	}()
	h := &watchlistHandler{store: watchlist.NewStore(watchlist.NewFakeAPI(), "t"), maxCodes: 1}
	h.registerCreate(http.NewServeMux())
}

// The verifier must only accept what Cloudflare confirmed for this site.
func TestTurnstileVerifier(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		token   string
		wantErr error // nil, errNotHuman, or errAny for "some other error"
	}{
		{"accepted for our host", 200, `{"success":true,"hostname":"watchlist.dot-core.com"}`, "tok", nil},
		{"token for another site is replayed", 200, `{"success":true,"hostname":"evil.example"}`, "tok", errNotHuman},
		{"rejected by Cloudflare", 200, `{"success":false,"error-codes":["invalid-input-response"]}`, "tok", errNotHuman},
		{"missing token never reaches Cloudflare", 200, `{"success":true,"hostname":"watchlist.dot-core.com"}`, "", errNotHuman},
		{"Cloudflare error is not a rejection", 500, `oops`, "tok", errAny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if err := r.ParseForm(); err != nil || r.PostForm.Get("secret") != "s3cret" || r.PostForm.Get("response") != tt.token {
					t.Errorf("verify request form = %v, want secret and token", r.PostForm)
				}
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer srv.Close()

			v := newTurnstileVerifier("s3cret", "watchlist.dot-core.com")
			v.verifyURL = srv.URL
			err := v.Verify(context.Background(), tt.token)

			switch {
			case tt.wantErr == nil && err != nil:
				t.Fatalf("got %v, want success", err)
			case tt.wantErr == errNotHuman && !errors.Is(err, errNotHuman):
				t.Fatalf("got %v, want errNotHuman", err)
			case tt.wantErr == errAny && (err == nil || errors.Is(err, errNotHuman)):
				t.Fatalf("got %v, want a non-rejection error so the handler fails closed with 502", err)
			}
			if tt.token == "" && calls != 0 {
				t.Errorf("empty token made %d calls to Cloudflare, want 0", calls)
			}
		})
	}
}

var errAny = errors.New("any non-rejection error")
