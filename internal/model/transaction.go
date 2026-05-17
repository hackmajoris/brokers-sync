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
	TxInterest       TxType = "INTEREST" // credit/debit interest — a cash movement, not investment income
	TxStockSplit     TxType = "STOCK_SPLIT"
	TxForex          TxType = "FOREX"
	TxTransferOut    TxType = "TRANSFER_OUT" // shares moved out to another broker (no P&L)
	TxTransferIn     TxType = "TRANSFER_IN"  // shares moved in from another broker; establishes a lot at carried cost basis (no cash flow)
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
	BrokerPnL  float64 // broker-reported realized P&L for sells (0 = not provided)
	Liquidate  bool    // sell closes the ENTIRE remaining position (delisting write-off; stated Quantity unreliable)
	Notes      string
}
