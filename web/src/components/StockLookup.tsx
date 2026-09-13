import { useEffect, useRef, useState, type ReactNode } from 'react'
import type { Position } from '../types/portfolio'
import { fetchTicker, fetchHistory, searchSymbols, type TickerSearchResult, type HistoryData } from '../services/portfolioService'
import { fmt, fmtCurrency, fmtPct, fmtKMBT, clr } from '../utils/format'
import { HEALTH_COLORS, VALUATION_COLORS, ratingLabel } from '../utils/ratings'
import { loadCode, loadTracked, saveTracked, upsertWatchlist, defaultTarget } from '../services/watchlistService'
import { RangeGauge } from './charts/RangeGauge'
import { Candlestick } from './charts/Candlestick'
import { InfoTooltip } from './ui/InfoTooltip'
import { useIsMobile } from '../lib/useIsMobile'

const CHART_RANGES = [
  { key: '1M', range: '1mo', interval: '1d' },
  { key: '6M', range: '6mo', interval: '1d' },
  { key: '1Y', range: '1y', interval: '1d' },
  { key: '5Y', range: '5y', interval: '1wk' },
  { key: '10Y', range: '10y', interval: '1wk' },
  { key: 'Max', range: 'max', interval: '1mo' },
] as const

type ChartRangeKey = (typeof CHART_RANGES)[number]['key']

const LOOKUP_EVENT = 'stock-lookup'

// openStockLookup opens the lookup modal for a symbol from anywhere in the app.
export function openStockLookup(symbol: string) {
  window.dispatchEvent(new CustomEvent(LOOKUP_EVENT, { detail: symbol }))
}

interface Props {
  accent: string
}

function Metric({ label, children, note }: { label: string; children: ReactNode; note?: string }) {
  const mobile = useIsMobile()
  if (mobile) {
    return (
      <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 8 }}>
        <span style={{ fontSize: 10, color: '#8b8fa3', display: 'inline-flex', alignItems: 'center', gap: 4, whiteSpace: 'nowrap' }}>
          {label}
          {note && <InfoTooltip text={note} />}
        </span>
        <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 12, fontWeight: 600, color: '#e0e0e0', textAlign: 'right' }}>{children}</span>
      </div>
    )
  }
  return (
    <div style={{ background: '#0a0a0a', border: '1px solid #161f31', borderRadius: 7, padding: '7px 10px', display: 'flex', flexDirection: 'column', gap: 4 }}>
      <span style={{ fontSize: 9, fontWeight: 600, color: '#8b8fa3', textTransform: 'uppercase', letterSpacing: '0.06em', display: 'inline-flex', alignItems: 'center', gap: 4 }}>
        {label}
        {note && <InfoTooltip text={note} />}
      </span>
      <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 13, fontWeight: 600, color: '#e0e0e0' }}>{children}</span>
    </div>
  )
}

function RatingPill({ rating, colors }: { rating?: string; colors: Record<string, string> }) {
  if (!rating) return <span style={{ color: '#8b8fa3' }}>—</span>
  const c = colors[rating] ?? '#8b8fa3'
  return (
    <span style={{ padding: '2px 8px', borderRadius: 4, fontSize: 12, fontWeight: 600, background: c + '22', color: c }}>
      {ratingLabel(rating)}
    </span>
  )
}

function MetricGroup({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      <div style={{ fontSize: 11, fontWeight: 700, color: '#c0c0c0', textTransform: 'uppercase', letterSpacing: '0.06em' }}>{title}</div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>{children}</div>
    </div>
  )
}

const NA = <span style={{ color: '#8b8fa3' }}>N/A</span>
const DASH = <span style={{ color: '#e0e0e0' }}>—</span>

// pctColored renders a percentage tinted by sign, or an em dash when absent.
function pctColored(v?: number): ReactNode {
  if (v == null) return DASH
  return <span style={{ color: clr(v) }}>{fmtPct(v)}</span>
}

