package model

import "time"

type TxType string

const (
	TxBuy            TxType = "BUY"
	TxSell           TxType = "SELL"
	TxDividend       TxType = "DIVIDEND"
	TxTaxWithholding TxType = "TAX_WITHHOLDING"
	TxDeposit        TxType = "DEPOSIT"
	TxWithdrawal     TxType = "WITHDRAWAL"
	TxFee            TxType = "FEE"
	TxStockSplit     TxType = "STOCK_SPLIT"
	TxForex          TxType = "FOREX"
	TxUnknown        TxType = "UNKNOWN"
)

type Transaction struct {
	ID         string // synthetic: broker+date+symbol+type hash
	Date       time.Time
	Broker     string // "revolut" | "ibkr" | "trading212"
	Account    string
	Type       TxType
	Symbol     string
	ISIN       string
	Name       string
	Quantity   float64
	Price      float64
	Currency   string
	Gross      float64 // pre-commission amount (positive = inflow)
	Commission float64 // always <= 0
	Net        float64 // Gross + Commission
	FXRate     float64 // price currency → base currency (0 means unknown)
	Notes      string
}
