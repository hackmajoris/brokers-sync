package ledger

import (
	"math"
	"sort"

	"brokers-sync/internal/model"
)

// maxTransferMatchDays bounds how far apart a TRANSFER_OUT and its matching
// TRANSFER_IN may be dated. Trade date vs settle date bookkeeping differences
// between brokers can put them several days apart (observed: 3 days).
const maxTransferMatchDays = 14

// ReconcileTransfers borrows the receiving broker's recorded arrival value for
// each TRANSFER_OUT row that has a matching TRANSFER_IN (same symbol and
// quantity, closest date within maxTransferMatchDays). Only IBKR's transfer
// report states a real monetary amount for an ACATS/FOP transfer — sending
// brokers either report nothing (Revolut: $0) or their own stated total, which
// can drift from what the position was actually marked at on arrival. Using
// the receiving side's figure for both ends of the same transfer makes
// transfer-out and transfer-in agree, instead of comparing two different
// valuations of the same shares.
func ReconcileTransfers(txs []model.Transaction) []model.Transaction {
	out := make([]model.Transaction, len(txs))
	copy(out, txs)

	type inCandidate struct {
		idx  int
		used bool
	}
	insBySymbol := make(map[string][]inCandidate)
	for i, tx := range out {
		if tx.Type == model.TxTransferIn {
			insBySymbol[tx.Symbol] = append(insBySymbol[tx.Symbol], inCandidate{idx: i})
		}
	}
	for _, cands := range insBySymbol {
		sort.Slice(cands, func(a, b int) bool { return out[cands[a].idx].Date.Before(out[cands[b].idx].Date) })
	}

	for i := range out {
		tx := &out[i]
		if tx.Type != model.TxTransferOut {
			continue
		}
		cands := insBySymbol[tx.Symbol]
		best := -1
		bestDiff := math.Inf(1)
		for ci, c := range cands {
			if c.used {
				continue
			}
			in := out[c.idx]
			if math.Abs(in.Quantity-tx.Quantity) > 1e-6 {
				continue
			}
			diffDays := math.Abs(in.Date.Sub(tx.Date).Hours() / 24)
			if diffDays > maxTransferMatchDays {
				continue
			}
			if diffDays < bestDiff {
				bestDiff = diffDays
				best = ci
			}
		}
		if best >= 0 {
			cands[best].used = true
			in := out[cands[best].idx]
			tx.Net = in.Net
			tx.Currency = in.Currency
		}
	}

	return out
}
