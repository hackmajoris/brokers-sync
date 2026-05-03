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

// IBKR CSV is multi-section. Each row begins with a section name and a row type
// ("Header" or "Data"). We only care about "Transaction History,Data,..." rows.
//
// Transaction History columns (after the two prefix fields):
// Date, Account, Description, Transaction Type, Symbol, Quantity, Price,
// Price Currency, Gross Amount, Commission, Net Amount

const ibkrSection = "Transaction History"

func ParseIBKR(r io.Reader) ([]model.Transaction, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1 // rows have variable column count

	var colIdx map[string]int
	var txs []model.Transaction
	lineNum := 0

	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("ibkr: line %d: %w", lineNum, err)
		}
		lineNum++

		if len(record) < 2 {
			continue
		}
		section := strings.TrimSpace(record[0])
		rowType := strings.TrimSpace(record[1])

		if section != ibkrSection {
			continue
		}

		data := record[2:]

		if rowType == "Header" {
			colIdx, err = mapColumns(data, []string{
				"Date", "Account", "Description", "Transaction Type",
				"Symbol", "Quantity", "Price", "Price Currency",
				"Gross Amount", "Commission", "Net Amount",
			})
			if err != nil {
				return nil, fmt.Errorf("ibkr: header: %w", err)
			}
			continue
		}

		if rowType != "Data" || colIdx == nil {
			continue
		}

		tx, err := parseIBKRRow(data, colIdx)
		if err != nil {
			return nil, fmt.Errorf("ibkr: line %d: %w", lineNum, err)
		}
		if tx.Type == model.TxForex {
			// Forex rounding rows are noise — skip tiny amounts
			if tx.Net > -0.01 && tx.Net < 0.01 {
				continue
			}
		}
		tx.Broker = "ibkr"
		tx.ID = syntheticID(tx)
		txs = append(txs, tx)
	}
	return txs, nil
}

func parseIBKRRow(r []string, idx map[string]int) (model.Transaction, error) {
	var tx model.Transaction
	var err error

	dateStr := strings.TrimSpace(r[idx["Date"]])
	tx.Date, err = time.Parse("2006-01-02", dateStr)
	if err != nil {
		return tx, fmt.Errorf("date %q: %w", dateStr, err)
	}

	tx.Account = strings.TrimSpace(r[idx["Account"]])
	tx.Notes = strings.TrimSpace(r[idx["Description"]])
	tx.Currency = strings.TrimSpace(r[idx["Price Currency"]])

	rawType := strings.TrimSpace(r[idx["Transaction Type"]])
	tx.Type = mapIBKRType(rawType)

	sym := strings.TrimSpace(r[idx["Symbol"]])
	if sym != "-" {
		tx.Symbol = sym
	}

	if s := strings.TrimSpace(r[idx["Quantity"]]); s != "" && s != "-" {
		tx.Quantity, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("quantity %q: %w", s, err)
		}
		// IBKR uses negative quantity for sells
		if tx.Quantity < 0 {
			tx.Quantity = -tx.Quantity
		}
	}

	if s := strings.TrimSpace(r[idx["Price"]]); s != "" && s != "-" {
		tx.Price, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("price %q: %w", s, err)
		}
	}

	if s := strings.TrimSpace(r[idx["Gross Amount"]]); s != "" && s != "-" {
		tx.Gross, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("gross %q: %w", s, err)
		}
	}

	if s := strings.TrimSpace(r[idx["Commission"]]); s != "" && s != "-" {
		tx.Commission, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("commission %q: %w", s, err)
		}
	}

	if s := strings.TrimSpace(r[idx["Net Amount"]]); s != "" && s != "-" {
		tx.Net, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return tx, fmt.Errorf("net %q: %w", s, err)
		}
	}

	return tx, nil
}

func mapIBKRType(s string) model.TxType {
	switch s {
	case "Buy":
		return model.TxBuy
	case "Sell":
		return model.TxSell
	case "Dividend":
		return model.TxDividend
	case "Foreign Tax Withholding":
		return model.TxTaxWithholding
	case "Deposit":
		return model.TxDeposit
	case "Withdrawal":
		return model.TxWithdrawal
	case "Forex Trade Component":
		return model.TxForex
	default:
		return model.TxUnknown
	}
}
