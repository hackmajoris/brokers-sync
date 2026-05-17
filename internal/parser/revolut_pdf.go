package parser

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"brokers-sync/internal/model"
)

// Revolut PDF account statements carry the COMPLETE transaction history, including
// the whole-share "Transfer" rows that the CSV export silently omits. Text is
// extracted with `pdftotext -layout`, which preserves the fixed columns:
//
//   Date                     SymbolType   Quantity  Price  SideValue  Fees  Commission
//   04 Mar 2020 19:20:51 GMT AAPL Trade - Market 0.296 US$298.72 Buy US$88.42 US$0 US$0
//
// The symbol may be glued to the type ("GOOGLTrade") and the price to the side
// ("US$2,689.86Buy"); splitting on the known type substring and matching amounts
// by their currency prefix handles both.

const revolutPDFDateLayout = "02 Jan 2006 15:04:05"

var (
	revolutPDFDate = regexp.MustCompile(`^(\d{2} \w{3} \d{4} \d{2}:\d{2}:\d{2}) GMT\s*(.*)$`)
	revolutPDFAmt  = regexp.MustCompile(`(-?)(?:US\$|€)([\d,]+(?:\.\d+)?)`)
	revolutPDFNum  = regexp.MustCompile(`-?\d[\d,]*(?:\.\d+)?`)
	// Most specific first: the long transfer string and "Trade - Market" must be
	// tested before their prefixes "Transfer" / "Trade - Limit".
	revolutPDFTypes = []string{
		"Trade - Market", "Trade - Limit", "Stock split", "Dividend",
		"Custody fee", "Cash top-up", "Cash withdrawal",
		"Transfer from Revolut Trading Ltd to Revolut Securities Europe UAB", "Transfer",
	}
)

// pdftotextLayout extracts the statement text. Kept as a var so tests can stub it.
var pdftotextLayout = func(path string) (string, error) {
	cmd := exec.Command("pdftotext", "-layout", path, "-")
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext (%s): %w: %s", path, err, strings.TrimSpace(stderr.String()))
	}
	return out.String(), nil
}

// ParseRevolutPDF extracts transactions from a Revolut PDF account statement.
func ParseRevolutPDF(path string) ([]model.Transaction, error) {
	text, err := pdftotextLayout(path)
	if err != nil {
		return nil, err
	}
	return parseRevolutPDFText(text)
}

func parseRevolutPDFText(text string) ([]model.Transaction, error) {
	var txs []model.Transaction
	currency := "USD"
	for line := range strings.SplitSeq(text, "\n") {
		// Statements group rows under a per-currency header ("USD Transactions").
		if strings.HasPrefix(strings.TrimSpace(line), "EUR Transactions") {
			currency = "EUR"
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "USD Transactions") {
			currency = "USD"
			continue
		}

		m := revolutPDFDate.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		tx, ok, err := parseRevolutPDFRow(m[1], m[2], currency)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		tx.Broker = "revolut"
		tx.ID = syntheticID(tx)
		txs = append(txs, tx)
	}
	return txs, nil
}

func parseRevolutPDFRow(dateStr, rest, currency string) (model.Transaction, bool, error) {
	var tx model.Transaction

	rawType, symbol, tail := "", "", ""
	for _, t := range revolutPDFTypes {
		if before, after, found := strings.Cut(rest, t); found {
			rawType = t
			symbol = strings.TrimSpace(before)
			tail = after
			break
		}
	}
	if rawType == "" {
		return tx, false, nil // header/footer/glossary line
	}

	t, err := parseAnyTime([]string{revolutPDFDateLayout}, dateStr)
	if err != nil {
		return tx, false, fmt.Errorf("revolut pdf: date %q: %w", dateStr, err)
	}
	tx.Date = t
	tx.Symbol = symbol
	tx.Currency = currency
	tx.Notes = rawType

	// Quantity is the bare number before the first currency amount; amounts follow.
	qtyPart, amtPart := tail, tail
	if loc := revolutPDFAmt.FindStringIndex(tail); loc != nil {
		qtyPart, amtPart = tail[:loc[0]], tail[loc[0]:]
	}
	qty := 0.0
	if s := revolutPDFNum.FindString(qtyPart); s != "" {
		qty = mustFloat(s)
	}
	var amounts []float64
	for _, a := range revolutPDFAmt.FindAllStringSubmatch(amtPart, -1) {
		v := mustFloat(a[2])
		if a[1] == "-" {
			v = -v
		}
		amounts = append(amounts, v)
	}
	value := 0.0
	if len(amounts) > 0 {
		value = amounts[0]
	}

	switch rawType {
	case "Trade - Market", "Trade - Limit":
		// amounts: [price, value, fees, commission]
		tx.Quantity = qty
		if len(amounts) >= 1 {
			tx.Price = amounts[0]
		}
		if len(amounts) >= 2 {
			tx.Net = amounts[1] // Revolut convention: positive value in account currency
		}
		if len(amounts) >= 4 && amounts[3] > 0 {
			tx.Commission = -amounts[3]
		}
		tx.Gross = tx.Net
		if strings.Contains(tail, "Sell") {
			tx.Type = model.TxSell
		} else {
			tx.Type = model.TxBuy
		}
	case "Dividend":
		tx.Type = model.TxDividend
		tx.Net, tx.Gross = value, value
	case "Custody fee":
		tx.Type = model.TxFee
		tx.Net, tx.Gross = value, value
	case "Cash top-up":
		tx.Type = model.TxDeposit
		tx.Net, tx.Gross = value, value
	case "Cash withdrawal":
		tx.Type = model.TxWithdrawal
		tx.Net, tx.Gross = value, value
	case "Stock split":
		tx.Type = model.TxStockSplit
		tx.Quantity = qty
	default: // both Transfer variants
		tx.Type = model.TxTransferOut
		if qty < 0 {
			qty = -qty
		}
		tx.Quantity = qty
	}

	return tx, true, nil
}

func mustFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
	return v
}
