package parser

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"brokers-sync/internal/model"
)

// Tradeville exports a tab-separated CSV with a leading "SEP=\t" hint line.
//
// Columns: Tip, Data, Simbol, Suma, Cantitate, Pret
//   Tip       full-word Romanian transaction type (see tradevilleOps)
//   Data      DD/MM/YYYY (no time component)
//   Simbol    ticker for trades/dividends; currency code for cash rows
//   Suma      signed net cash effect with inline currency, e.g. "-1,176.54 RON"
//   Cantitate share count for trades; cash amount for non-trades (thousands-grouped)
//   Pret      price per share for trades; "-" otherwise
//
// This export omits per-trade commission and dividend tax, so those stay 0.
// Schimb valutar / Transferuri interne are currency plumbing, not external cash
// flow, so they map to TxForex (ignored by the ledger and cash-flow stats).

var tradevilleOps = map[string]model.TxType{
	"cumparare":           model.TxBuy,
	"vanzare":             model.TxSell,
	"alimentare":          model.TxDeposit,
	"retragere":           model.TxWithdrawal,
	"dividend":            model.TxDividend,
	"comision":            model.TxFee,
	"schimb valutar":      model.TxForex,
	"transferuri interne": model.TxForex,
}

var tradevilleDateFormats = []string{"02/01/2006", "02.01.2006"}

func ParseTradeville(r io.Reader) ([]model.Transaction, error) {
	cr := csv.NewReader(r)
	cr.Comma = '\t'
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	cr.ReuseRecord = false

	// First line is either "SEP=\t" or the real header; skip it if it's the hint.
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("tradeville: read first line: %w", err)
	}
	if strings.HasPrefix(strings.TrimRight(header[0], "\r"), "SEP=") {
		header, err = cr.Read()
		if err != nil {
			return nil, fmt.Errorf("tradeville: read header: %w", err)
		}
	}

	idx, err := mapColumns(header, []string{"Tip", "Data", "Simbol", "Suma", "Cantitate", "Pret"})
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

	tip := strings.TrimSpace(r[idx["Tip"]])
	txType, ok := tradevilleOps[strings.ToLower(tip)]
	if !ok {
		txType = model.TxUnknown
	}
	tx.Type = txType
	tx.Notes = tip

	dateStr := strings.TrimSpace(r[idx["Data"]])
	t, err := parseAnyTime(tradevilleDateFormats, dateStr)
	if err != nil {
		return tx, fmt.Errorf("date %q: %w", dateStr, err)
	}
	tx.Date = t

	amount, currency, err := parseTradevilleAmount(r[idx["Suma"]])
	if err != nil {
		return tx, fmt.Errorf("suma %q: %w", r[idx["Suma"]], err)
	}
	tx.Net = amount
	tx.Gross = amount
	if currency != "" {
		tx.Currency = currency
	} else {
		tx.Currency = "RON"
	}

	simbol := strings.TrimSpace(r[idx["Simbol"]])
	qty, err := parseTradevilleNum(r[idx["Cantitate"]])
	if err != nil {
		return tx, fmt.Errorf("cantitate %q: %w", r[idx["Cantitate"]], err)
	}

	// "Transferuri interne" is overloaded: with a currency in Simbol it is an
	// internal cash move (kept as TxForex, ignored); with a stock ticker and
	// zero Suma it is a free-share distribution — a zero-cost BUY.
	if txType == model.TxForex && strings.EqualFold(tip, "transferuri interne") && !tradevilleCurrencies[simbol] {
		tx.Type = model.TxBuy
		tx.Symbol = simbol
		tx.Quantity = qty
		tx.Price = 0
		tx.Net = 0 // zero cost basis (ledger.buy falls back to qty×price = 0)
		tx.Gross = 0
		return tx, nil
	}

	// Symbol is a ticker only for trades and dividends; cash rows carry a
	// currency code in Simbol, which we ignore (currency comes from Suma).
	switch txType {
	case model.TxBuy, model.TxSell:
		tx.Symbol = simbol
		tx.Quantity = qty
		price, err := parseTradevilleNum(r[idx["Pret"]])
		if err != nil {
			return tx, fmt.Errorf("pret %q: %w", r[idx["Pret"]], err)
		}
		tx.Price = price
	case model.TxDividend:
		tx.Symbol = simbol
	}

	return tx, nil
}

// tradevilleCurrencies is the ISO code allowlist used to tell a currency in the
// Simbol column apart from a stock ticker (both are short uppercase strings).
var tradevilleCurrencies = map[string]bool{
	"RON": true, "EUR": true, "USD": true, "GBP": true, "CHF": true,
}

// parseTradevilleAmount parses a signed amount with an inline currency suffix and
// thousands separators, e.g. "-1,176.54 RON" → (-1176.54, "RON").
func parseTradevilleAmount(s string) (float64, string, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return 0, "", nil
	}
	var currency string
	if len(fields) >= 2 {
		currency = fields[len(fields)-1]
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(fields[0], ",", ""), 64)
	if err != nil {
		return 0, currency, err
	}
	return v, currency, nil
}

// parseTradevilleNum parses a thousands-grouped number, treating "" and "-" as 0.
func parseTradevilleNum(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0, nil
	}
	return strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
}
