package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"brokers-sync/internal/ledger"
	"brokers-sync/internal/parser"
	"brokers-sync/internal/prices"
	"brokers-sync/internal/stats"
)

func main() {
	f, err := os.Open("data/Archive/activit-tradeville.csv")
	if err != nil {
		panic(err)
	}
	defer func() { _ = f.Close() }()

	txs, err := parser.ParseTradeville(f)
	if err != nil {
		panic(err)
	}

	l := ledger.New()
	l.Process(txs)

	now := time.Now()
	ytdStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	fmt.Printf("now:      %s (%s)\n", now.Format(time.RFC3339), now.Location())
	fmt.Printf("ytdStart: %s\n\n", ytdStart.Format(time.RFC3339))

	fmt.Println("=== SNP lots (open, date vs ytdStart) ===")
	if pos, ok := l.Positions["SNP"]; ok {
		for _, lot := range pos.Lots {
			if lot.Quantity > 1e-9 {
				inYTD := !lot.Date.Before(ytdStart)
				fmt.Printf("  date=%s  qty=%.0f  cost=%.2f  inYTD=%v\n",
					lot.Date.Format("2006-01-02T15:04:05Z07:00"), lot.Quantity, lot.CostBasis, inYTD)
			}
		}
	}

	s := stats.Compute(l, txs, now, nil, "RON")
	fmt.Printf("\nYTD.Deposits:  %.2f\n", s.YTD.Deposits)
	fmt.Printf("YTD.Dividends: %.2f\n", s.YTD.Dividends)
	fmt.Printf("YTD.GainPct (before prices): %.4f%%\n", s.YTD.GainPct)

	// Use the real prices from Yahoo (same as the CLI does)
	yahooTickers, reverseMap := buildYahooTickerMap(s.OpenPositions)
	fmt.Printf("\nYahoo tickers to fetch: %v\n", yahooTickers)

	priceMap, err := prices.FetchQuotes(context.TODO(), yahooTickers)
	if err != nil {
		fmt.Printf("price fetch error: %v — using hardcoded fallback\n", err)
		priceMap = map[string]float64{"SNP": 1.002, "TLV": 37.3, "TVBETETF": 48.1}
	} else {
		for yahoo, orig := range reverseMap {
			if p, ok := priceMap[yahoo]; ok {
				priceMap[orig] = p
			}
		}
		for sym, p := range priceMap {
			fmt.Printf("  %s = %.4f\n", sym, p)
		}
	}

	stats.EnrichWithPrices(&s, priceMap, nil)
	fmt.Println("\n=== Positions after enrichment ===")
	for _, p := range s.OpenPositions {
		fmt.Printf("  %-12s cost=%.2f  price=%.4f  unrealized=%.2f\n",
			p.Symbol, p.TotalCost, p.CurrentPrice, p.UnrealizedPnL)
	}

	stats.RecalcGainPct(&s)
	fmt.Printf("\nYTD.GainPct (after prices): %.4f%%\n", s.YTD.GainPct)
}

func buildYahooTickerMap(positions []stats.PositionSummary) ([]string, map[string]string) {
	reverseMap := make(map[string]string)
	var tickers []string
	for _, p := range positions {
		yahoo := prices.YahooTicker(p.Symbol, p.Currency)
		tickers = append(tickers, yahoo)
		if yahoo != p.Symbol {
			reverseMap[yahoo] = p.Symbol
		}
	}
	return tickers, reverseMap
}
