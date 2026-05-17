package parser

import (
	"strings"
	"testing"

	"brokers-sync/internal/model"
)

// The report stacks two header rows, then data rows. This fixture mixes every
// Type: a kept external IN (ACATS), a kept external OUT, a dropped INTERCOMPANY
// (own-account move), and a dropped INTERNAL cash-adjustment row.
const ibkrTransfersFixture = `"ClientAccountID","Symbol","Description","AssetClass","TradeDate","SettleDateTarget","TransactionType","Direction","Delivered/Received","BrokerName","BrokerAccount","Quantity","TradePrice","TradeMoney","Proceeds","IBCommission","Taxes","NetTradeMoney","NetTradeMoneyInBase","NetTradePrice","NetCash","FifoPnlRealized","MtmPnl","CostBasis","ClosePrice"
"ClientAccountID","Symbol","Description","AssetClass","Date","DateTime","SettleDate","Direction","TransferCompany","TransferAccount","TransferAccountName","DeliveringBroker","Quantity","TransferPrice","PositionAmount","PositionAmountInBase","PnlAmount","PnlAmountInBase","CashTransfer","Code","Type"
"U10842564","AAPL","APPLE INC","STK","20230424","20230424","20230426","IN","--","REVO001REPD000711","","2402","12","0","1980.24","1980.24","0","0","0","","ACATS"
"U15842564","ZIM","ZIM INTEGRATED SHIPPING SERV","STK","20240830","20240830;202600","20240902","OUT","--","U15842564","","","-559","0","-10397.4","-10397.4","0","0","0","","INTERCOMPANY"
"U15842564","MSFT","MICROSOFT CORP","STK","20260101","20260101","20260101","OUT","--","OTHER","","NA","-3","0","-1200","-1200","0","0","0","","FOP"
"U10842564","--","ADJUSTMENT: CASH RECEIPT / DISBURSEMENT / TRANSFER","CASH","20250128","20250128;202600","20250129","-","--","-","-","","0","0","0","0","0","0","-5.28","","INTERNAL"
`

func TestParseIBKRTransfersFiltersInternalAndIntercompany(t *testing.T) {
	txs, err := ParseIBKRTransfers(strings.NewReader(ibkrTransfersFixture))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Only the ACATS IN and the FOP OUT survive; INTERCOMPANY and INTERNAL are
	// intra-portfolio plumbing the user asked to ignore.
	if len(txs) != 2 {
		t.Fatalf("got %d transactions, want 2 (ACATS + FOP); INTERNAL/INTERCOMPANY must be dropped", len(txs))
	}
	for _, tx := range txs {
		if strings.Contains(tx.Notes, "ADJUSTMENT") || tx.Symbol == "ZIM" {
			t.Errorf("kept a row that should be dropped: %+v", tx)
		}
	}

	in, out := txs[0], txs[1]

	// ACATS IN establishes a position at its carried cost basis (PositionAmount).
	if in.Type != model.TxTransferIn {
		t.Errorf("IN type: got %s, want TRANSFER_IN", in.Type)
	}
	if in.Symbol != "AAPL" || in.Quantity != 12 || in.Net != 1980.24 || in.Currency != "USD" {
		t.Errorf("IN row wrong: %+v", in)
	}

	// FOP OUT removes shares; the negative source quantity/amount become positive.
	if out.Type != model.TxTransferOut {
		t.Errorf("OUT type: got %s, want TRANSFER_OUT", out.Type)
	}
	if out.Symbol != "MSFT" || out.Quantity != 3 {
		t.Errorf("OUT row wrong: %+v", out)
	}

	if in.Broker != "ibkr" || out.Broker != "ibkr" {
		t.Errorf("Broker must be ibkr so rows merge into the IBKR ledger")
	}
}
