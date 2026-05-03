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

// Revolut CSV columns:
// Date, Ticker, Type, Quantity, Price per share, Total Amount, Currency, FX Rate

var revolutDateFormats = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999999Z",
	"2006-01-02T15:04:05Z",
}

func ParseRevolut(r io.Reader) ([]model.Transaction, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("revolut: read header: %w", err)
	}
	idx, err := mapColumns(header, []string{"Date", "Ticker", "Type", "Quantity", "Price per share", "Total Amount", "Currency", "FX Rate"})
	if err != nil {
		return nil, fmt.Errorf("revolut: %w", err)
	}

	var txs []model.Transaction
	lineNum := 1
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("revolut: line %d: %w", lineNum, err)
		}
		lineNum++

		tx, err := parseRevolutRow(record, idx)
		if err != nil {
			return nil, fmt.Errorf("revolut: line %d: %w", lineNum, err)
		}
		tx.Broker = "revolut"
		tx.ID = syntheticID(tx)
		txs = append(txs, tx)
	}
	return txs, nil
}

func parseRevolutRow(r []string, idx map[string]int) (model.Transaction, error) {
	var tx model.Transaction

	dateStr := r[idx["Date"]]
	t, err := parseAnyTime(revolutDateFormats, dateStr)
	if err != nil {
		return tx, fmt.Errorf("date %q: %w", dateStr, err)
	}
	tx.Date = t
	tx.Symbol = strings.TrimSpace(r[idx["Ticker"]])
	tx.Currency = strings.TrimSpace(r[idx["Currency"]])

	rawType := strings.TrimSpace(r[idx["Type"]])
	tx.Type = mapRevolutType(rawType)
	tx.Notes = rawType

	if s := strings.TrimSpace(r[idx["Quantity"]]); s != "" {
		tx.Quantity, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("quantity %q: %w", s, err)
		}
	}

	if s := strings.TrimSpace(r[idx["Price per share"]]); s != "" {
		tx.Price, err = parseRevolutAmount(s)
		if err != nil {
			return tx, fmt.Errorf("price %q: %w", s, err)
		}
	}

	if s := strings.TrimSpace(r[idx["Total Amount"]]); s != "" {
		tx.Net, err = parseRevolutAmount(s)
		if err != nil {
			return tx, fmt.Errorf("total amount %q: %w", s, err)
		}
		tx.Gross = tx.Net
	}

	if s := strings.TrimSpace(r[idx["FX Rate"]]); s != "" {
		tx.FXRate, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("fx rate %q: %w", s, err)
		}
	}

	return tx, nil
}

// parseRevolutAmount strips leading currency prefix like "USD " and parses the number.
func parseRevolutAmount(s string) (float64, error) {
	// Format: "USD 66.62" or "USD -0.04" or just a number
	parts := strings.SplitN(s, " ", 2)
	raw := s
	if len(parts) == 2 {
		raw = parts[1]
	}
	return strconv.ParseFloat(strings.TrimSpace(raw), 64)
}

func mapRevolutType(s string) model.TxType {
	switch s {
	case "BUY - MARKET", "BUY - LIMIT":
		return model.TxBuy
	case "SELL - MARKET", "SELL - LIMIT":
		return model.TxSell
	case "DIVIDEND":
		return model.TxDividend
	case "STOCK SPLIT":
		return model.TxStockSplit
	case "CUSTODY FEE", "CUSTODY FEE REVERSAL":
		return model.TxFee
	case "CASH TOP-UP":
		return model.TxDeposit
	case "CASH WITHDRAWAL":
		return model.TxWithdrawal
	default:
		return model.TxUnknown
	}
}
