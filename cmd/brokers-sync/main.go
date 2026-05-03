package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"brokers-sync/internal/ledger"
	"brokers-sync/internal/model"
	"brokers-sync/internal/output"
	"brokers-sync/internal/parser"
	"brokers-sync/internal/prices"
	"brokers-sync/internal/stats"
)

func main() {
	revolut := flag.String("revolut", "", "Revolut CSV file")
	ibkr := flag.String("ibkr", "", "IBKR CSV file")
	t212 := flag.String("t212", "", "Trading 212 CSV file")
	format := flag.String("format", "text", "Output format: text | json | csv")
	out := flag.String("out", "", "Output path: file for json, directory for csv (stdout if empty)")
	noPrices := flag.Bool("no-prices", false, "Skip live price fetch (no unrealized P&L)")
	flag.Parse()

	var allTxs []model.Transaction

	if *revolut != "" {
		txs, err := parseFile(*revolut, parser.ParseRevolut)
		if err != nil {
			log.Fatalf("revolut: %v", err)
		}
		allTxs = append(allTxs, txs...)
		fmt.Fprintf(os.Stderr, "Revolut:     %d transactions\n", len(txs))
	}
	if *ibkr != "" {
		txs, err := parseFile(*ibkr, parser.ParseIBKR)
		if err != nil {
			log.Fatalf("ibkr: %v", err)
		}
		allTxs = append(allTxs, txs...)
		fmt.Fprintf(os.Stderr, "IBKR:        %d transactions\n", len(txs))
	}
	if *t212 != "" {
		txs, err := parseFile(*t212, parser.ParseTrading212)
		if err != nil {
			log.Fatalf("trading212: %v", err)
		}
		allTxs = append(allTxs, txs...)
		fmt.Fprintf(os.Stderr, "Trading 212: %d transactions\n", len(txs))
	}

	if len(allTxs) == 0 {
		fmt.Fprintln(os.Stderr, "No input files. Use -revolut, -ibkr, -t212.")
		flag.Usage()
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Total:       %d transactions\n\n", len(allTxs))

	l := ledger.New()
	l.Process(allTxs)

	s := stats.Compute(l, allTxs, time.Now())

	// Fetch live prices for unrealized P&L unless skipped
	if !*noPrices {
		symbols := make([]string, 0, len(s.OpenPositions))
		for _, p := range s.OpenPositions {
			symbols = append(symbols, p.Symbol)
		}
		fmt.Fprintf(os.Stderr, "Fetching prices for %d symbols...\n", len(symbols))
		priceMap, err := prices.FetchQuotes(symbols)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: price fetch failed (%v) — unrealized P&L unavailable\n", err)
		} else {
			stats.EnrichWithPrices(&s, priceMap)
			fmt.Fprintf(os.Stderr, "Prices fetched for %d/%d symbols\n\n", countPriced(s.OpenPositions), len(symbols))
		}
	}

	switch *format {
	case "json":
		r := output.Build(s, l.Realized, allTxs)
		w, close := openWriter(*out)
		defer close()
		if err := output.WriteJSON(w, r); err != nil {
			log.Fatalf("json: %v", err)
		}
	case "csv":
		dir := *out
		if dir == "" {
			dir = "."
		}
		r := output.Build(s, l.Realized, allTxs)
		if err := output.WriteCSV(dir, r); err != nil {
			log.Fatalf("csv: %v", err)
		}
		fmt.Fprintf(os.Stderr, "CSV files written to %s/\n", dir)
	default:
		printText(s, l)
	}
}

func countPriced(positions []stats.PositionSummary) int {
	n := 0
	for _, p := range positions {
		if p.CurrentPrice > 0 {
			n++
		}
	}
	return n
}

func openWriter(path string) (io.Writer, func()) {
	if path == "" {
		return os.Stdout, func() {}
	}
	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("open output: %v", err)
	}
	return f, func() { f.Close() }
}

func parseFile(path string, fn func(r io.Reader) ([]model.Transaction, error)) ([]model.Transaction, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return fn(f)
}

// ── Text output ──────────────────────────────────────────────────────────────

func printText(s stats.Summary, l *ledger.Ledger) {
	printPositions(s.OpenPositions)
	printRealizedBySymbol(l.Realized)
	printPeriod(s.AllTime)
	printPeriod(s.YTD)
	printPeriod(s.MTD)
	printByYear(s.ByYear)
	printDividends(s.BySymbol)
}

