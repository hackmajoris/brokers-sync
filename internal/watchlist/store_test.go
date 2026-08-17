package watchlist

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const testPK = "P#test"

func newStore() (*Store, *FakeAPI) {
	db := NewFakeAPI()
	return NewStore(db, "test-table"), db
}

// A repeated create must not reset an existing portfolio: a user who re-enters
// their code would otherwise silently lose their watchlist.
func TestEnsureMetaIsIdempotent(t *testing.T) {
	s, db := newStore()
	ctx := context.Background()

	if err := s.EnsureMeta(ctx, testPK); err != nil {
		t.Fatalf("first EnsureMeta: %v", err)
	}
	created := db.Num(testPK, "META", "createdAt")

	if err := s.EnsureMeta(ctx, testPK); err != nil {
		t.Fatalf("second EnsureMeta: %v", err)
	}
	if !db.CondFailed {
		t.Error("second EnsureMeta did not hit the attribute_not_exists guard")
	}
	if got := db.Num(testPK, "META", "createdAt"); got != created {
		t.Errorf("createdAt changed: %q -> %q", created, got)
	}
}

// Unknown portfolios must be reported as not found so handlers can return an
// indistinguishable 404 and avoid confirming which codes exist.
func TestListUnknownPortfolio(t *testing.T) {
	s, _ := newStore()
	if _, err := s.List(context.Background(), "P#nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("List on unknown portfolio: got %v, want ErrNotFound", err)
	}
}

func TestUpsertAndList(t *testing.T) {
	s, _ := newStore()
	ctx := context.Background()
	if err := s.EnsureMeta(ctx, testPK); err != nil {
		t.Fatal(err)
	}

	if err := s.Upsert(ctx, testPK, Item{Symbol: "aapl", Note: "watch earnings"}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	items, err := s.List(ctx, testPK)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Symbol != "AAPL" {
		t.Errorf("symbol = %q, want AAPL (input must be normalized)", items[0].Symbol)
	}
	if items[0].AddedAt == 0 {
		t.Error("AddedAt not set")
	}
}

// Each cap exists to stop the table being used as free key-value storage.
func TestUpsertRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		item Item
		want error
	}{
		{"empty symbol", Item{Symbol: ""}, ErrInvalid},
		{"symbol too long", Item{Symbol: "ABCDEFGHIJKLM"}, ErrInvalid},
		{"symbol with illegal char", Item{Symbol: "AA PL"}, ErrInvalid},
		{"note too long", Item{Symbol: "AAPL", Note: strings.Repeat("x", MaxNoteLen+1)}, ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newStore()
			ctx := context.Background()
			if err := s.EnsureMeta(ctx, testPK); err != nil {
				t.Fatal(err)
			}
			if err := s.Upsert(ctx, testPK, tt.item); !errors.Is(err, tt.want) {
				t.Fatalf("Upsert: got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestUpsertEnforcesSymbolCap(t *testing.T) {
	s, _ := newStore()
	ctx := context.Background()
	if err := s.EnsureMeta(ctx, testPK); err != nil {
		t.Fatal(err)
	}
	for i := range MaxSymbols {
		sym := fmt.Sprintf("S%d", i)
		if err := s.Upsert(ctx, testPK, Item{Symbol: sym}); err != nil {
			t.Fatalf("Upsert %s: %v", sym, err)
		}
	}
	if err := s.Upsert(ctx, testPK, Item{Symbol: "OVER"}); !errors.Is(err, ErrTooMany) {
		t.Fatalf("symbol %d: got %v, want ErrTooMany", MaxSymbols+1, err)
	}
	// Updating an already-tracked symbol is not a new symbol, so the cap must
	// not block edits once a watchlist is full.
	if err := s.Upsert(ctx, testPK, Item{Symbol: "S0", Note: "edited"}); err != nil {
		t.Fatalf("updating existing symbol at cap: %v", err)
	}
}

// An actively used portfolio must never expire out from under the user.
func TestWritesRefreshTTL(t *testing.T) {
	s, db := newStore()
	ctx := context.Background()
	if err := s.EnsureMeta(ctx, testPK); err != nil {
		t.Fatal(err)
	}

	stale := strconv.FormatInt(time.Now().Add(-100*time.Hour).Unix(), 10)
	meta := db.Items[testPK]["META"].(*types.AttributeValueMemberM)
	meta.Value["ttl"] = &types.AttributeValueMemberN{Value: stale}

	if err := s.Upsert(ctx, testPK, Item{Symbol: "AAPL"}); err != nil {
		t.Fatal(err)
	}
	if got := db.Num(testPK, "META", "ttl"); got == stale {
		t.Error("Upsert did not refresh ttl")
	}

	meta.Value["ttl"] = &types.AttributeValueMemberN{Value: stale}
	if err := s.Delete(ctx, testPK, "AAPL"); err != nil {
		t.Fatal(err)
	}
	if got := db.Num(testPK, "META", "ttl"); got == stale {
		t.Error("Delete did not refresh ttl")
	}
}

func TestDelete(t *testing.T) {
	s, _ := newStore()
	ctx := context.Background()
	if err := s.EnsureMeta(ctx, testPK); err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(ctx, testPK, Item{Symbol: "AAPL"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, testPK, "aapl"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	items, err := s.List(ctx, testPK)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items after delete, want 0", len(items))
	}
}
