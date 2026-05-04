package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
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
	dataDir := flag.String("data", "data", "Directory to scan for broker CSV/XLSX files (auto-detected)")
	revolut := flag.String("revolut", "", "Explicit Revolut CSV file (overrides -data scan)")
	ibkr := flag.String("ibkr", "", "Explicit IBKR CSV file (overrides -data scan)")
	t212 := flag.String("t212", "", "Explicit Trading 212 CSV file (overrides -data scan)")
	xtb := flag.String("xtb", "", "Explicit XTB XLSX file (overrides -data scan)")
	broker := flag.String("broker", "", "Filter output to a single broker: revolut | ibkr | trading212 | xtb | tradeville")
	format := flag.String("format", "text", "Output format: text | json")
	out := flag.String("out", "", "Output file for json (stdout if empty)")
	noPrices := flag.Bool("no-prices", false, "Skip live price fetch (no unrealized P&L)")
	flag.Parse()

	var allTxs []model.Transaction

	// Auto-scan the data directory first (if it exists)
	if info, err := os.Stat(*dataDir); err == nil && info.IsDir() {
		fmt.Fprintf(os.Stderr, "Scanning %s/\n", *dataDir)
		txs, err := parser.LoadDir(*dataDir, os.Stderr)
		if err != nil {
			log.Fatalf("data dir: %v", err)
		}
		allTxs = append(allTxs, txs...)
	}

	// Explicit files supplement (or replace) the directory scan
	if *revolut != "" {
		txs, err := parseFile(*revolut, parser.ParseRevolut)
		if err != nil {
			log.Fatalf("revolut: %v", err)
		}
		allTxs = append(allTxs, txs...)
		fmt.Fprintf(os.Stderr, "Revolut (explicit): %d transactions\n", len(txs))
	}
	if *ibkr != "" {
		txs, err := parseFile(*ibkr, parser.ParseIBKR)
		if err != nil {
			log.Fatalf("ibkr: %v", err)
		}
		allTxs = append(allTxs, txs...)
		fmt.Fprintf(os.Stderr, "IBKR (explicit):    %d transactions\n", len(txs))
	}
	if *t212 != "" {
		txs, err := parseFile(*t212, parser.ParseTrading212)
		if err != nil {
			log.Fatalf("trading212: %v", err)
		}
		allTxs = append(allTxs, txs...)
		fmt.Fprintf(os.Stderr, "T212 (explicit):    %d transactions\n", len(txs))
	}
	if *xtb != "" {
		txs, err := parser.ParseXTB(*xtb)
		if err != nil {
			log.Fatalf("xtb: %v", err)
		}
		allTxs = append(allTxs, txs...)
		fmt.Fprintf(os.Stderr, "XTB (explicit):     %d transactions\n", len(txs))
	}

	// Final dedup across all sources (explicit files may overlap with dir scan)
	var dropped []model.Transaction
	allTxs, dropped = parser.Dedup(allTxs)
	if len(dropped) > 0 {
		logPath := "dedup.log.csv"
		if err := parser.WriteDedupeLog(logPath, dropped); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not write dedup log: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Final dedup: removed %d duplicate(s) — logged to %s\n", len(dropped), logPath)
		}
	}

	if len(allTxs) == 0 {
		fmt.Fprintln(os.Stderr, "No transactions found. Drop CSV/XLSX files into data/ or use -revolut/-ibkr/-t212/-xtb.")
		flag.Usage()
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Total:       %d transactions\n\n", len(allTxs))

	// Validate broker filter if provided.
	if *broker != "" {
		validBrokers := []string{parser.BrokerRevolut, parser.BrokerIBKR, parser.BrokerTrading212, parser.BrokerXTB, parser.BrokerTradeville}
		if !contains(validBrokers, *broker) {
			log.Fatalf("unknown broker %q — valid values: %s", *broker, strings.Join(validBrokers, ", "))
		}
	}

	const baseCurrency = "USD"
	fxRates := map[string]float64{baseCurrency: 1.0}
	if !*noPrices {
		currencies := uniqueCurrencies(allTxs)
		fmt.Fprintf(os.Stderr, "Fetching FX rates for %v → %s...\n", currencies, baseCurrency)
		if rates, err := prices.FetchFXRates(currencies, baseCurrency); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: FX fetch failed (%v) — amounts not normalized\n", err)
		} else {
			fxRates = rates
			for c, r := range rates {
				if c != baseCurrency {
					fmt.Fprintf(os.Stderr, "  1 %s = %.4f %s\n", c, r, baseCurrency)
				}
			}
		}
	}

	// Build the combined (cross-broker) ledger — used for both the combined
	// section and as the authoritative source for cross-broker position closures.
	combinedLedger := ledger.New()
	combinedLedger.Process(allTxs)
	combinedStats := stats.Compute(combinedLedger, allTxs, time.Now(), fxRates, baseCurrency)

	// Fetch live prices once for all unique symbols; used by both per-broker
	// and combined sections.
	var priceMap map[string]float64
	if !*noPrices {
		allSymbols := uniqueSymbols(combinedStats.OpenPositions)
		fmt.Fprintf(os.Stderr, "Fetching prices for %d symbols...\n", len(allSymbols))
		var err error
		priceMap, err = prices.FetchQuotes(allSymbols)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: price fetch failed (%v) — unrealized P&L unavailable\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Prices fetched for %d/%d symbols\n\n", len(priceMap)/2, len(allSymbols))
		}
	}
	stats.EnrichWithPrices(&combinedStats, priceMap)

	now := time.Now()

	// Compute per-broker stats (used by both text and json output).
	type brokerEntry struct {
		name string
		s    stats.Summary
		l    *ledger.Ledger
	}
	var brokerEntries []brokerEntry
	for _, b := range brokersInOrder(allTxs) {
		brokerTxs := filterByBroker(allTxs, b)
		nativeCur := brokerNativeCurrency(brokerTxs)
		bl := ledger.New()
		bl.Process(brokerTxs)
		bs := stats.Compute(bl, brokerTxs, now, nil, nativeCur)
		stats.EnrichWithPrices(&bs, priceMap)
		brokerEntries = append(brokerEntries, brokerEntry{b, bs, bl})
	}

	switch *format {
	case "json":
		var brokerReports []output.BrokerReport
		for _, be := range brokerEntries {
			brokerReports = append(brokerReports, output.BuildBrokerReport(be.name, be.s, be.l.Realized))
		}
		r := output.Build(combinedStats, combinedLedger.Realized, brokerReports)
		w, close := openWriter(*out)
		defer close()
		if err := output.WriteJSON(w, r); err != nil {
			log.Fatalf("json: %v", err)
		}
	default:
		entries := brokerEntries
		// If -broker is set, restrict to that one broker only (no combined section).
		if *broker != "" {
			entries = nil
			for _, be := range brokerEntries {
				if be.name == *broker {
					entries = []brokerEntry{be}
					break
				}
			}
		}
		for _, be := range entries {
			printSectionHeader(be.name, be.s.BaseCurrency)
			printText(be.s, be.l)
		}
		// Combined section — only shown when not filtering to a single broker.
		if *broker == "" {
			printSectionHeader("COMBINED", baseCurrency)
			printText(combinedStats, combinedLedger)
		}
	}
}

