package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dataDir := flag.String("data", "data", "Directory containing broker CSV/XLSX files and result.json")
	webDir := flag.String("web", "web/dist", "Directory with built web assets")
	flag.Parse()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/upload/zip", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleUpload(w, r)
	})

	mux.HandleFunc("/data/result.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(*dataDir, "result.json"))
	})

	mux.Handle("/", http.FileServer(http.Dir(*webDir)))

	log.Printf("listening on %s  (data=%s  web=%s)", *addr, *dataDir, *webDir)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

// sseEvent writes a single SSE event to w and flushes immediately.
func sseEvent(w http.ResponseWriter, payload any) {
	b, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	logf := func(msg string) {
		sseEvent(w, map[string]string{"type": "log", "line": msg})
	}

	fail := func(msg string) {
		sseEvent(w, map[string]any{"type": "done", "success": false, "error": msg})
	}

	const maxZipBytes = 10 << 20 // 10 MB — matches API Gateway hard limit

	if err := r.ParseMultipartForm(maxZipBytes); err != nil {
		fail("parse form: " + err.Error())
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		fail("read file: " + err.Error())
		return
	}
	defer func() { _ = f.Close() }()

	buf, err := io.ReadAll(f)
	if err != nil {
		fail("read body: " + err.Error())
		return
	}

	if len(buf) > maxZipBytes {
		fail(fmt.Sprintf("zip file is too large (%.1f MB); maximum allowed size is 10 MB", float64(len(buf))/1024/1024))
		return
	}

	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		fail("open zip: " + err.Error())
		return
	}

	// Extract CSV/XLSX/PDF files from the zip into a temp directory.
	tmpDir, err := os.MkdirTemp("", "brokers-upload-*")
	if err != nil {
		fail("create temp dir: " + err.Error())
		return
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	extracted := 0
	for _, zf := range zr.File {
		name := filepath.Base(zf.Name)
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".csv") && !strings.HasSuffix(lower, ".xlsx") && !strings.HasSuffix(lower, ".pdf") {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			logf(fmt.Sprintf("skip %s: %v", name, err))
			continue
		}
		dst := filepath.Join(tmpDir, name)
		out, err := os.Create(dst)
		if err != nil {
			_ = rc.Close()
			logf(fmt.Sprintf("skip %s: %v", name, err))
			continue
		}
		_, err = io.Copy(out, rc)
		_ = rc.Close()
		_ = out.Close()
		if err != nil {
			logf(fmt.Sprintf("skip %s: %v", name, err))
			continue
		}
		extracted++
		logf(fmt.Sprintf("extracted %s", name))
	}

	if extracted == 0 {
		fail("no CSV, XLSX, or PDF files found in the zip")
		return
	}

	logf(fmt.Sprintf("parsing %d file(s)…", extracted))

	var logBuf strings.Builder
	txs, err := parser.LoadDir(tmpDir, &logBuf)
	for _, line := range strings.Split(strings.TrimSpace(logBuf.String()), "\n") {
		if line != "" {
			logf(line)
		}
	}
	if err != nil {
		fail("parse: " + err.Error())
		return
	}
	if len(txs) == 0 {
		fail("no transactions found in the uploaded files")
		return
	}
	txs = ledger.ReconcileTransfers(txs)
	logf(fmt.Sprintf("total: %d transactions", len(txs)))

	// Pipeline.
	const baseCurrency = "USD"
	logf("fetching FX rates…")
	fxRates := map[string]float64{baseCurrency: 1.0}
	if rates, err := prices.FetchFXRates(context.Background(), uniqueCurrencies(txs), baseCurrency); err != nil {
		logf("warning: FX fetch failed — amounts not normalized")
	} else {
		fxRates = rates
		for c, rate := range rates {
			if c != baseCurrency {
				logf(fmt.Sprintf("  1 %s = %.4f %s", c, rate, baseCurrency))
			}
		}
	}

	combinedLedger := ledger.New()
	combinedLedger.Process(txs)
	combinedStats := stats.Compute(combinedLedger, txs, time.Now(), fxRates, baseCurrency)

	yahooTickers, reverseMap := buildYahooTickerMap(combinedStats.OpenPositions)
	log.Printf("fetching prices for %d symbol(s): %v", len(yahooTickers), yahooTickers)
	logf(fmt.Sprintf("fetching prices for %d symbol(s): %v", len(yahooTickers), yahooTickers))
	priceMap, err := prices.FetchQuotes(context.Background(), yahooTickers)
	if err != nil {
		log.Printf("price fetch error: %v", err)
		logf("warning: price fetch failed — unrealized P&L unavailable")
	} else {
		for yahooTicker, origSymbol := range reverseMap {
			if p, ok := priceMap[yahooTicker]; ok {
				priceMap[origSymbol] = p
				log.Printf("  mapped %s → %s = %.4f", yahooTicker, origSymbol, p)
				logf(fmt.Sprintf("  mapped %s → %s = %.4f", yahooTicker, origSymbol, p))
			} else {
				log.Printf("  warning: no price returned for %s", yahooTicker)
				logf(fmt.Sprintf("  warning: no price returned for %s", yahooTicker))
			}
		}
		log.Printf("prices fetched for %d/%d symbols", len(priceMap)/2, len(yahooTickers))
		logf(fmt.Sprintf("prices fetched for %d/%d symbols", len(priceMap)/2, len(yahooTickers)))
	}
	// Positions with no live price after a mostly-successful fetch are presumed
	// delisted/worthless; book zero-proceeds write-offs and recompute so the loss
	// is realized and the per-broker reports below (which filter txs) stay consistent.
	if writeoffs := stats.AutoWriteOffs(combinedLedger, priceMap, time.Now(), io.Discard); len(writeoffs) > 0 {
		for _, wo := range writeoffs {
			log.Printf("auto write-off: %s (no live price — presumed delisted)", wo.Symbol)
			logf(fmt.Sprintf("auto write-off: %s (no live price — presumed delisted)", wo.Symbol))
		}
		txs = append(txs, writeoffs...)
		combinedLedger = ledger.New()
		combinedLedger.Process(txs)
		combinedStats = stats.Compute(combinedLedger, txs, time.Now(), fxRates, baseCurrency)
	}

	logf(fmt.Sprintf("fetching 52-week ranges for %d symbol(s)…", len(yahooTickers)))
	rangeMap, err := prices.FetchFiftyTwoWeekRanges(context.Background(), yahooTickers)
	lowMap := make(map[string]float64, len(rangeMap))
	highMap := make(map[string]float64, len(rangeMap))
	if err != nil {
		log.Printf("52-week range fetch error: %v", err)
		logf("warning: 52-week range fetch failed")
	} else {
		for yahooTicker, rng := range rangeMap {
			lowMap[yahooTicker] = rng.Low
			highMap[yahooTicker] = rng.High
			if origSymbol, ok := reverseMap[yahooTicker]; ok {
				lowMap[origSymbol] = rng.Low
				highMap[origSymbol] = rng.High
			}
		}
	}

	logf(fmt.Sprintf("fetching P/E ratios for %d symbol(s)…", len(yahooTickers)))
	peRatioMap, err := prices.FetchPERatios(context.Background(), yahooTickers)
	peMap := make(map[string]float64, len(peRatioMap))
	forwardPEMap := make(map[string]float64, len(peRatioMap))
	if err != nil {
		log.Printf("P/E ratio fetch error: %v", err)
		logf("warning: P/E ratio fetch failed")
	} else {
		for yahooTicker, r := range peRatioMap {
			peMap[yahooTicker] = r.PE
			forwardPEMap[yahooTicker] = r.ForwardPE
			if origSymbol, ok := reverseMap[yahooTicker]; ok {
				peMap[origSymbol] = r.PE
				forwardPEMap[origSymbol] = r.ForwardPE
			}
		}
	}

	stats.EnrichWithPrices(&combinedStats, priceMap, fxRates)
	stats.EnrichWithFiftyTwoWeekRange(&combinedStats, lowMap, highMap, fxRates)
	stats.EnrichWithPERatio(&combinedStats, peMap, forwardPEMap)
	stats.RecalcGainPct(&combinedStats)

	var brokerReports []output.BrokerReport
	for _, b := range brokersInOrder(txs) {
		brokerTxs := filterByBroker(txs, b)
		nativeCur := brokerNativeCurrency(brokerTxs)
		bl := ledger.New()
		bl.Process(brokerTxs)
		brokerFxRates := rebaseFXRates(fxRates, nativeCur)
		bs := stats.Compute(bl, brokerTxs, time.Now(), brokerFxRates, nativeCur)
		stats.EnrichWithPrices(&bs, priceMap, brokerFxRates)
		stats.EnrichWithFiftyTwoWeekRange(&bs, lowMap, highMap, brokerFxRates)
		stats.EnrichWithPERatio(&bs, peMap, forwardPEMap)
		stats.RecalcGainPct(&bs)
		brokerReports = append(brokerReports, output.BuildBrokerReport(b, bs, bl.Realized))
	}

	report := output.Build(combinedStats, combinedLedger.Realized, brokerReports)

	sseEvent(w, map[string]any{"type": "done", "success": true, "report": report})
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

