package parser

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	sellPositions := map[int]string{} // index in txs → XTB Position ID
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
		if i, ok := idx["Position ID"]; ok && tx.Type == model.TxSell && i < len(row) {
			sellPositions[len(txs)] = strings.TrimSpace(row[i])
		}
		txs = append(txs, tx)
	}

	windowStart, err := xtbWindowStart(rows)
	if err != nil {
		return nil, err
	}
	closed, err := xtbClosedPositions(f)
	if err != nil {
		return nil, err
	}
	open, err := xtbOpenPositions(f)
	if err != nil {
		return nil, err
	}

	applyXTBBrokerPnL(txs, sellPositions, closed)
	txs = append(txs, xtbPreWindowLots(closed, open, windowStart, account)...)
	return txs, nil
}

// xtbLot is one row of the Closed Positions or Open Positions sheet: a slice
// of an XTB position. Cost is the purchase value in account currency.
type xtbLot struct {
	positionID string
	symbol     string
	name       string
	volume     float64
	openTime   time.Time
	closeTime  time.Time // zero for open lots
	cost       float64
	grossPnL   float64 // closed lots only
	used       bool
}

const xtbTimeLayout = "2006-01-02 15:04:05"

// xtbWindowStart reads the "Date from (UTC)" metadata row of Cash Operations.
// Cash Operations only lists operations inside this window, while the
// position sheets also list lots opened before it.
func xtbWindowStart(rows [][]string) (time.Time, error) {
	for _, row := range rows {
		if len(row) >= 2 && strings.TrimSpace(row[0]) == "Date from (UTC)" {
			t, err := time.Parse(xtbTimeLayout, strings.TrimSpace(row[1]))
			if err != nil {
				return time.Time{}, fmt.Errorf("xtb: window start %q: %w", row[1], err)
			}
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("xtb: 'Date from (UTC)' row not found in Cash Operations sheet")
}

// xtbSheetRows returns the data rows of sheet below its header row and the
// header's column index. The header is the first row that has all required
// columns; Open Positions starts with a summary table whose header differs.
func xtbSheetRows(f *excelize.File, sheet string, required []string) ([][]string, map[string]int, error) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, nil, fmt.Errorf("xtb: sheet %q not found: %w", sheet, err)
	}
	for i, row := range rows {
		if idx, err := mapColumns(row, required); err == nil {
			return rows[i+1:], idx, nil
		}
	}
	return nil, nil, fmt.Errorf("xtb: cannot find header row in %s sheet", sheet)
}

func xtbClosedPositions(f *excelize.File) ([]xtbLot, error) {
	rows, idx, err := xtbSheetRows(f, "Closed Positions",
		[]string{"Instrument", "Ticker", "Volume", "Open Time (UTC)", "Close Time (UTC)", "Gross Profit", "Purchase Value", "Position ID"})
	if err != nil {
		return nil, err
	}
	var lots []xtbLot
	for _, row := range rows {
		get := cellGetter(row, idx)
		if get("Position ID") == "" {
			continue // footer "Profit/loss" totals row
		}
		var lot xtbLot
		var errs [5]error
		lot.positionID = get("Position ID")
		lot.symbol = get("Ticker")
		lot.name = get("Instrument")
		lot.volume, errs[0] = strconv.ParseFloat(get("Volume"), 64)
		lot.openTime, errs[1] = time.Parse(xtbTimeLayout, get("Open Time (UTC)"))
		lot.closeTime, errs[2] = time.Parse(xtbTimeLayout, get("Close Time (UTC)"))
		lot.cost, errs[3] = strconv.ParseFloat(get("Purchase Value"), 64)
		lot.grossPnL, errs[4] = strconv.ParseFloat(get("Gross Profit"), 64)
		for _, e := range errs {
			if e != nil {
				return nil, fmt.Errorf("xtb: Closed Positions position %s: %w", lot.positionID, e)
			}
		}
		lots = append(lots, lot)
	}
	return lots, nil
}

