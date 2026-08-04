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
    transferIn: p.transfer_in,
    transferOut: p.transfer_out,
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
    avgCost: p.avg_cost,
    currentPrice: p.current_price,
    weekLow52: p.week_52_low,
    weekHigh52: p.week_52_high,
  }
}

function mapYearStat(p: RawPeriod): YearStat {
  return {
    label: p.label,
    gainPct: p.gain_pct,
    rPnl: p.realized_pnl,
    divs: p.dividends_net,
    deposits: p.deposits,
    transferIn: p.transfer_in,
    transferOut: p.transfer_out,
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
    transferIn: b.all_time.transfer_in,
    transferOut: b.all_time.transfer_out,
    dividends: b.all_time.dividends_net,
    buyVol: b.all_time.buy_volume,
    sellVol: b.all_time.sell_volume,
    positions: b.open_positions.map(mapPosition),
    realizedBySymbol: b.realized_pnl_by_symbol,
    byYear: b.by_year.map(mapYearStat),
    dividendsBySymbol: b.dividends_by_symbol.map(mapDividend),
  }
}

const CACHE_KEY = 'brokers-sync:portfolio'

export function cacheRawPortfolio(raw: RawPortfolio): void {
  try { localStorage.setItem(CACHE_KEY, JSON.stringify(raw)) } catch { /* quota exceeded */ }
}

export function loadCachedPortfolio(): PortfolioData | null {
  try {
    const raw = localStorage.getItem(CACHE_KEY)
    if (!raw) return null
    return mapRawPortfolio(JSON.parse(raw) as RawPortfolio)
  } catch { return null }
}

export async function uploadZip(
  blob: Blob,
  name: string,
  onLog?: (line: string) => void,
): Promise<RawPortfolio | null> {
  const fd = new FormData()
  fd.append('file', blob, name)
  const res = await fetch('/api/upload/zip', { method: 'POST', body: fd })
  if (!res.ok || !res.body) throw new Error(`Server returned ${res.status} ${res.statusText}`)

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buf = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buf += decoder.decode(value, { stream: true })
    const parts = buf.split('\n\n')
    buf = parts.pop() ?? ''
    for (const part of parts) {
      const dataLine = part.split('\n').find(l => l.startsWith('data: '))
      if (!dataLine) continue
      let ev: { type: string; line?: string; success?: boolean; report?: unknown; error?: string } | null = null
      try { ev = JSON.parse(dataLine.slice(6)) } catch { /* ignore malformed */ }
      if (!ev) continue
      if (ev.type === 'log' && ev.line != null) onLog?.(ev.line)
      else if (ev.type === 'done') {
        if (!ev.success) throw new Error(ev.error ?? 'Import failed')
        return ev.report ? (ev.report as RawPortfolio) : null
      }
    }
  }
  return null
}

export async function fetchPortfolioData(): Promise<PortfolioData | null> {
  const res = await fetch('/data/result.json')
  if (!res.ok) return loadCachedPortfolio()
  // CloudFront maps 403/404 to index.html (200) — guard against parsing HTML as JSON
  const ct = res.headers.get('content-type') ?? ''
  if (!ct.includes('application/json')) return loadCachedPortfolio()
  const raw: RawPortfolio = await res.json()
  return mapRawPortfolio(raw)
}

export function mapRawPortfolio(raw: RawPortfolio): PortfolioData {

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
