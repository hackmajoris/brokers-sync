import type {
  RawPortfolio,
  RawPeriod,
  RawPosition,
  RawBroker,
  PortfolioData,
  BrokerData,
  Position,
  YearStat,
  PeriodSummary,
  DividendBySymbol,
} from '../types/portfolio'

function mapPeriod(p: RawPeriod): PeriodSummary {
  return {
    realizedPnl: p.realized_pnl,
    dividends: p.dividends_net,
    taxWithheld: p.tax_withheld,
    fees: p.fees,
    deposits: p.deposits,
    withdrawals: p.withdrawals,
    gainPct: p.gain_pct,
    buyVol: p.buy_volume,
    sellVol: p.sell_volume,
  }
}

function mapPosition(p: RawPosition): Position {
  return {
    symbol: p.symbol,
    mv: p.market_value,
    pnl: p.unrealized_pnl,
    pct: p.unrealized_pct_omitempty,
    cost: p.total_cost,
    quantity: p.quantity,
    currentPrice: p.current_price,
  }
}

function mapYearStat(p: RawPeriod): YearStat {
  return {
    label: p.label,
    gainPct: p.gain_pct,
    rPnl: p.realized_pnl,
    divs: p.dividends_net,
    deposits: p.deposits,
    buyVol: p.buy_volume,
    sellVol: p.sell_volume,
  }
}

function mapDividend(d: { symbol: string; gross: number; tax_withheld: number; net: number }): DividendBySymbol {
  return { symbol: d.symbol, gross: d.gross, taxWithheld: d.tax_withheld, net: d.net }
}

function mapBroker(b: RawBroker): BrokerData {
  return {
    name: b.name,
    currency: b.base_currency,
    cashBalance: b.cash_balance ?? 0,
    allTimeGain: b.all_time.gain_pct,
    allTimeRPnl: b.all_time.realized_pnl,
    ytdGain: b.ytd.gain_pct,
    ytdRPnl: b.ytd.realized_pnl,
    ytdDivs: b.ytd.dividends_net,
    mtdGain: b.mtd.gain_pct,
    mtdRPnl: b.mtd.realized_pnl,
    deposits: b.all_time.deposits,
    withdrawals: b.all_time.withdrawals,
    dividends: b.all_time.dividends_net,
    buyVol: b.all_time.buy_volume,
    sellVol: b.all_time.sell_volume,
    positions: b.open_positions.map(mapPosition),
    realizedBySymbol: b.realized_pnl_by_symbol,
    byYear: b.by_year.map(mapYearStat),
    dividendsBySymbol: b.dividends_by_symbol.map(mapDividend),
  }
}

export async function fetchPortfolioData(): Promise<PortfolioData> {
  const res = await fetch('/data/result.json')
  if (!res.ok) throw new Error(`Failed to load portfolio data: ${res.statusText}`)
  const raw: RawPortfolio = await res.json()

  const topRPnlMap = new Map<string, number>()
  for (const b of raw.brokers) {
    for (const r of b.realized_pnl_by_symbol) {
      topRPnlMap.set(r.symbol, (topRPnlMap.get(r.symbol) ?? 0) + r.pnl)
    }
  }
  const topRPnl = Array.from(topRPnlMap.entries())
    .map(([symbol, pnl]) => ({ symbol, pnl }))
    .sort((a, b) => b.pnl - a.pnl)

  const topDivs = raw.dividends_by_symbol
    .map(mapDividend)
    .sort((a, b) => b.net - a.net)

  return {
    generatedAt: raw.generated_at,
    baseCurrency: raw.base_currency,
    cashBalance: raw.cash_balance ?? 0,
    brokers: raw.brokers.map(mapBroker),
    allTime: mapPeriod(raw.all_time),
    ytd: mapPeriod(raw.ytd),
    mtd: mapPeriod(raw.mtd),
    byYear: raw.by_year.map(mapYearStat),
    openPositions: raw.open_positions.map(mapPosition),
    topRPnl,
    topDivs,
  }
}