// xtbOpenPositions reads per-lot rows of the Open Positions sheet. Rows whose
// Type is empty are per-instrument aggregates and are skipped.
func xtbOpenPositions(f *excelize.File) ([]xtbLot, error) {
	rows, idx, err := xtbSheetRows(f, "Open Positions",
		[]string{"Instrument/Position", "Ticker", "Type", "Volume", "Value", "Open time (UTC)", "Gross Profit"})
	if err != nil {
		return nil, err
	}
	var lots []xtbLot
	for _, row := range rows {
		get := cellGetter(row, idx)
		if get("Type") == "" {
			continue
		}
		var lot xtbLot
		var value float64
		var errs [4]error
		lot.positionID = get("Instrument/Position")
		lot.symbol = get("Ticker")
		lot.volume, errs[0] = strconv.ParseFloat(get("Volume"), 64)
		lot.openTime, errs[1] = time.Parse(xtbTimeLayout, get("Open time (UTC)"))
		value, errs[2] = strconv.ParseFloat(get("Value"), 64)
		lot.grossPnL, errs[3] = strconv.ParseFloat(get("Gross Profit"), 64)
		for _, e := range errs {
			if e != nil {
				return nil, fmt.Errorf("xtb: Open Positions position %s: %w", lot.positionID, e)
			}
		}
		// The sheet has no purchase value column; Value and Gross Profit are
		// both in account currency, so their difference is the purchase cost.
		lot.cost = value - lot.grossPnL
		lot.grossPnL = 0
		lots = append(lots, lot)
	}
	return lots, nil
}

// applyXTBBrokerPnL sets each sell's BrokerPnL from its Closed Positions row,
// matched by Position ID and volume, preferring the nearest close time.
// Gross Profit is used because commissions arrive as separate fee rows.
func applyXTBBrokerPnL(txs []model.Transaction, sellPositions map[int]string, closed []xtbLot) {
	byPosition := map[string][]int{}
	for i, lot := range closed {
		byPosition[lot.positionID] = append(byPosition[lot.positionID], i)
	}
	for txIdx, posID := range sellPositions {
		tx := &txs[txIdx]
		best := -1
		var bestDiff time.Duration
		for _, ci := range byPosition[posID] {
			lot := closed[ci]
			if lot.used || math.Abs(lot.volume-tx.Quantity) > 1e-6 {
				continue
			}
			diff := lot.closeTime.Sub(tx.Date).Abs()
			if best == -1 || diff < bestDiff {
				best, bestDiff = ci, diff
			}
		}
		if best >= 0 {
			closed[best].used = true
			tx.BrokerPnL = closed[best].grossPnL
		}
	}
}

// xtbPreWindowLots returns a TRANSFER_IN per position opened before the export
// window. Their purchases are not in Cash Operations, so without these the
// ledger would sell them at zero cost basis. TRANSFER_IN establishes the lot
// without a cash outflow, since the purchase cash also predates the window.
func xtbPreWindowLots(closed, open []xtbLot, windowStart time.Time, account string) []model.Transaction {
	byPosition := map[string]*model.Transaction{}
	var order []string
	for _, lot := range append(closed, open...) {
		if !lot.openTime.Before(windowStart) {
			continue
		}
		tx, ok := byPosition[lot.positionID]
		if !ok {
			tx = &model.Transaction{
				ID:       "xtb|pos|" + lot.positionID,
				Date:     lot.openTime,
				Broker:   BrokerXTB,
				Account:  account,
				Type:     model.TxTransferIn,
				Symbol:   lot.symbol,
				Name:     lot.name,
				Currency: "EUR",
				Notes:    "XTB position " + lot.positionID + " opened before export window",
			}
			byPosition[lot.positionID] = tx
			order = append(order, lot.positionID)
		}
		tx.Quantity += lot.volume
		tx.Net += lot.cost
	}
	txs := make([]model.Transaction, 0, len(order))
	for _, id := range order {
		tx := byPosition[id]
		tx.Gross = tx.Net
		txs = append(txs, *tx)
	}
	return txs
}

func cellGetter(row []string, idx map[string]int) func(string) string {
	return func(col string) string {
		i, ok := idx[col]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
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
	case "Free funds interest", "Dividend":
		tx.Type = model.TxDividend
	case "Withholding tax":
		tx.Type = model.TxTaxWithholding
	// "RO tax" is Romanian tax on a sale's gain (its comment names the sold
	// ticker and sale date), not dividend withholding.
	case "Commission", "SEC fee", "RO tax":
		tx.Type = model.TxFee
	case "Transfer":
		tx.Type = model.TxForex
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