// brokersInOrder returns unique broker names present in txs using a stable preferred order.
func brokersInOrder(txs []model.Transaction) []string {
	preferred := []string{
		parser.BrokerTrading212,
		parser.BrokerRevolut,
		parser.BrokerIBKR,
		parser.BrokerXTB,
		parser.BrokerTradeville,
	}
	present := map[string]bool{}
	for _, tx := range txs {
		present[tx.Broker] = true
	}
	var out []string
	for _, b := range preferred {
		if present[b] {
			out = append(out, b)
			delete(present, b)
		}
	}
	// Append any unknown brokers alphabetically.
	var extra []string
	for b := range present {
		extra = append(extra, b)
	}
	sort.Strings(extra)
	return append(out, extra...)
}

// brokerNativeCurrency returns the most common currency among buy/sell/deposit transactions
// for a set of broker transactions.
func brokerNativeCurrency(txs []model.Transaction) string {
	counts := map[string]int{}
	for _, tx := range txs {
		if isCurrencyCode.MatchString(tx.Currency) &&
			(tx.Type == model.TxBuy || tx.Type == model.TxSell || tx.Type == model.TxDeposit) {
			counts[tx.Currency]++
		}
	}
	best, bestN := "USD", 0
	for c, n := range counts {
		if n > bestN {
			best, bestN = c, n
		}
	}
	return best
}

func uniqueSymbols(positions []stats.PositionSummary) []string {
	out := make([]string, 0, len(positions))
	for _, p := range positions {
		out = append(out, p.Symbol)
	}
	return out
}

