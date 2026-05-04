package parser

import (
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"brokers-sync/internal/model"
)

// Tradeville exports a tab-separated CSV with a leading "SEP=\t" hint line.
// Columns used: id, data, op, simbol, cant, pret, comis, suma, valuta
//
// op mapping:
//   cump  → BUY
//   vanz  → SELL
//   in    → DEPOSIT
//   out   → WITHDRAWAL
//   div   → DIVIDEND
//   comis → FEE

var tradevilleOps = map[string]model.TxType{
	"cump":  model.TxBuy,
	"vanz":  model.TxSell,
	"in":    model.TxDeposit,
	"out":   model.TxWithdrawal,
	"div":   model.TxDividend,
	"comis": model.TxFee,
}

var currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)

var tradevilleDateFormats = []string{
	"2006-01-02 15:04:05.000",
	"2006-01-02 15:04:05",
}

func ParseTradeville(r io.Reader) ([]model.Transaction, error) {
	cr := csv.NewReader(r)
	cr.Comma = '\t'
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	cr.ReuseRecord = false

	// First line is either "SEP=\t" or the real header; skip it if it's the hint.
	first, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("tradeville: read first line: %w", err)
	}
	if strings.HasPrefix(strings.TrimRight(first[0], "\r"), "SEP=") {
		first, err = cr.Read()
		if err != nil {
			return nil, fmt.Errorf("tradeville: read header: %w", err)
		}
	}

	idx, err := mapColumns(first, []string{"id", "data", "op", "simbol", "cant", "pret", "comis", "suma", "valuta"})
	if err != nil {
		return nil, fmt.Errorf("tradeville: %w", err)
	}

	var txs []model.Transaction
	lineNum := 1
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tradeville: line %d: %w", lineNum, err)
		}
		lineNum++

		tx, err := parseTradevilleRow(record, idx)
		if err != nil {
			return nil, fmt.Errorf("tradeville: line %d: %w", lineNum, err)
		}
		tx.Broker = "tradeville"
		tx.ID = syntheticID(tx)
		txs = append(txs, tx)
	}
	return txs, nil
}

func parseTradevilleRow(r []string, idx map[string]int) (model.Transaction, error) {
	var tx model.Transaction
	var err error

	dateStr := strings.TrimSpace(r[idx["data"]])
	tx.Date, err = parseAnyTime(tradevilleDateFormats, dateStr)
	if err != nil {
		return tx, fmt.Errorf("date %q: %w", dateStr, err)
	}

	op := strings.TrimSpace(r[idx["op"]])
	txType, ok := tradevilleOps[op]
	if !ok {
		txType = model.TxUnknown
	}
	tx.Type = txType
	tx.Notes = op

	tx.Symbol = strings.TrimSpace(r[idx["simbol"]])
	if cur := strings.TrimSpace(r[idx["valuta"]]); currencyRe.MatchString(cur) {
		tx.Currency = cur
	} else {
		tx.Currency = "RON" // default for Tradeville; field misaligned due to HTML in last column
	}

	if s := strings.TrimSpace(r[idx["cant"]]); s != "" {
		tx.Quantity, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("cant %q: %w", s, err)
		}
	}

	if s := strings.TrimSpace(r[idx["pret"]]); s != "" {
		tx.Price, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("pret %q: %w", s, err)
		}
	}

	if s := strings.TrimSpace(r[idx["comis"]]); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("comis %q: %w", s, err)
		}
		if v > 0 {
			v = -v
		}
		tx.Commission = v
	}

	// "suma" is the net cash effect (negative for buys, positive for sells/deposits).
	if s := strings.TrimSpace(r[idx["suma"]]); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("suma %q: %w", s, err)
		}
		tx.Net = v
		tx.Gross = v - tx.Commission
	}

	return tx, nil
}
