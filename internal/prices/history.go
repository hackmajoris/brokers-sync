package prices

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// MAPoint is a moving-average value at a point in time.
type MAPoint struct {
	Time  int64
	Value float64
}

// SimpleMA returns the period-length simple moving average of candle closes.
// The first value is emitted once `period` closes are available.
func SimpleMA(candles []Candle, period int) []MAPoint {
	if period <= 0 || len(candles) < period {
		return nil
	}
	out := make([]MAPoint, 0, len(candles)-period+1)
	var sum float64
	for i, c := range candles {
		sum += c.Close
		if i >= period {
			sum -= candles[i-period].Close
		}
		if i >= period-1 {
			out = append(out, MAPoint{Time: c.Time, Value: sum / float64(period)})
		}
	}
	return out
}

// Candle is a single OHLC bar.
type Candle struct {
	Time   int64   `json:"t"` // unix seconds
	Open   float64 `json:"o"`
	High   float64 `json:"h"`
	Low    float64 `json:"l"`
	Close  float64 `json:"c"`
	Volume int64   `json:"v"`
}

// FetchHistory fetches OHLC candles for a symbol from Yahoo's v8 chart endpoint.
// rng is a Yahoo range (e.g. "1mo", "6mo", "1y", "5y"); interval is a Yahoo
// interval (e.g. "1d", "1wk", "1mo"). Bars with missing OHLC data are skipped.
func FetchHistory(ctx context.Context, symbol, rng, interval string) ([]Candle, error) {
	u := "https://query1.finance.yahoo.com/v8/finance/chart/" + url.PathEscape(symbol) + "?" + url.Values{
		"range":    {rng},
		"interval": {interval},
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
		return nil, fmt.Errorf("yahoo chart returned %d", res.StatusCode)
	}

	var payload struct {
		Chart struct {
			Result []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Open   []*float64 `json:"open"`
						High   []*float64 `json:"high"`
						Low    []*float64 `json:"low"`
						Close  []*float64 `json:"close"`
						Volume []*float64 `json:"volume"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if len(payload.Chart.Result) == 0 || len(payload.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, fmt.Errorf("no history for %q", symbol)
	}

	r := payload.Chart.Result[0]
	q := r.Indicators.Quote[0]
	out := make([]Candle, 0, len(r.Timestamp))
	for i, ts := range r.Timestamp {
		if i >= len(q.Open) || i >= len(q.High) || i >= len(q.Low) || i >= len(q.Close) {
			break
		}
		o, h, l, c := q.Open[i], q.High[i], q.Low[i], q.Close[i]
		if o == nil || h == nil || l == nil || c == nil {
			continue
		}
		var vol int64
		if i < len(q.Volume) && q.Volume[i] != nil {
			vol = int64(*q.Volume[i])
		}
		out = append(out, Candle{Time: ts, Open: *o, High: *h, Low: *l, Close: *c, Volume: vol})
	}
	return out, nil
}
