package prices

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// SearchResult is a single symbol match from Yahoo's search/autocomplete API.
type SearchResult struct {
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Exchange string `json:"exchange"`
	Type     string `json:"type"`
}

// SearchSymbols queries Yahoo Finance's search endpoint for symbols matching
// query. It needs no auth crumb — only a browser-like User-Agent. Returns at
// most a handful of matches for typeahead.
func SearchSymbols(ctx context.Context, query string) ([]SearchResult, error) {
	u := "https://query1.finance.yahoo.com/v1/finance/search?" + url.Values{
		"q":                {query},
		"quotesCount":      {"8"},
		"newsCount":        {"0"},
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
		return nil, fmt.Errorf("yahoo search returned %d", res.StatusCode)
	}

	var payload struct {
		Quotes []struct {
			Symbol    string `json:"symbol"`
			ShortName string `json:"shortname"`
			LongName  string `json:"longname"`
			Exchange  string `json:"exchange"`
			QuoteType string `json:"quoteType"`
		} `json:"quotes"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}

	out := make([]SearchResult, 0, len(payload.Quotes))
	for _, q := range payload.Quotes {
		if q.Symbol == "" {
			continue
		}
		name := q.LongName
		if name == "" {
			name = q.ShortName
		}
		out = append(out, SearchResult{
			Symbol:   q.Symbol,
			Name:     name,
			Exchange: q.Exchange,
			Type:     q.QuoteType,
		})
	}
	return out, nil
}
