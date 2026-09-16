package prices

import (
	"testing"
	"time"
)

// Only headlines published on the caller's current UTC calendar date belong in
// "today's news" — a stale or future-dated item slipping through would show
// old/wrong headlines in the ticker dialog.
func TestFilterTodayNewsKeepsOnlyToday(t *testing.T) {
	now := time.Date(2026, 9, 16, 14, 0, 0, 0, time.UTC)
	today := time.Date(2026, 9, 16, 3, 0, 0, 0, time.UTC).Unix()
	yesterday := time.Date(2026, 9, 15, 23, 59, 0, 0, time.UTC).Unix()
	tomorrow := time.Date(2026, 9, 17, 0, 1, 0, 0, time.UTC).Unix()

	items := []rawNewsItem{
		{Title: "Today's headline", Link: "https://finance.yahoo.com/today", ProviderPublishTime: today},
		{Title: "Old headline", Link: "https://finance.yahoo.com/old", ProviderPublishTime: yesterday},
		{Title: "Future headline", Link: "https://finance.yahoo.com/future", ProviderPublishTime: tomorrow},
	}

	got := filterTodayNews(items, now)
	if len(got) != 1 || got[0].Title != "Today's headline" {
		t.Fatalf("want only today's headline, got %+v", got)
	}
}

// Yahoo's feed occasionally returns entries missing a title or link; rendering
// those would produce a blank or dead row in the news list.
func TestFilterTodayNewsSkipsIncompleteItems(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	today := now.Unix()

	items := []rawNewsItem{
		{Title: "", Link: "https://finance.yahoo.com/no-title", ProviderPublishTime: today},
		{Title: "No link", Link: "", ProviderPublishTime: today},
		{Title: "Complete", Link: "https://finance.yahoo.com/complete", ProviderPublishTime: today},
	}

	got := filterTodayNews(items, now)
	if len(got) != 1 || got[0].Title != "Complete" {
		t.Fatalf("want only the complete item, got %+v", got)
	}
}

// No matches must return an empty slice, not nil — the JSON encoder renders
// nil as `null`, which the frontend would have to special-case instead of
// treating an empty news day the same as any other empty list.
func TestFilterTodayNewsReturnsEmptySliceNotNil(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	got := filterTodayNews(nil, now)
	if got == nil {
		t.Fatal("want empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("want no items, got %+v", got)
	}
}
