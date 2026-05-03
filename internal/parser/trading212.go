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

// Trading 212 CSV columns:
// Action, Time, ISIN, Ticker, Name, Notes, ID, No. of shares,
// Price / share, Currency (Price / share), Exchange rate,
// Result, Currency (Result), Total, Currency (Total)

var t212DateFormats = []string{
	"2006-01-02 15:04:05",
	"2006-01-02 15:04:05.999999",
	time.RFC3339,
}

func ParseTrading212(r io.Reader) ([]model.Transaction, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("trading212: read header: %w", err)
	}
	idx, err := mapColumns(header, []string{
		"Action", "Time", "ISIN", "Ticker", "Name",
		"No. of shares", "Price / share", "Currency (Price / share)",
		"Exchange rate", "Result", "Total", "Currency (Total)",
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

	if s := strings.TrimSpace(r[idx["Total"]]); s != "" {
		tx.Net, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("total %q: %w", s, err)
		}
		tx.Gross = tx.Net
	}

	// Result is the realized P&L on sells — store in notes for now,
	// the ledger recomputes this from lots.
	if s := strings.TrimSpace(r[idx["Result"]]); s != "" {
		tx.Notes = rawAction + " result=" + s
	}

	return tx, nil
}

func mapT212Type(s string) model.TxType {
	switch s {
	case "Market buy", "Limit buy":
		return model.TxBuy
	case "Market sell", "Limit sell":
		return model.TxSell
	case "Dividend":
		return model.TxDividend
	case "Deposit":
		return model.TxDeposit
	case "Withdrawal":
		return model.TxWithdrawal
	default:
		return model.TxUnknown
	}
}