func printPositions(positions []stats.PositionSummary) {
	hasPrices := false
	for _, p := range positions {
		if p.CurrentPrice > 0 {
			hasPrices = true
			break
		}
	}

	fmt.Println("=== Open Positions ===")
	if hasPrices {
		fmt.Printf("%-10s %12s %10s %12s %12s %14s %8s\n",
			"Symbol", "Qty", "Avg Cost", "Cost Basis", "Mkt Value", "Unrealized", "%")
		fmt.Println(strings.Repeat("-", 84))
		var totalCost, totalMkt, totalUnreal float64
		for _, p := range positions {
			fmt.Printf("%-10s %12.4f %10.2f %12.2f %12.2f %+14.2f %+7.1f%%\n",
				p.Symbol, p.Quantity, p.AvgCost, p.TotalCost,
				p.MarketValue, p.UnrealizedPnL, p.UnrealizedPct)
			totalCost += p.TotalCost
			totalMkt += p.MarketValue
			totalUnreal += p.UnrealizedPnL
		}
		pct := 0.0
		if totalCost > 0.01 {
			pct = totalUnreal / totalCost * 100
		}
		fmt.Println(strings.Repeat("-", 84))
		fmt.Printf("%-10s %12s %10s %12.2f %12.2f %+14.2f %+7.1f%%\n",
			"TOTAL", "", "", totalCost, totalMkt, totalUnreal, pct)
	} else {
		fmt.Printf("%-10s %12s %10s %12s\n", "Symbol", "Qty", "Avg Cost", "Cost Basis")
		fmt.Println(strings.Repeat("-", 48))
		for _, p := range positions {
			fmt.Printf("%-10s %12.4f %10.4f %12.2f\n",
				p.Symbol, p.Quantity, p.AvgCost, p.TotalCost)
		}
	}
	fmt.Println()
}

func printRealizedBySymbol(realized []ledger.RealizedTx) {
	bySymbol := stats.RealizedBySymbol(realized)
	type row struct {
		sym string
		pnl float64
	}
	var rows []row
	for sym, pnl := range bySymbol {
		rows = append(rows, row{sym, pnl})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].pnl > rows[j].pnl })

	fmt.Println("=== Realized P&L by Symbol ===")
	fmt.Printf("%-10s %12s\n", "Symbol", "P&L")
	fmt.Println(strings.Repeat("-", 25))
	var total float64
	for _, r := range rows {
		fmt.Printf("%-10s %+12.2f\n", r.sym, r.pnl)
		total += r.pnl
	}
	fmt.Printf("%-10s %+12.2f\n", "TOTAL", total)
	fmt.Println()
}

func printPeriod(p stats.PeriodSummary) {
	fmt.Printf("=== %s ===\n", p.Label)
	fmt.Printf("  Realized P&L:   %+12.2f\n", p.Realized)
	fmt.Printf("  Dividends:      %+12.2f  (tax withheld: %.2f)\n", p.Dividends, -p.TaxWithheld)
	fmt.Printf("  Fees:           %+12.2f\n", p.Fees)
	fmt.Printf("  Deposits:       %+12.2f\n", p.Deposits)
	fmt.Printf("  Withdrawals:    %+12.2f\n", p.Withdrawals)
	fmt.Printf("  Buy volume:     %12.2f\n", p.BuyVolume)
	fmt.Printf("  Sell volume:    %12.2f\n", p.SellVolume)
	fmt.Println()
}

func printByYear(years []stats.PeriodSummary) {
	fmt.Println("=== Year-by-Year Breakdown ===")
	fmt.Printf("%-6s %12s %12s %12s %12s %12s %12s\n",
		"Year", "Realized", "Dividends", "TaxWithheld", "Fees", "Deposits", "Withdrawals")
	fmt.Println(strings.Repeat("-", 84))
	for _, p := range years {
		fmt.Printf("%-6s %+12.2f %+12.2f %12.2f %12.2f %12.2f %12.2f\n",
			p.Label, p.Realized, p.Dividends, -p.TaxWithheld,
			p.Fees, p.Deposits, p.Withdrawals)
	}
	fmt.Println()
}

func printDividends(divs []stats.DividendBySymbol) {
	fmt.Println("=== Dividends by Symbol (all time) ===")
	fmt.Printf("%-10s %10s %12s %10s\n", "Symbol", "Gross", "Tax Withheld", "Net")
	fmt.Println(strings.Repeat("-", 46))
	var totalGross, totalTax, totalNet float64
	for _, d := range divs {
		fmt.Printf("%-10s %10.2f %12.2f %10.2f\n", d.Symbol, d.Gross, d.TaxWithheld, d.Net)
		totalGross += d.Gross
		totalTax += d.TaxWithheld
		totalNet += d.Net
	}
	fmt.Printf("%-10s %10.2f %12.2f %10.2f\n", "TOTAL", totalGross, totalTax, totalNet)
	fmt.Println()
}