// PerfPill is the mobile performance-strip tile: a bordered box with a small
// label over a colored return value, four to a row.
function PerfPill({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div style={{ background: '#0a0a0a', border: '1px solid #161f31', borderRadius: 10, padding: '10px 8px', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4 }}>
      <span style={{ fontSize: 10, color: '#8b8fa3' }}>{label}</span>
      <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 15, fontWeight: 700 }}>{children}</span>
    </div>
  )
}

// RangeBar is the mobile full-width 52-week range: a wide gradient track plus
// the percent-of-range and the low/high prices anchoring each end.
function RangeBar({ low, high, current }: { low: number; high: number; current: number }) {
  const pct = high > low ? Math.min(Math.max((current - low) / (high - low), 0), 1) : 0.5
  return (
    <div style={{ background: '#0a0a0a', border: '1px solid #161f31', borderRadius: 10, padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 10 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11, color: '#8b8fa3' }}>
        <span>52-Week Range</span>
        <span>{Math.round(pct * 100)}% of range</span>
      </div>
      <div style={{ position: 'relative', height: 6, borderRadius: 999 }}>
        <div style={{ position: 'absolute', inset: 0, borderRadius: 999, background: 'linear-gradient(90deg, #5eead4, #fde68a, #fb923c, #f87171)' }} />
        <div style={{ position: 'absolute', left: `${pct * 100}%`, top: -4, width: 3, height: 14, background: '#f5f5f5', borderRadius: 1, transform: 'translateX(-50%)' }} />
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, fontFamily: "'DM Mono', monospace", color: '#e0e0e0' }}>
        <span>{fmtCurrency(low)}</span>
        <span>{fmtCurrency(high)}</span>
      </div>
    </div>
  )
}

