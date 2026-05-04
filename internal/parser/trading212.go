package parser

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"brokers-sync/internal/model"
)

// Trading 212 CSV columns (base set, present in all export versions):
// Action, Time, ISIN, Ticker, Name, Notes, ID, No. of shares,
// Price / share, Currency (Price / share), Exchange rate,
// Result, Currency (Result), Total, Currency (Total)
//
// Newer exports append additional fee columns:
// Withholding tax, Currency (Withholding tax),
// Charge amount, Currency (Charge amount),
// Deposit fee, Currency (Deposit fee),
// Currency conversion fee, Currency (Currency conversion fee)

var t212DateFormats = []string{
	"2006-01-02 15:04:05",
	"2006-01-02 15:04:05.999999",
	time.RFC3339,
}

// t212FeeColumns lists optional columns that represent costs charged by T212.
var t212FeeColumns = []string{
	"Withholding tax",
	"Charge amount",
	"Deposit fee",
	"Currency conversion fee",
}

func ParseTrading212(r io.Reader) ([]model.Transaction, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1 // tolerate variable column count across export versions

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("trading212: read header: %w", err)
	}
	idx, err := mapColumns(header, []string{
		"Action", "Time", "ISIN", "Ticker", "Name",
		"No. of shares", "Price / share", "Currency (Price / share)",
		"Exchange rate", "Total", "Currency (Total)",
	})
	if err != nil {
		return nil, fmt.Errorf("trading212: %w", err)
	}

	var txs []model.Transaction
	lineNum := 1
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("trading212: line %d: %w", lineNum, err)
		}
		lineNum++

		tx, err := parseT212Row(record, idx)
		if err != nil {
			return nil, fmt.Errorf("trading212: line %d: %w", lineNum, err)
		}
		tx.Broker = "trading212"
		tx.ID = syntheticID(tx)
		txs = append(txs, tx)
	}
	return txs, nil
}

func parseT212Row(r []string, idx map[string]int) (model.Transaction, error) {
	var tx model.Transaction
	var err error

	dateStr := strings.TrimSpace(r[idx["Time"]])
	tx.Date, err = parseAnyTime(t212DateFormats, dateStr)
	if err != nil {
		return tx, fmt.Errorf("date %q: %w", dateStr, err)
	}

	tx.Symbol = strings.TrimSpace(r[idx["Ticker"]])
	tx.ISIN = strings.TrimSpace(r[idx["ISIN"]])
	tx.Name = strings.TrimSpace(r[idx["Name"]])
	tx.Currency = strings.TrimSpace(r[idx["Currency (Total)"]])

	rawAction := strings.TrimSpace(r[idx["Action"]])
	tx.Type = mapT212Type(rawAction)
	tx.Notes = rawAction

	if s := strings.TrimSpace(r[idx["No. of shares"]]); s != "" {
		tx.Quantity, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("shares %q: %w", s, err)
		}
	}

	if s := strings.TrimSpace(r[idx["Price / share"]]); s != "" {
		tx.Price, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("price %q: %w", s, err)
		}
	}

	if s := strings.TrimSpace(r[idx["Exchange rate"]]); s != "" {
		tx.FXRate, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("exchange rate %q: %w", s, err)
		}
	}

	// Total is always positive in T212: it's the absolute USD amount paid (buy) or received (sell).
	// Net keeps this positive value; the ledger handles both sign conventions.
	if s := strings.TrimSpace(r[idx["Total"]]); s != "" {
		tx.Net, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("total %q: %w", s, err)
		}
		tx.Gross = tx.Net
	}

	// Result is the realized P&L reported by T212 (absent in some export versions).
	// Use it as the authoritative P&L for sells — it accounts for corporate actions
	// (splits, buybacks) that may not appear as separate transactions in the export.
	if i, ok := idx["Result"]; ok && i < len(r) {
		if s := strings.TrimSpace(r[i]); s != "" {
			if tx.Type == model.TxSell {
				if v, err := strconv.ParseFloat(s, 64); err == nil {
					tx.BrokerPnL = v
				}
			}
			tx.Notes = rawAction + " result=" + s
		}
	}

	// Sum optional fee columns (present only in newer export versions) into Commission.
	for _, col := range t212FeeColumns {
		colIdx, ok := idx[col]
		if !ok || colIdx >= len(r) {
			continue
		}
		s := strings.TrimSpace(r[colIdx])
		if s == "" {
			continue
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			continue
		}
		// Fees are always negative costs; store as negative in Commission.
		if v > 0 {
			v = -v
		}
		tx.Commission += v
	}

	return tx, nil
}

func mapT212Type(s string) model.TxType {
	switch s {
	case "Market buy", "Limit buy":
		return model.TxBuy
	case "Market sell", "Limit sell":
		return model.TxSell
	case "Dividend", "Dividend (Dividend)", "Dividend (Ordinary)":
		return model.TxDividend
	case "Deposit":
		return model.TxDeposit
	case "Withdrawal":
		return model.TxWithdrawal
	case "Interest on cash":
		return model.TxDividend
	case "ADR Fee":
		return model.TxFee
	default:
		return model.TxUnknown
	}
}