func buildYahooTickerMap(positions []stats.PositionSummary) (tickers []string, reverseMap map[string]string) {
	reverseMap = make(map[string]string)
	for _, p := range positions {
		yahoo := prices.YahooTicker(p.Symbol, p.Currency)
		tickers = append(tickers, yahoo)
		if yahoo != p.Symbol {
			reverseMap[yahoo] = p.Symbol
		}
	}
	return tickers, reverseMap
}

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
	var extra []string
	for b := range present {
		extra = append(extra, b)
	}
	sort.Strings(extra)
	return append(out, extra...)
}

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

// rebaseFXRates converts a fxRates map (currency -> rate to its original base)
// into one expressed relative to newBase, so amounts in any currency convert
// correctly into newBase instead of being left at face value.
func rebaseFXRates(fxRates map[string]float64, newBase string) map[string]float64 {
	baseRate, ok := fxRates[newBase]
	if !ok || baseRate == 0 {
		return fxRates
	}
	rebased := make(map[string]float64, len(fxRates))
	for c, rate := range fxRates {
		rebased[c] = rate / baseRate
	}
	return rebased
}

func filterByBroker(txs []model.Transaction, broker string) []model.Transaction {
	var out []model.Transaction
	for _, tx := range txs {
		if tx.Broker == broker {
			out = append(out, tx)
		}
	}
	return out
}
