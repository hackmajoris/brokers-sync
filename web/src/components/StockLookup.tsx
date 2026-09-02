import { useEffect, useRef, useState, type ReactNode } from 'react'
import type { Position } from '../types/portfolio'
import { fetchTicker, fetchHistory, searchSymbols, type TickerSearchResult, type HistoryData } from '../services/portfolioService'
import { fmt, fmtCurrency, fmtPct, fmtKMBT, clr } from '../utils/format'
import { HEALTH_COLORS, VALUATION_COLORS, ratingLabel } from '../utils/ratings'
import { RangeGauge } from './charts/RangeGauge'
import { Candlestick } from './charts/Candlestick'
import { InfoTooltip } from './ui/InfoTooltip'

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

function DetailBody({ p }: { p: Position }) {
  const hasRange = p.weekLow52 != null && p.weekHigh52 != null && p.currentPrice != null
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
  }

  async function pick(symbol: string) {
    const sym = symbol.trim().toUpperCase()
    if (!sym) return
    setOpen(false)
    setResults([])
    setDetail(null)
    setError(null)
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
                <button className="modal-close" onClick={closeAll}>✕</button>
              </div>
            </div>
            <div className="chart-modal-body" style={{ overflowY: 'auto' }}>
              {loading && <div style={{ color: '#8b8fa3', fontSize: 14, padding: '30px 0', textAlign: 'center' }}>Loading {detailSymbol}…</div>}
              {!loading && error && <div style={{ color: '#f87171', fontSize: 14, padding: '30px 0', textAlign: 'center' }}>{error}</div>}
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
