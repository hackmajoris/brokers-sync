package parser

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"brokers-sync/internal/model"
)

// Broker names returned by Detect.
const (
	BrokerRevolut    = "revolut"
	BrokerIBKR       = "ibkr"
	BrokerTrading212 = "trading212"
	BrokerXTB        = "xtb"
	BrokerTradeville = "tradeville"
)

// Detect reads a file and returns the broker name.
// For .xlsx files it checks for the XTB Cash Operations sheet; for CSV files it inspects the first line.
func Detect(path string) (string, error) {
	if strings.HasSuffix(strings.ToLower(path), ".xlsx") {
		return detectXlsx(path)
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	line, err := firstLine(f)
	if err != nil {
		return "", fmt.Errorf("detect %s: %w", path, err)
	}

	switch {
	case strings.Contains(line, "Statement,") && strings.Contains(line, "Header"):
		return BrokerIBKR, nil
	case strings.HasPrefix(line, "Action,") && strings.Contains(line, "ISIN"):
		return BrokerTrading212, nil
	case strings.HasPrefix(line, "Date,") && strings.Contains(line, "Price per share"):
		return BrokerRevolut, nil
	case strings.HasPrefix(strings.TrimRight(line, "\r"), "SEP="):
		return BrokerTradeville, nil
	default:
		return "", fmt.Errorf("unrecognised format in %s (first line: %q)", filepath.Base(path), line)
	}
}

// AutoParse detects the broker format and parses the file.
func AutoParse(path string) (string, []model.Transaction, error) {
	broker, err := Detect(path)
	if err != nil {
		return "", nil, err
	}

	// XTB uses xlsx; parse directly by path without an io.Reader.
	if broker == BrokerXTB {
		txs, err := ParseXTB(path)
		if err != nil {
			return broker, nil, fmt.Errorf("%s (%s): %w", broker, filepath.Base(path), err)
		}
		return broker, txs, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return broker, nil, err
	}
	defer f.Close()

	var txs []model.Transaction
	switch broker {
	case BrokerRevolut:
		txs, err = ParseRevolut(f)
	case BrokerIBKR:
		txs, err = ParseIBKR(f)
	case BrokerTrading212:
		txs, err = ParseTrading212(f)
	case BrokerTradeville:
		txs, err = ParseTradeville(f)
	}
	if err != nil {
		return broker, nil, fmt.Errorf("%s (%s): %w", broker, filepath.Base(path), err)
	}
	return broker, txs, nil
}

// LoadDir scans dir for CSV files, auto-detects each one, parses them all,
// and returns a deduplicated merged slice sorted by date.
// Files that cannot be detected are skipped with a warning printed to w.
func LoadDir(dir string, warn io.Writer) ([]model.Transaction, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	counts := map[string]int{}
	var all []model.Transaction

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".csv") && !strings.HasSuffix(lower, ".xlsx") {
			continue
		}

		path := filepath.Join(dir, name)
		broker, txs, err := AutoParse(path)
		if err != nil {
			fmt.Fprintf(warn, "  skip %s: %v\n", name, err)
			continue
		}
		counts[broker] += len(txs)
		all = append(all, txs...)
		fmt.Fprintf(warn, "  %-14s → %-12s (%d transactions)\n", name, broker, len(txs))
	}

	kept, dropped := Dedup(all)

	if len(dropped) > 0 {
		logPath := filepath.Join(dir, "dedup.log.csv")
		if err := WriteDedupeLog(logPath, dropped); err != nil {
			fmt.Fprintf(warn, "  dedup: warning: could not write log: %v\n", err)
		} else {
			fmt.Fprintf(warn, "  dedup: removed %d duplicate(s) — logged to %s\n", len(dropped), logPath)
		}
	}

	return kept, nil
}

// Dedup removes transactions with duplicate IDs, keeping the first occurrence.
// Returns (kept, dropped).
func Dedup(txs []model.Transaction) (kept, dropped []model.Transaction) {
	seen := make(map[string]struct{}, len(txs))
	for _, tx := range txs {
		if _, ok := seen[tx.ID]; ok {
			dropped = append(dropped, tx)
			continue
		}
		seen[tx.ID] = struct{}{}
		kept = append(kept, tx)
	}
	return kept, dropped
}

// WriteDedupeLog writes dropped duplicate transactions to a CSV file at path.
func WriteDedupeLog(path string, dropped []model.Transaction) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	_ = w.Write([]string{"id", "date", "broker", "account", "type", "symbol", "quantity", "net", "notes"})
	for _, tx := range dropped {
		_ = w.Write([]string{
			tx.ID,
			tx.Date.UTC().Format(time.RFC3339),
			tx.Broker,
			tx.Account,
			string(tx.Type),
			tx.Symbol,
			fmt.Sprintf("%f", tx.Quantity),
			fmt.Sprintf("%f", tx.Net),
			tx.Notes,
		})
	}
	w.Flush()
	return w.Error()
}

func firstLine(r io.Reader) (string, error) {
	sc := bufio.NewScanner(r)
	if sc.Scan() {
		return sc.Text(), nil
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("empty file")
}
