package prices

import (
	"context"

	"github.com/hackmajoris/go-finance/pkg/yahoo"
)

// FetchQuotes fetches current prices for a list of ticker symbols in parallel.
// Returns a map of both original and normalized symbol → price.
func FetchQuotes(ctx context.Context, symbols []string) (map[string]float64, error) {
	client, err := yahoo.New()
	if err != nil {
		return nil, err
	}
	return client.FetchQuotes(ctx, symbols)
}

// FetchFXRates fetches FX spot rates relative to a base currency (e.g. "USD")
// in parallel. The base currency always gets rate 1.0.
func FetchFXRates(ctx context.Context, currencies []string, base string) (map[string]float64, error) {
	client, err := yahoo.New()
	if err != nil {
		return nil, err
	}
	return client.FetchFXRates(ctx, currencies, base)
}
