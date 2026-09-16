package prices

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// NewsItem is a single headline from Yahoo Finance's search/news feed.
type NewsItem struct {
	Title string `json:"title"`
	Link  string `json:"link"`
}

// FetchNews returns today's Yahoo Finance headlines for symbol. It reuses the
// same unauthenticated search endpoint as SearchSymbols, requesting news
// results instead of quotes.
func FetchNews(ctx context.Context, symbol string) ([]NewsItem, error) {
	u := "https://query1.finance.yahoo.com/v1/finance/search?" + url.Values{
		"q":                {symbol},
		"quotesCount":      {"0"},
		"newsCount":        {"10"},
		"listsCount":       {"0"},
		"enableFuzzyQuery": {"false"},
	}.Encode()

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo news returned %d", res.StatusCode)
	}

	var payload struct {
		News []rawNewsItem `json:"news"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return filterTodayNews(payload.News, time.Now()), nil
}

type rawNewsItem struct {
	Title               string `json:"title"`
	Link                string `json:"link"`
	ProviderPublishTime int64  `json:"providerPublishTime"`
}

// filterTodayNews keeps only headlines published on now's UTC calendar date,
// discarding anything with a blank title or link. Separated from FetchNews so
// the date-boundary logic can be tested without a live HTTP call.
func filterTodayNews(items []rawNewsItem, now time.Time) []NewsItem {
	todayY, todayM, todayD := now.UTC().Date()
	out := make([]NewsItem, 0, len(items))
	for _, n := range items {
		if n.Title == "" || n.Link == "" {
			continue
		}
		y, m, d := time.Unix(n.ProviderPublishTime, 0).UTC().Date()
		if y != todayY || m != todayM || d != todayD {
			continue
		}
		out = append(out, NewsItem{Title: n.Title, Link: n.Link})
	}
	return out
}