func printSectionHeader(name, currency string) {
	title := fmt.Sprintf("  %s  (%s)  ", strings.ToUpper(name), currency)
	line := strings.Repeat("─", len(title))
	fmt.Println()
	fmt.Println(line)
	fmt.Println(title)
	fmt.Println(line)
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

// openPositionsByBroker returns open positions from the combined ledger whose
// remaining FIFO lots were contributed by the given broker. Positions whose
// lots were fully consumed by sells at another broker (e.g. post-migration)
// will not appear.
func openPositionsByBroker(l *ledger.Ledger, broker string, fxRates map[string]float64) []stats.PositionSummary {
	var result []stats.PositionSummary
	for sym, pos := range l.Positions {
		var qty, cost float64
		var currency string
		for _, lot := range pos.Lots {
			if lot.Broker == broker {
				qty += lot.Quantity
				cost += toBaseRate(lot.CostBasis, lot.Currency, fxRates)
				if currency == "" {
					currency = lot.Currency
				}
			}
		}
		if qty > 1e-6 {
			avgCost := 0.0
			if qty > 0 {
				avgCost = cost / qty
			}
			result = append(result, stats.PositionSummary{
				Symbol:    sym,
				Currency:  currency,
				Quantity:  qty,
				AvgCost:   avgCost,
				TotalCost: cost,
			})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Symbol < result[j].Symbol })
	return result
}

func toBaseRate(amount float64, currency string, fxRates map[string]float64) float64 {
	if len(fxRates) == 0 {
		return amount
	}
	if rate, ok := fxRates[currency]; ok {
		return amount * rate
	}
	return amount
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

var isCurrencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

func uniqueCurrencies(txs []model.Transaction) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, tx := range txs {
		if isCurrencyCode.MatchString(tx.Currency) {
			if _, ok := seen[tx.Currency]; !ok {
				seen[tx.Currency] = struct{}{}
				out = append(out, tx.Currency)
			}
		}
	}
	return out
}

func filterByBroker(txs []model.Transaction, broker string) []model.Transaction {
	out := txs[:0:0]
	for _, tx := range txs {
		if tx.Broker == broker {
			out = append(out, tx)
		}
	}
	return out
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
	cur := s.BaseCurrency
	printPositions(s.OpenPositions, cur)
	printRealizedBySymbol(l.Realized, cur)
	printPeriod(s.AllTime)
	printPeriod(s.YTD)
	printPeriod(s.MTD)
	printByYear(s.ByYear)
	printDividends(s.BySymbol, cur)
}

func printPositions(positions []stats.PositionSummary, cur string) {
	hasPrices := false
	for _, p := range positions {
		if p.CurrentPrice > 0 {
			hasPrices = true
			break
		}
	}

	fmt.Printf("=== Open Positions (%s) ===\n", cur)
	if hasPrices {
		fmt.Printf("%-10s %-4s %12s %10s %12s %12s %14s %8s\n",
			"Symbol", "Cur", "Qty", "Avg Cost", "Cost Basis", "Mkt Value", "Unrealized", "%")
		fmt.Println(strings.Repeat("-", 92))
		var totalCost, totalMkt, totalUnreal float64
		for _, p := range positions {
			fmt.Printf("%-10s %-4s %12.4f %10.2f %12.2f %12.2f %+14.2f %+7.1f%%\n",
				p.Symbol, p.Currency, p.Quantity, p.AvgCost, p.TotalCost,
				p.MarketValue, p.UnrealizedPnL, p.UnrealizedPct)
			totalCost += p.TotalCost
			totalMkt += p.MarketValue
			totalUnreal += p.UnrealizedPnL
		}
		pct := 0.0
		if totalCost > 0.01 {
			pct = totalUnreal / totalCost * 100
		}
		fmt.Println(strings.Repeat("-", 92))
		fmt.Printf("%-10s %-4s %12s %10s %12.2f %12.2f %+14.2f %+7.1f%%\n",
			"TOTAL", "", "", "", totalCost, totalMkt, totalUnreal, pct)
	} else {
		fmt.Printf("%-10s %-4s %12s %10s %12s\n", "Symbol", "Cur", "Qty", "Avg Cost", "Cost Basis")
		fmt.Println(strings.Repeat("-", 52))
		for _, p := range positions {
			fmt.Printf("%-10s %-4s %12.4f %10.2f %12.2f\n",
				p.Symbol, p.Currency, p.Quantity, p.AvgCost, p.TotalCost)
		}
	}
	fmt.Println()
}

func printRealizedBySymbol(realized []ledger.RealizedTx, cur string) {
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

	fmt.Printf("=== Realized P&L by Symbol (%s) ===\n", cur)
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
	fmt.Printf("  Commissions:    %+12.2f\n", p.Commissions)
	fmt.Printf("  Fees:           %+12.2f\n", p.Fees)
	fmt.Printf("  Deposits:       %+12.2f\n", p.Deposits)
	fmt.Printf("  Withdrawals:    %+12.2f\n", p.Withdrawals)
	fmt.Printf("  Buy volume:     %12.2f\n", p.BuyVolume)
	fmt.Printf("  Sell volume:    %12.2f\n", p.SellVolume)
	fmt.Println()
}

func printByYear(years []stats.PeriodSummary) {
	fmt.Println("=== Year-by-Year Breakdown ===")
	fmt.Printf("%-6s %12s %12s %12s %12s %12s %12s %12s %8s\n",
		"Year", "Realized", "Dividends", "TaxWithheld", "Commissions", "Fees", "Deposits", "Withdrawals", "Gain%")
	fmt.Println(strings.Repeat("-", 108))
	for _, p := range years {
		gainStr := "n/a"
		if p.Deposits > 0.01 {
			gainStr = fmt.Sprintf("%+.1f%%", p.GainPct)
		}
		fmt.Printf("%-6s %+12.2f %+12.2f %12.2f %12.2f %12.2f %12.2f %12.2f %8s\n",
			p.Label, p.Realized, p.Dividends, -p.TaxWithheld,
			p.Commissions, p.Fees, p.Deposits, p.Withdrawals, gainStr)
	}
	fmt.Println()
}

func printDividends(divs []stats.DividendBySymbol, cur string) {
	fmt.Printf("=== Dividends by Symbol — all time (%s) ===\n", cur)
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
