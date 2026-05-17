package parser

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"brokers-sync/internal/model"
)

// detectXlsx opens an xlsx file and returns BrokerXTB if it contains XTB-specific sheets,
// or an error if the format is not recognised.
func detectXlsx(path string) (string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return "", fmt.Errorf("detect %s: %w", filepath.Base(path), err)
	}
	defer func() { _ = f.Close() }()

	sheets := make(map[string]bool, len(f.GetSheetList()))
	for _, s := range f.GetSheetList() {
		sheets[s] = true
	}
	if sheets["Cash Operations"] && sheets["Closed Positions"] {
		return BrokerXTB, nil
	}
	return "", fmt.Errorf("unrecognised xlsx format in %s", filepath.Base(path))
}

// ParseXTB reads an XTB xlsx export (Cash Operations sheet) and returns transactions.
func ParseXTB(path string) ([]model.Transaction, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("xtb: open: %w", err)
	}
	defer func() { _ = f.Close() }()

	account := xtbAccount(f)

	rows, err := f.GetRows("Cash Operations")
	if err != nil {
		return nil, fmt.Errorf("xtb: sheet 'Cash Operations' not found: %w", err)
	}

	// Find header row (first row whose first cell is "Type")
	headerIdx := -1
	for i, row := range rows {
		if len(row) > 0 && row[0] == "Type" {
			headerIdx = i
			break
		}
	}
	if headerIdx == -1 {
		return nil, fmt.Errorf("xtb: cannot find header row in Cash Operations sheet")
	}

	idx, err := mapColumns(rows[headerIdx], []string{"Type", "Ticker", "Instrument", "Time", "Amount", "ID", "Comment"})
	if err != nil {
		return nil, fmt.Errorf("xtb: %w", err)
	}

	var txs []model.Transaction
	for lineNum, row := range rows[headerIdx+1:] {
		if len(row) == 0 || strings.TrimSpace(row[0]) == "" || strings.TrimSpace(row[0]) == "Total" {
			continue
		}
		tx, xtbID, skip, err := parseXTBCashRow(row, idx, account)
		if err != nil {
			return nil, fmt.Errorf("xtb: row %d: %w", headerIdx+2+lineNum, err)
		}
		if skip {
			continue
		}
		tx.Broker = BrokerXTB
		// XTB provides a unique numeric ID per cash operation — use it directly
		// to avoid collisions between same-second same-price trades (e.g. split orders).
		if xtbID != "" {
			tx.ID = "xtb|" + xtbID
		} else {
			tx.ID = syntheticID(tx)
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

// xtbAccount extracts the account number from the first metadata row.
func xtbAccount(f *excelize.File) string {
	rows, err := f.GetRows("Cash Operations")
	if err != nil || len(rows) == 0 {
		return ""
	}
	if len(rows[0]) >= 2 {
		return strings.TrimSpace(rows[0][1])
	}
	return ""
}

func parseXTBCashRow(r []string, idx map[string]int, account string) (model.Transaction, string, bool, error) {
	var tx model.Transaction
	tx.Account = account
	tx.Currency = "EUR"

	get := func(col string) string {
		i, ok := idx[col]
		if !ok || i >= len(r) {
			return ""
		}
		return strings.TrimSpace(r[i])
	}

	rawType := get("Type")
	switch rawType {
	case "Stock purchase":
		tx.Type = model.TxBuy
	case "Stock sell":
		tx.Type = model.TxSell
	case "Deposit":
		tx.Type = model.TxDeposit
	case "Withdrawal":
		tx.Type = model.TxWithdrawal
	case "Free funds interest":
		tx.Type = model.TxDividend
	case "RO tax":
		tx.Type = model.TxTaxWithholding
	default:
		return tx, "", true, nil
	}

	dateStr := get("Time")
	var err error
	tx.Date, err = parseAnyTime([]string{"2006-01-02 15:04:05"}, dateStr)
	if err != nil {
		return tx, "", false, fmt.Errorf("date %q: %w", dateStr, err)
	}

	tx.Symbol = get("Ticker")
	tx.Name = get("Instrument")

	amountStr := get("Amount")
	if amountStr != "" {
		tx.Net, err = strconv.ParseFloat(amountStr, 64)
		if err != nil {
			return tx, "", false, fmt.Errorf("amount %q: %w", amountStr, err)
		}
	}
	tx.Gross = tx.Net

	comment := get("Comment")
	tx.Notes = comment

	if tx.Type == model.TxBuy || tx.Type == model.TxSell {
		tx.Quantity, tx.Price = parseXTBComment(comment)
	}

	return tx, get("ID"), false, nil
}

// parseXTBComment extracts qty and price from comments like:
//
//	"OPEN BUY 1 @ 90.1920"
//	"CLOSE BUY 2/5 @ 114.2000"
func parseXTBComment(comment string) (qty, price float64) {
	atIdx := strings.Index(comment, "@")
	if atIdx == -1 {
		return
	}
	price, _ = strconv.ParseFloat(strings.TrimSpace(comment[atIdx+1:]), 64)

	before := strings.Fields(comment[:atIdx])
	if len(before) == 0 {
		return
	}
	qtyStr := before[len(before)-1]
	if slash := strings.IndexByte(qtyStr, '/'); slash != -1 {
		qtyStr = qtyStr[:slash]
	}
	qty, _ = strconv.ParseFloat(qtyStr, 64)
	return
}
