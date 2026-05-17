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
	"2006-01-02 15:04:05Z07:00",        // newer exports: space separator + UTC offset
	"2006-01-02 15:04:05.999999Z07:00", // same, with fractional seconds
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
	// Newer exports label the timestamp column "Time (UTC)" instead of "Time".
	for i, h := range header {
		if strings.TrimSpace(h) == "Time (UTC)" {
			header[i] = "Time"
		}
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

		// Currency conversion rows move cash between currency sub-accounts at the
		// broker's historical rate; emit them as separate forex legs so per-currency
		// cash stays accurate (a single Transaction can hold only one currency).
		if strings.TrimSpace(record[idx["Action"]]) == "Currency conversion" {
			legs, err := parseT212Conversion(record, idx)
			if err != nil {
				return nil, fmt.Errorf("trading212: line %d: %w", lineNum, err)
			}
			for _, tx := range legs {
				tx.Broker = "trading212"
				tx.ID = syntheticID(tx)
				txs = append(txs, tx)
			}
			continue
		}

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

// parseT212Conversion turns one "Currency conversion" row into per-currency cash
// legs: the debited from-amount, the credited to-amount, and the fee. Each leg is
// a TxForex so the ledger and cash-balance logic move cash between currencies
// without treating it as external capital. Legs with no amount/currency are omitted.
func parseT212Conversion(r []string, idx map[string]int) ([]model.Transaction, error) {
	date, err := parseAnyTime(t212DateFormats, strings.TrimSpace(r[idx["Time"]]))
	if err != nil {
		return nil, fmt.Errorf("date %q: %w", strings.TrimSpace(r[idx["Time"]]), err)
	}

	col := func(name string) string {
		if i, ok := idx[name]; ok && i < len(r) {
			return strings.TrimSpace(r[i])
		}
		return ""
	}
	num := func(s string) float64 {
		v, _ := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
		return v
	}

	leg := func(amount float64, currency string) (model.Transaction, bool) {
		if currency == "" || amount == 0 {
			return model.Transaction{}, false
		}
		return model.Transaction{
			Date:     date,
			Type:     model.TxForex,
			Currency: currency,
			Net:      amount,
			Gross:    amount,
			Notes:    "Currency conversion",
		}, true
	}

	var legs []model.Transaction
	if tx, ok := leg(-num(col("Currency conversion from amount")), col("Currency (Currency conversion from amount)")); ok {
		legs = append(legs, tx)
	}
	if tx, ok := leg(num(col("Currency conversion to amount")), col("Currency (Currency conversion to amount)")); ok {
		legs = append(legs, tx)
	}
	// Total carries the conversion fee (negative) in Currency (Total).
	if tx, ok := leg(num(col("Total")), col("Currency (Total)")); ok {
		legs = append(legs, tx)
	}
	return legs, nil
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

	// A sell at a nominal price (e.g. 0.0001) is a delisting write-off: the broker
	// closes the whole position but the exported share count can be off (unrecorded
	// reverse splits), so flag it to liquidate all remaining shares in the ledger.
	if tx.Type == model.TxSell && tx.Price > 0 && tx.Price < 0.001 {
		tx.Liquidate = true
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
	// T212 uses many dividend labels: "Dividend", "Dividend (Dividend)",
	// "Dividend (Ordinary)", "Dividend (Dividends paid by us corporations)", …
	if strings.HasPrefix(s, "Dividend") {
		return model.TxDividend
	}
	switch s {
	case "Market buy", "Limit buy":
		return model.TxBuy
	case "Market sell", "Limit sell":
		return model.TxSell
	case "Deposit":
		return model.TxDeposit
	case "Withdrawal":
		return model.TxWithdrawal
	case "Interest on cash":
		return model.TxDividend
	case "ADR Fee":
		return model.TxFee
	case "Transfer out":
		return model.TxTransferOut
	default:
		return model.TxUnknown
	}
}