// MobileDetailBody trades the always-visible five-group grid for one group at
// a time behind a pill switcher, so a phone screen shows dense metrics
// without the endless stacked scroll the grid produces at 1-2 columns.
function MobileDetailBody({ p }: { p: Position }) {
  const hasRange = p.weekLow52 != null && p.weekHigh52 != null && p.currentPrice != null
  const groups: { title: string; content: ReactNode }[] = [
    {
      title: 'Valuation',
      content: (
        <>
          <Metric label="Market Cap" note={p.marketCapInterpretation}>{p.marketCap != null ? fmtKMBT(p.marketCap) : DASH}</Metric>
          <Metric label="P/E">{p.pe != null && p.pe > 0 ? fmt(p.pe, 1) : '—'}</Metric>
          <Metric label="Forward P/E">{p.forwardPE != null && p.forwardPE > 0 ? fmt(p.forwardPE, 1) : '—'}</Metric>
          <Metric label="Price/Sales" note={p.priceToSalesInterpretation}>{p.priceToSales != null && p.priceToSales !== 0 ? fmt(p.priceToSales, 2) : DASH}</Metric>
          <Metric label="EV/EBITDA" note={p.evToEbitdaInterpretation}>{p.evToEbitda != null && p.evToEbitda !== 0 ? fmt(p.evToEbitda, 1) : '—'}</Metric>
          <Metric label="Price/Book" note={p.priceToBookInterpretation}>{p.priceToBook != null && p.priceToBook !== 0 ? fmt(p.priceToBook, 2) : DASH}</Metric>
          <Metric label="Valuation" note={p.valuationReason}><RatingPill rating={p.valuationRating} colors={VALUATION_COLORS} /></Metric>
        </>
      ),
    },
    {
      title: 'Cash Flow',
      content: (
        <>
          <Metric label="FCF" note={p.fcfInterpretation}><span style={{ color: p.fcf != null ? clr(p.fcf) : '#e0e0e0' }}>{p.fcf != null ? fmtKMBT(p.fcf) : '—'}</span></Metric>
          <Metric label="Free Cash Flow Yield" note={p.fcfYieldInterpretation}>{p.fcfYield != null ? pctColored(p.fcfYield) : DASH}</Metric>
          <Metric label="SBC Adj. FCF Yield">{NA}</Metric>
          <Metric label="SBC Impact">{NA}</Metric>
          <Metric label="CF Quality" note={p.cashFlowQualityInterpretation}>{p.cashFlowQuality != null && p.cashFlowQuality !== 0 ? fmt(p.cashFlowQuality, 2) : '—'}</Metric>
        </>
      ),
    },
    {
      title: 'Margins & Growth',
      content: (
        <>
          <Metric label="Profit Margin" note={p.profitMarginInterpretation}>{pctColored(p.profitMargin)}</Metric>
          <Metric label="Operating Margin" note={p.operatingMarginInterpretation}>{pctColored(p.operatingMargin)}</Metric>
          <Metric label="Quarterly Earnings (YoY)" note={p.quarterlyEarningsGrowthInterpretation}>{pctColored(p.quarterlyEarningsGrowth)}</Metric>
          <Metric label="Quarterly Revenue (YoY)" note={p.quarterlyRevenueGrowthInterpretation}>{pctColored(p.quarterlyRevenueGrowth)}</Metric>
        </>
      ),
    },
    {
      title: 'Balance',
      content: (
        <>
          <Metric label="Cash" note={p.cashInterpretation}>{p.cash != null ? fmtKMBT(p.cash) : DASH}</Metric>
          <Metric label="Debt" note={p.debtInterpretation}>{p.debt != null ? fmtKMBT(p.debt) : DASH}</Metric>
          <Metric label="Net">{p.net != null ? <span style={{ color: clr(p.net) }}>{fmtKMBT(p.net)}</span> : DASH}</Metric>
          <Metric label="Debt/Equity" note={p.debtToEquityInterpretation}>{p.debtToEquity != null ? fmt(p.debtToEquity, 1) : '—'}</Metric>
          <Metric label="Health" note={p.healthReason}><RatingPill rating={p.healthRating} colors={HEALTH_COLORS} /></Metric>
        </>
      ),
    },
    {
      title: 'Dividend',
      content: (
        <>
          <Metric label="Dividend Yield" note={p.dividendYieldInterpretation}>{p.dividendYield != null ? fmtPct(p.dividendYield) : DASH}</Metric>
          <Metric label="Payout Ratio" note={p.payoutRatioInterpretation}>{p.payoutRatio != null ? fmtPct(p.payoutRatio) : DASH}</Metric>
          <Metric label="Payout Date" note={p.payoutDateInterpretation}>{p.payoutDate || DASH}</Metric>
        </>
      ),
    },
  ]
  const [active, setActive] = useState(0)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 8 }}>
        <PerfPill label="1D">{pctColored(p.todayReturn)}</PerfPill>
        <PerfPill label="YTD">{pctColored(p.ytdReturn)}</PerfPill>
        <PerfPill label="3Y">{pctColored(p.threeYrReturn)}</PerfPill>
        <PerfPill label="5Y">{pctColored(p.fiveYrReturn)}</PerfPill>
        <PerfPill label="10Y">{pctColored(p.tenYrReturn)}</PerfPill>
      </div>

      {hasRange && <RangeBar low={p.weekLow52!} high={p.weekHigh52!} current={p.currentPrice!} />}

      <div className="rail" style={{ display: 'flex', gap: 8, overflowX: 'auto', paddingBottom: 2 }}>
        {groups.map((g, i) => (
          <button
            key={g.title}
            onClick={() => setActive(i)}
            style={{
              flexShrink: 0, padding: '7px 14px', borderRadius: 999, fontSize: 12, fontWeight: 600,
              whiteSpace: 'nowrap', cursor: 'pointer',
              background: i === active ? '#6366f11f' : 'transparent',
              border: `1px solid ${i === active ? '#6366f1' : '#262626'}`,
              color: i === active ? '#a5a6f6' : '#8b8fa3',
              fontFamily: "'DM Sans', sans-serif",
            }}
          >
            {g.title}
          </button>
        ))}
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
          <div style={{ width: 3, height: 14, borderRadius: 2, background: '#6366f1' }} />
          <div style={{ fontSize: 12, fontWeight: 700, color: '#c0c0c0', textTransform: 'uppercase', letterSpacing: '0.06em' }}>{groups[active].title}</div>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>{groups[active].content}</div>
      </div>

      <div style={{ fontSize: 10, color: '#5c6070', textAlign: 'center', padding: '4px 0 2px' }}>
        Fundamentals as of {new Date().toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}. Verdicts are heuristics, not advice.
      </div>
    </div>
  )
}

