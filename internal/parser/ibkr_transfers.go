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

// IBKR "Transferred In/Out Positions" reports are separate from the Transaction
// History: the share transfers they record (ACATS in from another broker, FOP,
// intercompany moves between the owner's own IBKR entities) never appear in the
// Transaction History section, so without them transferred-in positions are
// missing from the ledger entirely.
//
// The file stacks one or more header pairs (a 25-column trade-style header and a
// 21-column transfer header) followed by data rows that follow the 21-column
// transfer schema:
//
//	0 ClientAccountID  1 Symbol  2 Description  3 AssetClass  4 Date
//	5 DateTime  6 SettleDate  7 Direction  8 TransferCompany  9 TransferAccount
//	10 TransferAccountName  11 DeliveringBroker  12 Quantity  13 TransferPrice
//	14 PositionAmount  15 PositionAmountInBase  16 PnlAmount  17 PnlAmountInBase
//	18 CashTransfer  19 Code  20 Type
//
// We keep external transfers (ACATS, FOP, ...) and drop INTERNAL (cash
// adjustment rows) and INTERCOMPANY (moves between the owner's own IBKR
// accounts) — those are net-zero intra-portfolio plumbing.
const ibkrTransferCols = 21

// Type values that represent intra-portfolio plumbing rather than a real
// external transfer. Compared case-insensitively.
var ibkrTransferSkipTypes = map[string]bool{
	"INTERNAL":     true,
	"INTERCOMPANY": true,
}

func ParseIBKRTransfers(r io.Reader) ([]model.Transaction, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1 // header pairs have different column counts

	var txs []model.Transaction
	lineNum := 0

	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("ibkr-transfers: line %d: %w", lineNum, err)
		}
		lineNum++

		// Header rows repeat throughout the file; data rows always carry the
		// 21-column transfer schema.
		if len(record) != ibkrTransferCols || strings.TrimSpace(record[0]) == "ClientAccountID" {
			continue
		}

		typ := strings.ToUpper(strings.TrimSpace(record[20]))
		if ibkrTransferSkipTypes[typ] {
			continue
		}

		tx, ok, err := parseIBKRTransferRow(record)
		if err != nil {
			return nil, fmt.Errorf("ibkr-transfers: line %d: %w", lineNum, err)
		}
		if !ok {
			continue
		}
		tx.Broker = "ibkr"
		tx.ID = syntheticID(tx)
		txs = append(txs, tx)
	}
	return txs, nil
}

// parseIBKRTransferRow maps one 21-column data row. ok is false for rows without
// a symbol/quantity (e.g. cash adjustments) or an unrecognised direction.
func parseIBKRTransferRow(r []string) (model.Transaction, bool, error) {
	var tx model.Transaction
	var err error

	sym := strings.TrimSpace(r[1])
	if sym == "" || sym == "-" || sym == "--" {
		return tx, false, nil
	}
	tx.Symbol = sym

	dateStr := strings.TrimSpace(r[4])
	tx.Date, err = time.Parse("20060102", dateStr)
	if err != nil {
		return tx, false, fmt.Errorf("date %q: %w", dateStr, err)
	}

	tx.Account = strings.TrimSpace(r[0])
	tx.Notes = strings.TrimSpace(r[2])
	// IBKR transfer reports omit an instrument-currency column. Every observed
	// external transfer is a USD-denominated US equity, and PositionAmount ==
	// PositionAmountInBase for those rows, so the lot is denominated in USD to
	// stay consistent with the same symbol's Transaction-History buy lots.
	tx.Currency = "USD"

	switch strings.ToUpper(strings.TrimSpace(r[7])) {
	case "IN":
		tx.Type = model.TxTransferIn
	case "OUT":
		tx.Type = model.TxTransferOut
	default:
		return tx, false, nil
	}

	qty, err := strconv.ParseFloat(strings.TrimSpace(r[12]), 64)
	if err != nil {
		return tx, false, fmt.Errorf("quantity %q: %w", r[12], err)
	}
	if qty < 0 {
		qty = -qty // OUT rows carry a negative quantity
	}
	if qty == 0 {
		return tx, false, nil
	}
	tx.Quantity = qty

	// PositionAmount is the carried cost basis (negative for OUT). transferIn
	// uses it as the lot cost basis; transferOut ignores it (removes FIFO).
	amt, err := strconv.ParseFloat(strings.TrimSpace(r[14]), 64)
	if err != nil {
		return tx, false, fmt.Errorf("position amount %q: %w", r[14], err)
	}
	if amt < 0 {
		amt = -amt
	}
	tx.Net = amt

	return tx, true, nil
}
