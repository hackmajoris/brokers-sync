import rawMock from '../../mock.json'
import type { RawPortfolio, RawPeriod, RawPosition, RawBroker } from '../types/portfolio'
import { mapRawPortfolio } from './portfolioService'
import type { PortfolioData } from '../types/portfolio'

// One random scale per session so numbers look different each time but stay consistent within the session
const SCALE = 0.5 + Math.random() * 1.5

// Guard: some mock positions have null market_value/unrealized_pnl when price data is unavailable
const sc = (n: number) => (Number.isFinite(n) ? n * SCALE : n)

function scalePeriod(p: RawPeriod): RawPeriod {
  return {
    ...p,
    realized_pnl: p.realized_pnl * SCALE,
    dividends_net: p.dividends_net * SCALE,
    tax_withheld: p.tax_withheld * SCALE,
    fees: p.fees * SCALE,
    commissions: p.commissions * SCALE,
    deposits: p.deposits * SCALE,
    withdrawals: p.withdrawals * SCALE,
    transfer_in: (p.transfer_in ?? 0) * SCALE,
    transfer_out: (p.transfer_out ?? 0) * SCALE,
    buy_volume: p.buy_volume * SCALE,
    sell_volume: p.sell_volume * SCALE,
  }
}

function scalePosition(p: RawPosition): RawPosition {
  return {
    ...p,
    avg_cost: sc(p.avg_cost),
    total_cost: sc(p.total_cost),
    current_price: sc(p.current_price),
    market_value: sc(p.market_value),
    unrealized_pnl: sc(p.unrealized_pnl),
  }
}

function scaleBroker(b: RawBroker): RawBroker {
  return {
    ...b,
    cash_balance: b.cash_balance * SCALE,
    open_positions: b.open_positions.map(scalePosition),
    realized_pnl_by_symbol: b.realized_pnl_by_symbol.map(r => ({ ...r, pnl: r.pnl * SCALE })),
    all_time: scalePeriod(b.all_time),
    ytd: scalePeriod(b.ytd),
    mtd: scalePeriod(b.mtd),
    by_year: b.by_year.map(scalePeriod),
    dividends_by_symbol: b.dividends_by_symbol.map(d => ({
      ...d,
      gross: d.gross * SCALE,
      tax_withheld: d.tax_withheld * SCALE,
      net: d.net * SCALE,
    })),
  }
}

export function loadMockPortfolio(): PortfolioData {
  const mock = rawMock as RawPortfolio
  const scaled: RawPortfolio = {
    ...mock,
    cash_balance: mock.cash_balance * SCALE,
    brokers: mock.brokers.map(scaleBroker),
    open_positions: mock.open_positions.map(scalePosition),
    realized_pnl_by_symbol: mock.realized_pnl_by_symbol.map(r => ({ ...r, pnl: r.pnl * SCALE })),
    all_time: scalePeriod(mock.all_time),
    ytd: scalePeriod(mock.ytd),
    mtd: scalePeriod(mock.mtd),
    by_year: mock.by_year.map(scalePeriod),
    dividends_by_symbol: mock.dividends_by_symbol.map(d => ({
      ...d,
      gross: d.gross * SCALE,
      tax_withheld: d.tax_withheld * SCALE,
      net: d.net * SCALE,
    })),
  }
  return mapRawPortfolio(scaled)
}