function DetailBody({ p }: { p: Position }) {
  const mobile = useIsMobile()
  const hasRange = p.weekLow52 != null && p.weekHigh52 != null && p.currentPrice != null
  if (mobile) return <MobileDetailBody p={p} />
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* Performance strip — not part of the picture's five groups */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(128px, 1fr))', gap: 8 }}>
        <Metric label="Price">{p.currentPrice != null ? fmtCurrency(p.currentPrice) : '—'}</Metric>
        <Metric label="52 Week Range">
          {hasRange ? <RangeGauge low={p.weekLow52!} high={p.weekHigh52!} current={p.currentPrice!} width={80} /> : '—'}
        </Metric>
        <Metric label="YTD">{pctColored(p.ytdReturn)}</Metric>
        <Metric label="3Y">{pctColored(p.threeYrReturn)}</Metric>
        <Metric label="5Y">{pctColored(p.fiveYrReturn)}</Metric>
        <Metric label="10Y">{pctColored(p.tenYrReturn)}</Metric>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: 16, alignItems: 'start' }}>
        <MetricGroup title="Valuation">
          <Metric label="Market Cap" note={p.marketCapInterpretation}>{p.marketCap != null ? fmtKMBT(p.marketCap) : DASH}</Metric>
          <Metric label="P/E">{p.pe != null && p.pe > 0 ? fmt(p.pe, 1) : '—'}</Metric>
          <Metric label="Forward P/E">{p.forwardPE != null && p.forwardPE > 0 ? fmt(p.forwardPE, 1) : '—'}</Metric>
          <Metric label="Price/Sales" note={p.priceToSalesInterpretation}>{p.priceToSales != null && p.priceToSales !== 0 ? fmt(p.priceToSales, 2) : DASH}</Metric>
          <Metric label="EV/EBITDA" note={p.evToEbitdaInterpretation}>{p.evToEbitda != null && p.evToEbitda !== 0 ? fmt(p.evToEbitda, 1) : '—'}</Metric>
          <Metric label="Price/Book" note={p.priceToBookInterpretation}>{p.priceToBook != null && p.priceToBook !== 0 ? fmt(p.priceToBook, 2) : DASH}</Metric>
          <Metric label="Valuation" note={p.valuationReason}><RatingPill rating={p.valuationRating} colors={VALUATION_COLORS} /></Metric>
        </MetricGroup>

        <MetricGroup title="Cash Flow">
          <Metric label="FCF" note={p.fcfInterpretation}><span style={{ color: p.fcf != null ? clr(p.fcf) : '#e0e0e0' }}>{p.fcf != null ? fmtKMBT(p.fcf) : '—'}</span></Metric>
          <Metric label="Free Cash Flow Yield" note={p.fcfYieldInterpretation}>{p.fcfYield != null ? pctColored(p.fcfYield) : DASH}</Metric>
          <Metric label="SBC Adj. FCF Yield">{NA}</Metric>
          <Metric label="SBC Impact">{NA}</Metric>
          <Metric label="CF Quality" note={p.cashFlowQualityInterpretation}>{p.cashFlowQuality != null && p.cashFlowQuality !== 0 ? fmt(p.cashFlowQuality, 2) : '—'}</Metric>
        </MetricGroup>

        <MetricGroup title="Margins & Growth">
          <Metric label="Profit Margin" note={p.profitMarginInterpretation}>{pctColored(p.profitMargin)}</Metric>
          <Metric label="Operating Margin" note={p.operatingMarginInterpretation}>{pctColored(p.operatingMargin)}</Metric>
          <Metric label="Quarterly Earnings (YoY)" note={p.quarterlyEarningsGrowthInterpretation}>{pctColored(p.quarterlyEarningsGrowth)}</Metric>
          <Metric label="Quarterly Revenue (YoY)" note={p.quarterlyRevenueGrowthInterpretation}>{pctColored(p.quarterlyRevenueGrowth)}</Metric>
        </MetricGroup>

        <MetricGroup title="Balance">
          <Metric label="Cash" note={p.cashInterpretation}>{p.cash != null ? fmtKMBT(p.cash) : DASH}</Metric>
          <Metric label="Debt" note={p.debtInterpretation}>{p.debt != null ? fmtKMBT(p.debt) : DASH}</Metric>
          <Metric label="Net">{p.net != null ? <span style={{ color: clr(p.net) }}>{fmtKMBT(p.net)}</span> : DASH}</Metric>
          <Metric label="Debt/Equity" note={p.debtToEquityInterpretation}>{p.debtToEquity != null ? fmt(p.debtToEquity, 1) : '—'}</Metric>
          <Metric label="Health" note={p.healthReason}><RatingPill rating={p.healthRating} colors={HEALTH_COLORS} /></Metric>
        </MetricGroup>

        <MetricGroup title="Dividend">
          <Metric label="Dividend Yield" note={p.dividendYieldInterpretation}>{p.dividendYield != null ? fmtPct(p.dividendYield) : DASH}</Metric>
          <Metric label="Payout Ratio" note={p.payoutRatioInterpretation}>{p.payoutRatio != null ? fmtPct(p.payoutRatio) : DASH}</Metric>
          <Metric label="Payout Date" note={p.payoutDateInterpretation}>{p.payoutDate || DASH}</Metric>
        </MetricGroup>
      </div>
    </div>
  )
}

