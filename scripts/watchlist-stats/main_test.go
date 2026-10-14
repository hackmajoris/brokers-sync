package main

import (
	"testing"
	"time"
)

// The report exists to show which lists are actively used, so the order must
// follow the latest addition, and the per-list counts must only include real
// stock rows: not the META record, not the cap counter, not other key spaces.
func TestAggregate(t *testing.T) {
	day := func(d int) int64 { return time.Date(2026, 10, d, 12, 0, 0, 0, time.UTC).Unix() }
	rows := []row{
		{PK: "SYSTEM#codes", SK: "COUNTER", Created: 7},
		{PK: "P#old", SK: "META", CreatedAt: day(1), LastSeen: day(2)},
		{PK: "P#old", SK: "W#KO", AddedAt: day(2)},
		{PK: "P#busy", SK: "META", CreatedAt: day(3), LastSeen: day(10), LastAccess: day(11)},
		{PK: "P#busy", SK: "W#NVDA", AddedAt: day(4)},
		{PK: "P#busy", SK: "W#KO", AddedAt: day(10)},
		{PK: "P#busy", SK: "W#ASML", AddedAt: day(6)},
		{PK: "P#empty", SK: "META", CreatedAt: day(5), LastSeen: day(5)},
		{PK: "OTHER#x", SK: "W#AAPL", AddedAt: day(11)}, // not a portfolio
	}

	s := aggregate(rows)

	order := []string{}
	for _, a := range s.Accounts {
		order = append(order, a.Key)
	}
	want := []string{"P#busy", "P#old", "P#empty"}
	if len(order) != len(want) {
		t.Fatalf("accounts = %v, want %v (non-portfolio keys excluded)", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v (latest addition first, empty last)", order, want)
		}
	}

	busy := s.Accounts[0]
	if busy.Stocks != 3 || busy.LastSymbol != "KO" || !busy.LastAdded.Equal(time.Unix(day(10), 0)) {
		t.Errorf("busy = %d stocks, last %s at %v; want 3, KO at day 10", busy.Stocks, busy.LastSymbol, busy.LastAdded)
	}
	if !busy.LastAccess.Equal(time.Unix(day(11), 0)) || !s.Accounts[1].LastAccess.IsZero() {
		t.Errorf("lastAccess = %v / %v; want day 11 for busy and none for a list never opened since tracking began", busy.LastAccess, s.Accounts[1].LastAccess)
	}
	if s.TotalStocks != 4 || s.Empty != 1 || s.CodesCreated != 7 {
		t.Errorf("totals = %d stocks, %d empty, %d codes; want 4, 1, 7", s.TotalStocks, s.Empty, s.CodesCreated)
	}
	if s.Symbols["KO"] != 2 || s.Symbols["AAPL"] != 0 {
		t.Errorf("symbol counts = %v; want KO 2 and no AAPL from a non-portfolio key", s.Symbols)
	}
}

// A millisecond value must not sort as a date tens of thousands of years out.
func TestUnixAcceptsMilliseconds(t *testing.T) {
	sec := int64(1_760_000_000)
	if !unix(sec).Equal(unix(sec * 1000)) {
		t.Errorf("unix(%d) = %v, unix(ms) = %v; want equal", sec, unix(sec), unix(sec*1000))
	}
}