export function StockLookup({ accent }: Props) {
  const mobile = useIsMobile()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<TickerSearchResult[]>([])
  const [highlight, setHighlight] = useState(0)
  const [detailSymbol, setDetailSymbol] = useState<string | null>(null)
  const [detail, setDetail] = useState<Position | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [chartRange, setChartRange] = useState<ChartRangeKey>('1Y')
  const [history, setHistory] = useState<HistoryData | null>(null)
  const [chartError, setChartError] = useState<string | null>(null)
  // Tracked state comes from the locally cached symbol list, not a request: the
  // watchlist endpoint re-fetches indicators for every symbol, which is far too
  // much work to answer a yes/no question about one of them.
  const [tracked, setTracked] = useState(false)
  const [adding, setAdding] = useState(false)
  const [addError, setAddError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  // Ctrl+Space opens the lookup from anywhere.
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.ctrlKey && e.code === 'Space') {
        e.preventDefault()
        setDetailSymbol(null)
        setDetail(null)
        setOpen(true)
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [])

  useEffect(() => {
    if (open) inputRef.current?.focus()
  }, [open])

  // Debounced autocomplete search.
  useEffect(() => {
    const q = query.trim()
    if (!open || q.length < 1) {
      setResults([])
      return
    }
    const ctrl = new AbortController()
    const t = setTimeout(() => {
      searchSymbols(q, ctrl.signal)
        .then(r => { setResults(r); setHighlight(0) })
        .catch(() => { /* aborted or search unavailable — silently drop suggestions */ })
    }, 250)
    return () => { clearTimeout(t); ctrl.abort() }
  }, [query, open])

  // Lock page scroll while either layer is open, so a touch drag that starts
  // inside the modal doesn't chain into scrolling the page behind it once the
  // modal's own content runs out of room.
  useEffect(() => {
    if (!open && detailSymbol == null) return
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [open, detailSymbol])

  // Load candles whenever the detail symbol or selected range changes.
  useEffect(() => {
    if (detailSymbol == null) return
    const cfg = CHART_RANGES.find(r => r.key === chartRange)!
    const ctrl = new AbortController()
    setHistory(null)
    setChartError(null)
    fetchHistory(detailSymbol, cfg.range, cfg.interval, ctrl.signal)
      .then(setHistory)
      .catch(err => { if (!ctrl.signal.aborted) setChartError(err instanceof Error ? err.message : 'Chart unavailable') })
    return () => ctrl.abort()
  }, [detailSymbol, chartRange])

  function closeAll() {
    setOpen(false)
    setQuery('')
    setResults([])
    setDetailSymbol(null)
    setDetail(null)
    setError(null)
    setLoading(false)
    setHistory(null)
    setChartError(null)
    setAddError(null)
  }

  // Adding from here seeds the same 20%-below-price target the watchlist tab
  // seeds, so a symbol arrives in the same state either way. Never called for a
  // symbol already tracked: the write is a full replace server-side and would
  // drop the note, target and pin the user already set.
  async function addToWatchlist() {
    if (detailSymbol == null || tracked || adding) return
    setAdding(true)
    setAddError(null)
    try {
      const price = detail?.currentPrice
      await upsertWatchlist({ symbol: detailSymbol, ...(price != null ? { targetPrice: defaultTarget(price) } : {}) })
      saveTracked([...loadTracked(), detailSymbol])
      setTracked(true)
    } catch (err) {
      setAddError(err instanceof Error ? err.message : 'Could not add')
    }
    setAdding(false)
  }

  async function pick(symbol: string) {
    const sym = symbol.trim().toUpperCase()
    if (!sym) return
    setOpen(false)
    setResults([])
    setDetail(null)
    setError(null)
    setAddError(null)
    setTracked(loadTracked().includes(sym))
    setLoading(true)
    setChartRange('1Y')
    setDetailSymbol(sym)
    try {
      setDetail(await fetchTicker(sym))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Lookup failed')
    }
    setLoading(false)
  }

  // Let other parts of the app (e.g. a positions row) open the lookup directly.
  const pickRef = useRef(pick)
  pickRef.current = pick
  useEffect(() => {
    const handler = (e: Event) => {
      const sym = (e as CustomEvent<string>).detail
      if (sym) pickRef.current(sym)
    }
    window.addEventListener(LOOKUP_EVENT, handler as EventListener)
    return () => window.removeEventListener(LOOKUP_EVENT, handler as EventListener)
  }, [])

  function onInputKey(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setHighlight(h => Math.min(h + 1, results.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setHighlight(h => Math.max(h - 1, 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      const chosen = results[highlight]
      pick(chosen ? chosen.symbol : query)
    }
  }

  // Escape closes whichever layer is open.
  useEffect(() => {
    if (!open && detailSymbol == null) return
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (detailSymbol != null) { setDetailSymbol(null); setDetail(null); setError(null) }
        else setOpen(false)
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [open, detailSymbol])

  return (
    <>
      {open && detailSymbol == null && (
        <div
          onClick={e => e.target === e.currentTarget && setOpen(false)}
          style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.6)', zIndex: 50, display: 'flex', alignItems: 'flex-start', justifyContent: 'center', paddingTop: '12vh' }}
        >
          <div style={{ width: 'min(520px, 92vw)', background: '#090f1c', border: '1px solid #262626', borderRadius: 12, overflow: 'hidden', boxShadow: '0 24px 60px rgba(0,0,0,0.6)' }}>
            <input
              ref={inputRef}
              value={query}
              onChange={e => setQuery(e.target.value)}
              onKeyDown={onInputKey}
              placeholder="Search any stock — symbol or name…"
              style={{ width: '100%', boxSizing: 'border-box', background: 'transparent', border: 'none', outline: 'none', color: '#f0f0f0', fontSize: 16, padding: '16px 18px' }}
            />
            {results.length > 0 && (
              <div style={{ borderTop: '1px solid #161f31', maxHeight: 320, overflowY: 'auto' }}>
                {results.map((r, i) => (
                  <div
                    key={`${r.symbol}-${i}`}
                    onMouseEnter={() => setHighlight(i)}
                    onClick={() => pick(r.symbol)}
                    style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '9px 18px', cursor: 'pointer', background: i === highlight ? accent + '1c' : 'transparent' }}
                  >
                    <span style={{ fontWeight: 700, fontSize: 13, color: '#f0f0f0', minWidth: 64 }}>{r.symbol}</span>
                    <span style={{ flex: 1, fontSize: 12, color: '#999999', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{r.name}</span>
                    <span style={{ fontSize: 10, color: '#8b8fa3' }}>{r.exchange}</span>
                  </div>
                ))}
              </div>
            )}
            <div style={{ borderTop: '1px solid #161f31', padding: '7px 18px', fontSize: 10, color: '#8b8fa3' }}>
              ↑↓ to navigate · Enter to open · Esc to close
            </div>
          </div>
        </div>
      )}

      {detailSymbol != null && (
        <div className="chart-modal-backdrop" onClick={e => e.target === e.currentTarget && closeAll()}>
          <div className="chart-modal" style={{ height: '92vh', maxHeight: 980 }}>
            <div className="chart-modal-header">
              <div className="chart-modal-meta">
                <div className="chart-modal-label">Stock Lookup</div>
                <div className="chart-modal-value" style={{ color: accent }}>{detailSymbol}</div>
              </div>
              <div className="chart-modal-actions">
                {loadCode() && (
                  <button
                    onClick={addToWatchlist}
                    disabled={tracked || adding}
                    title={addError ?? (tracked ? `${detailSymbol} is on your watchlist` : `Add ${detailSymbol} to your watchlist`)}
                    style={{
                      padding: '6px 12px',
                      borderRadius: 999,
                      background: tracked ? '#34d39914' : accent + '1f',
                      border: `1px solid ${tracked ? '#34d39955' : accent + '55'}`,
                      color: addError ? '#f87171' : tracked ? '#34d399' : accent,
                      fontSize: 11,
                      fontWeight: 700,
                      fontFamily: "'DM Sans', sans-serif",
                      cursor: tracked || adding ? 'default' : 'pointer',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {addError ? 'Failed' : tracked ? '✓ On watchlist' : adding ? 'Adding…' : '+ Watchlist'}
                  </button>
                )}
                <button className="modal-close" onClick={closeAll}>✕</button>
              </div>
            </div>
            <div className="chart-modal-body">
              {loading && <div style={{ color: '#8b8fa3', fontSize: 14, padding: '30px 0', textAlign: 'center' }}>Loading {detailSymbol}…</div>}
              {!loading && error && <div style={{ color: '#f87171', fontSize: 14, padding: '30px 0', textAlign: 'center' }}>{error}</div>}
              {!loading && !error && detail && mobile && (
                <div style={{ display: 'flex', alignItems: 'baseline', gap: 10, flexWrap: 'wrap', marginBottom: 12 }}>
                  {detail.currentPrice != null && (
                    <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 30, fontWeight: 700, color: '#f0f0f0' }}>{fmtCurrency(detail.currentPrice)}</span>
                  )}
                  {detail.todayReturn != null && (
                    <span style={{ padding: '3px 9px', borderRadius: 6, fontSize: 13, fontWeight: 700, background: clr(detail.todayReturn) + '1f', color: clr(detail.todayReturn) }}>
                      {fmtPct(detail.todayReturn)}
                    </span>
                  )}
                  {(detail.currency || detail.sector) && (
                    <span style={{ fontSize: 12, color: '#8b8fa3', marginLeft: 'auto' }}>
                      {[detail.currency, detail.sector].filter(Boolean).join(' · ')}
                    </span>
                  )}
                </div>
              )}
              {!loading && !error && detail && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
                  <div>
                    <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 8 }}>
                      <div className="chart-type-toggle">
                        {CHART_RANGES.map(r => (
                          <button
                            key={r.key}
                            className={`chart-type-btn ${chartRange === r.key ? 'active' : ''}`}
                            onClick={() => setChartRange(r.key)}
                          >
                            {r.key}
                          </button>
                        ))}
                      </div>
                    </div>
                    {chartError ? (
                      <div style={{ color: '#8b8fa3', fontSize: 13, padding: '30px 0', textAlign: 'center' }}>{chartError}</div>
                    ) : history == null ? (
                      <div style={{ color: '#8b8fa3', fontSize: 13, padding: '30px 0', textAlign: 'center' }}>Loading chart…</div>
                    ) : (
                      <Candlestick candles={history.candles} ma={history.ma} />
                    )}
                  </div>
                  <DetailBody p={detail} />
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  )
}
