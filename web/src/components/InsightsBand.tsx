import { Children, Fragment, useState, type ReactNode } from 'react'
import type { Position } from '../types/portfolio'
import { fmt, fmtCurrency, fmtPct, clr } from '../utils/format'
import { HEALTH_COLORS, VALUATION_COLORS, ratingLabel } from '../utils/ratings'
import { rangePct, type IndicatorKey } from './IndicatorColumns'
import { openStockLookup } from './StockLookup'

// The summary band above the watchlist and positions tables. Both tabs show the
// same reading of the same indicators; only the watchlist has buy targets and
// pinned rows, so those two widgets are opt-in rather than forked copies.
export type BandMetric = IndicatorKey | 'targetGap'

// The shape the band reads. A watchlist item satisfies it directly; a position
// supplies itself as `indicators` and leaves the watchlist-only fields unset.
export interface BandRow {
  symbol: string
  indicators?: Position
  pinned?: boolean
  // Percent the price still has to move to reach the user's buy target.
  // Undefined wherever targets do not apply, which is the positions tab.
  targetGap?: number | null
}

// Which widgets a tab opts into. Positions have neither a buy target nor a
// pinned flag, so those widgets and their filter chips are absent there rather
// than rendering permanently empty.
export interface BandFeatures {
  targets?: boolean
  pinned?: boolean
}

const PIN_EDGE = '#d9a441'

export const MC = {
  surf: '#111827',
  line: '#161f31',
  mut: '#64748b',
  dim: '#475569',
  txt: '#e2e8f0',
}


const ALL_FILTERS = ['All', '★ Pinned', 'Gainers', 'Losers', 'Undervalued'] as const

// Pinned and Undervalued read fields only the watchlist has; offering them on
// the positions tab would be a chip that always returns nothing.
export function filtersFor(features: BandFeatures): RowFilter[] {
  return ALL_FILTERS.filter(f => (f === '★ Pinned' ? !!features.pinned : f === 'Undervalued' ? !!features.targets : true))
}
export type RowFilter = (typeof ALL_FILTERS)[number]

// "Undervalued" is the target reading, not a screen: the price still has room
// to rise to the target the user set.
export function matchesFilter(item: BandRow, filter: RowFilter, tf: BandMetric): boolean {
  if (filter === 'All') return true
  if (filter === '★ Pinned') return !!item.pinned
  if (filter === 'Undervalued') return (item.targetGap ?? 0) > 0
  const v = cellValue(item, tf)
  if (v == null) return false
  return filter === 'Gainers' ? v > 0 : v < 0
}

export function cellValue(item: BandRow, key: BandMetric | 'marketCap'): number | undefined {
  const p = item.indicators
  switch (key) {
    case 'today':
      return p?.todayReturn
    case 'oneWeek':
      return p?.oneWeekReturn
    case 'oneMonth':
      return p?.oneMonthReturn
    case 'ytd':
      return p?.ytdReturn
    case 'fiveYr':
      return p?.fiveYrReturn
    case 'tenYr':
      return p?.tenYrReturn
    case 'pe':
      return p?.pe != null && p.pe > 0 ? p.pe : undefined
    case 'forwardPe':
      return p?.forwardPE != null && p.forwardPE > 0 ? p.forwardPE : undefined
    case 'peVsSector':
      return p?.peVsSector
    case 'targetGap':
      return item.targetGap ?? undefined
    case 'marketCap':
      return p?.marketCap
    default:
      return undefined
  }
}

// A four-digit percentage eats two columns, so anything past 1000% collapses to
// a multiple: +1,284% reads as 13x in the same width.
export function compactPct(v: number): string {
  if (Math.abs(v) >= 1000) return `${v < 0 ? '-' : ''}${(Math.abs(v) / 100).toFixed(0)}x`
  return `${v >= 0 ? '+' : ''}${v.toFixed(1)}%`
}


function Rail({
  title,
  accentBar,
  items,
  tf,
  onOpen,
}: {
  title: string
  accentBar: string
  items: BandRow[]
  tf: BandMetric
  onOpen: (symbol: string) => void
}) {
  if (items.length === 0) return null
  return (
    <div style={{ width: '100%' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
        <span style={{ width: 3, height: 11, borderRadius: 2, background: accentBar }} />
        <span style={{ fontSize: 11, fontWeight: 700, letterSpacing: '0.09em', textTransform: 'uppercase', color: '#94a3b8' }}>{title}</span>
      </div>
      <div className="rail" style={{ display: 'flex', gap: 8, overflowX: 'auto', scrollSnapType: 'x mandatory' }}>
        {items.map(item => (
          <div key={item.symbol} style={{ scrollSnapAlign: 'start' }}>
            <MoverCard item={item} tone={accentBar} tf={tf} onOpen={onOpen} />
          </div>
        ))}
      </div>
    </div>
  )
}

function MoverCard({ item, tone, tf, onOpen }: { item: BandRow; tone: string; tf: BandMetric; onOpen: (symbol: string) => void }) {
  const p = item.indicators
  const move = cellValue(item, tf)
  return (
    <button
      onClick={() => onOpen(item.symbol)}
      style={{
        width: 132,
        background: MC.surf,
        border: `1px solid ${tone}2e`,
        borderRadius: 11,
        padding: '6px 10px',
        display: 'flex',
        flexDirection: 'column',
        gap: 3,
        cursor: 'pointer',
        textAlign: 'left',
        fontFamily: "'DM Sans', sans-serif",
        flexShrink: 0,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 4, width: '100%' }}>
        <span style={{ fontSize: 12.5, fontWeight: 700, color: MC.txt, letterSpacing: '-0.01em', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {item.symbol}
        </span>
        {item.pinned && <span style={{ color: PIN_EDGE, fontSize: 9, flexShrink: 0 }}>★</span>}
      </div>
      <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', width: '100%' }}>
        <span style={{ fontSize: 12, fontFamily: "'DM Mono', monospace", color: '#cbd5e1' }}>
          {p?.currentPrice != null ? fmtCurrency(p.currentPrice) : '—'}
        </span>
        <span style={{ fontSize: 11.5, fontWeight: 700, fontFamily: "'DM Mono', monospace", color: tone }}>
          {move != null ? compactPct(move) : '—'}
        </span>
      </div>
    </button>
  )
}

// Up and down counts live on the breadth bar, which shows the split as well as
// the numbers, so the tiles spend their width on the extremes instead.
function StatTiles({ total, rows, tf, strip = false }: { total: number; rows: BandRow[]; tf: BandMetric; strip?: boolean }) {
  const scored = rows
    .map(r => ({ symbol: r.symbol, v: cellValue(r, tf) }))
    .filter((x): x is { symbol: string; v: number } => x.v != null)
    .sort((a, b) => b.v - a.v)
  const vals = scored.map(x => x.v)
  const avg = vals.length ? vals.reduce((s, v) => s + v, 0) / vals.length : null
  const best = scored[0]
  const worst = scored[scored.length - 1]
  const extreme = (x: typeof best) => (x ? `${x.symbol} ${compactPct(x.v)}` : '—')
  const tiles: [string, string, string][] = [
    ['Tracked', String(total), MC.txt],
    ['Avg', avg == null ? '—' : compactPct(avg), avg == null ? MC.mut : clr(avg)],
    ['Best', extreme(best), best ? clr(best.v) : MC.mut],
    ['Worst', extreme(worst), worst ? clr(worst.v) : MC.mut],
  ]
  // On a desktop the four readings are one sentence rather than four boxes:
  // they are a single thought about the list, and four bordered tiles gave them
  // more visual weight than the rails they sit above.
  if (strip) {
    return (
      <div style={{ display: 'flex', alignItems: 'baseline', flexWrap: 'wrap', gap: 10, width: '100%' }}>
        {tiles.map(([k, v, c], i) => (
          <Fragment key={k}>
            {i > 0 && <span style={{ color: MC.line, fontSize: 11 }}>·</span>}
            <span style={{ display: 'inline-flex', alignItems: 'baseline', gap: 5 }}>
              <span style={{ fontSize: 9, color: MC.dim, fontWeight: 700, letterSpacing: '0.07em', textTransform: 'uppercase' }}>{k}</span>
              <span style={{ fontSize: 13, fontWeight: 700, color: c, fontFamily: "'DM Mono', monospace", letterSpacing: '-0.03em' }}>{v}</span>
            </span>
          </Fragment>
        ))}
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', gap: 6, width: '100%' }}>
      {tiles.map(([k, v, c]) => (
        <div key={k} style={{ flex: 1, background: MC.surf, border: `1px solid ${MC.line}`, borderRadius: 9, padding: '4px 8px' }}>
          <div style={{ fontSize: 8.5, color: MC.dim, fontWeight: 600, letterSpacing: '0.06em', textTransform: 'uppercase' }}>{k}</div>
          <div
            title={v}
            style={{
              // "NVDA +4.2%" is twice the width of a bare count, and the tiles
              // share the row evenly, so the longer readings step down a size.
              fontSize: v.length > 7 ? 11 : 14,
              fontWeight: 700,
              color: c,
              fontFamily: "'DM Mono', monospace",
              letterSpacing: '-0.03em',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
            }}
          >
            {v}
          </div>
        </div>
      ))}
    </div>
  )
}


// The timeframe every summary widget reads against, and the one "Gainers" and
// "Losers" mean. Independent of the column sort, which owns row order only.
export const TIMEFRAMES: { key: BandMetric; label: string }[] = [
  { key: 'today', label: '1D' },
  { key: 'oneWeek', label: '1W' },
  { key: 'oneMonth', label: '1M' },
  { key: 'ytd', label: 'YTD' },
  { key: 'fiveYr', label: '5Y' },
  { key: 'tenYr', label: '10Y' },
]

function TimeframePills({ value, onPick, accent }: { value: BandMetric; onPick: (k: BandMetric) => void; accent: string }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 6, width: '100%' }}>
      <span style={{ fontSize: 9, fontWeight: 700, letterSpacing: '0.07em', textTransform: 'uppercase', color: MC.dim }}>Period</span>
      {TIMEFRAMES.map(t => (
        <button
          key={t.key}
          onClick={() => onPick(t.key)}
          aria-pressed={value === t.key}
          style={{
            padding: '3px 9px',
            borderRadius: 999,
            fontSize: 10.5,
            fontWeight: 700,
            cursor: 'pointer',
            fontFamily: "'DM Sans', sans-serif",
            background: value === t.key ? accent + '26' : 'transparent',
            color: value === t.key ? accent : MC.mut,
            border: `1px solid ${value === t.key ? accent + '55' : MC.line}`,
          }}
        >
          {t.label}
        </button>
      ))}
    </div>
  )
}

function FilterChips({ value, onPick, accent, filters }: { value: RowFilter; onPick: (f: RowFilter) => void; accent: string; filters: RowFilter[] }) {
  return (
    <div className="rail" style={{ display: 'flex', gap: 6, overflowX: 'auto', width: '100%' }}>
      {filters.map(c => (
        <button
          key={c}
          onClick={() => onPick(c)}
          style={{
            padding: '5px 11px',
            borderRadius: 999,
            fontSize: 11,
            fontWeight: 600,
            cursor: 'pointer',
            whiteSpace: 'nowrap',
            fontFamily: "'DM Sans', sans-serif",
            background: value === c ? accent + '26' : MC.surf,
            color: value === c ? accent : MC.mut,
            border: `1px solid ${value === c ? accent + '55' : MC.line}`,
          }}
        >
          {c}
        </button>
      ))}
    </div>
  )
}


export function InsightsBand({
  accent,
  items,
  rows,
  tf,
  filter,
  onFilter,
  onTf,
  features = {},
  compact = false,
}: {
  accent: string
  // Every row the tab holds — the movers and signal lists read the whole list,
  // so a filter narrowing the table below never redefines "biggest gainer".
  items: BandRow[]
  // The rows left after filtering, which is what the summary readings describe.
  rows: BandRow[]
  tf: BandMetric
  filter: RowFilter
  onFilter: (f: RowFilter) => void
  onTf: (k: BandMetric) => void
  features?: BandFeatures
  // The phone gets the same widgets, re-laid out: the rails stack and the
  // signal cards become a side-scrolling row rather than shrinking to a width
  // where the symbol chips inside them wrap one per line.
  compact?: boolean
}) {
  const tfLabel = TIMEFRAMES.find(t => t.key === tf)?.label ?? ''
  const railSize = compact ? 3 : 5
  const byMove = items
    .map(i => ({ item: i, v: cellValue(i, tf) }))
    .filter((x): x is { item: BandRow; v: number } => x.v != null)
    .sort((a, b) => b.v - a.v)
  const gainers = byMove.filter(x => x.v > 0).slice(0, railSize).map(x => x.item)
  const losers = byMove.filter(x => x.v < 0).slice(-railSize).reverse().map(x => x.item)

  // Price below the buy target the user set — the whole point of the target
  // column, surfaced instead of having to scan for green inputs.
  const targetHits = items
    .filter(i => (i.targetGap ?? 0) > 0)
    .sort((a, b) => (b.targetGap ?? 0) - (a.targetGap ?? 0))

  const ranged = items.filter(i => i.indicators != null).map(i => ({ item: i, r: rangePct(i.indicators!) }))
  const nearHigh = ranged.filter(x => x.r >= 0.9 && x.r <= 1.5).map(x => x.item)
  const nearLow = ranged.filter(x => x.r >= 0 && x.r <= 0.1).map(x => x.item)

  const flagged = items.map(i => ({ item: i, flag: redFlag(i.indicators) })).filter(x => x.flag != null)
  // Yield is worth nothing if the payout cannot be sustained, so the two are
  // read together rather than ranking on yield alone.
  const income = items
    .filter(i => (i.indicators?.dividendYield ?? 0) > 0 && (i.indicators?.payoutRatio ?? 0) <= SAFE_PAYOUT)
    .sort((a, b) => (b.indicators!.dividendYield ?? 0) - (a.indicators!.dividendYield ?? 0))

  return (
    <div style={{ width: '100%', display: 'flex', flexDirection: 'column', gap: 16 }}>
      <TimeframePills value={tf} onPick={onTf} accent={accent} />
      {compact ? (
        <>
          <StatTiles total={items.length} rows={rows} tf={tf} />
          <BreadthBar rows={rows} tf={tf} />
          <Rail title={`${tfLabel} Gainers`} accentBar="#34d399" items={gainers} tf={tf} onOpen={openStockLookup} />
          <Rail title={`${tfLabel} Losers`} accentBar="#f87171" items={losers} tf={tf} onOpen={openStockLookup} />
        </>
      ) : (
        <>
          <StatTiles total={items.length} rows={rows} tf={tf} strip />
          <BreadthBar rows={rows} tf={tf} />
          <div style={{ display: 'flex', gap: 14, width: '100%', alignItems: 'flex-start' }}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <Rail title={`${tfLabel} Gainers`} accentBar="#34d399" items={gainers} tf={tf} onOpen={openStockLookup} />
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <Rail title={`${tfLabel} Losers`} accentBar="#f87171" items={losers} tf={tf} onOpen={openStockLookup} />
            </div>
          </div>
        </>
      )}
      {/* Collapsed, each of these is a single header strip, so all six fit one
          row. Expanding one grows that row only. */}
      <SignalRow compact={compact}>
        {features.targets && <SignalList title="Target hit" tone="#34d399" items={targetHits} note={i => fmtPct(i.targetGap ?? 0)} />}
        <SignalList title="Near 52w high" tone="#f59e0b" items={nearHigh} />
        <SignalList title="Near 52w low" tone="#60a5fa" items={nearLow} />
        <QualityValueGrid items={items} />
        <SignalList title="Red flags" tone="#f87171" items={flagged.map(x => x.item)} note={i => redFlag(i.indicators) ?? ''} />
        <SignalList title="Income" tone="#a78bfa" items={income} note={i => fmt(i.indicators?.dividendYield, 1) + '%'} />
      </SignalRow>
      <FilterChips value={filter} onPick={onFilter} accent={accent} filters={filtersFor(features)} />
    </div>
  )
}


// Payout above this share of earnings leaves no buffer, which is the line the
// upstream interpretation draws too, so a high yield behind it is not income
// worth chasing.
const SAFE_PAYOUT = 80

// Balance-sheet problems the valuation columns say nothing about. The first
// match wins: the point is to flag the symbol for a closer look, not to score
// it. Debt-to-equity above 200% rather than the upstream 100% — leverage over
// equity is ordinary in capital-heavy sectors and flagging it fires on half the
// list.
function redFlag(p?: Position): string | null {
  if (!p) return null
  if (p.fcf != null && p.fcf < 0) return 'FCF<0'
  if (p.cashFlowQuality != null && p.cashFlowQuality < 0.8) return `CFQ ${p.cashFlowQuality.toFixed(1)}`
  if (p.debtToEquity != null && p.debtToEquity >= 200) return `D/E ${p.debtToEquity.toFixed(0)}`
  if (p.payoutRatio != null && p.payoutRatio > 100) return `Payout ${p.payoutRatio.toFixed(0)}%`
  return null
}

// Health and Valuation are separate columns and answer different questions, so
// the pair only becomes useful crossed: cheap-and-healthy is the shortlist,
// cheap-and-unhealthy is the value trap. Counts here, symbols on hover.
const GRID_HEALTH = ['healthy', 'fair', 'weak', 'unhealthy']
const GRID_VALUATION = ['undervalued', 'fair', 'overvalued']

function QualityValueGrid({ items }: { items: BandRow[] }) {
  const cell = (h: string, v: string) =>
    items.filter(i => i.indicators?.healthRating === h && i.indicators?.valuationRating === v)

  // Only symbols carrying both ratings land in a cell, so the count on the
  // header is the population the grid actually describes — not the list length.
  const rated = items.filter(i => i.indicators?.healthRating && i.indicators?.valuationRating).length

  return (
    <SignalCard title="Health × Valuation" tone="#818cf8" count={rated}>
      <div style={{ display: 'grid', gridTemplateColumns: `54px repeat(${GRID_VALUATION.length}, 1fr)`, gap: 2 }}>
        <span />
        {GRID_VALUATION.map(v => (
          <span key={v} style={{ fontSize: 8.5, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.05em', color: VALUATION_COLORS[v], textAlign: 'center' }}>
            {v.slice(0, 5)}
          </span>
        ))}
        {GRID_HEALTH.map(h => (
          <Fragment key={h}>
            <span style={{ fontSize: 8.5, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.05em', color: HEALTH_COLORS[h], alignSelf: 'center' }}>
              {ratingLabel(h)}
            </span>
            {GRID_VALUATION.map(v => {
              const hit = cell(h, v)
              // The one combination worth acting on gets the accent; everything
              // else stays neutral so the eye lands on it first.
              const sweet = v === 'undervalued' && (h === 'healthy' || h === 'fair')
              return (
                <span
                  key={v}
                  title={hit.length ? hit.map(i => i.symbol).join(', ') : `No ${ratingLabel(h)} · ${v} symbols`}
                  style={{
                    textAlign: 'center',
                    padding: '3px 0',
                    borderRadius: 5,
                    fontSize: 11,
                    fontWeight: 700,
                    fontFamily: "'DM Mono', monospace",
                    cursor: hit.length ? 'help' : 'default',
                    background: hit.length === 0 ? 'transparent' : sweet ? '#34d39926' : '#1e293b',
                    color: hit.length === 0 ? MC.dim : sweet ? '#34d399' : '#cbd5e1',
                    border: `1px solid ${hit.length && sweet ? '#34d39955' : 'transparent'}`,
                  }}
                >
                  {hit.length || '·'}
                </span>
              )
            })}
          </Fragment>
        ))}
      </div>
    </SignalCard>
  )
}

// Up against down over the active timeframe, as one bar. Counts come from the
// filtered rows, so it always describes what the table below is showing.
function BreadthBar({ rows, tf }: { rows: BandRow[]; tf: BandMetric }) {
  const vals = rows.map(r => cellValue(r, tf)).filter((v): v is number => v != null)
  const up = vals.filter(v => v > 0).length
  const down = vals.filter(v => v < 0).length
  const total = up + down
  const upPct = total === 0 ? 0 : (up / total) * 100
  return (
    <div style={{ width: '100%', display: 'flex', alignItems: 'center', gap: 10 }}>
      <span style={{ fontSize: 10, fontFamily: "'DM Mono', monospace", color: '#34d399', width: 46, textAlign: 'right' }}>{up} up</span>
      <div style={{ flex: 1, height: 6, borderRadius: 3, background: total === 0 ? MC.line : '#f87171', overflow: 'hidden' }}>
        <div style={{ width: `${upPct}%`, height: '100%', background: '#34d399' }} />
      </div>
      <span style={{ fontSize: 10, fontFamily: "'DM Mono', monospace", color: '#f87171', width: 52 }}>{down} down</span>
    </div>
  )
}

// Three signal cards across on a desktop. On a phone that leaves ~110px each,
// which wraps every symbol chip onto its own line, so they become a swipeable
// row at a readable width instead.
function SignalRow({ compact, children }: { compact: boolean; children: React.ReactNode }) {
  if (!compact) {
    return <div style={{ display: 'flex', gap: 10, width: '100%', alignItems: 'flex-start' }}>{children}</div>
  }
  return (
    <div className="rail" style={{ display: 'flex', gap: 8, width: '100%', overflowX: 'auto', WebkitOverflowScrolling: 'touch' }}>
      {Children.map(children, child => (
        <div style={{ width: 212, flexShrink: 0, display: 'flex', scrollSnapAlign: 'start' }}>{child}</div>
      ))}
    </div>
  )
}

// A named set of symbols worth looking at, each opening the same lookup modal a
// row does. Empty sets still render, so the band does not resize as prices move.
// A signal card. Six of these stacked open cost more vertical space than the
// table they sit above, and most are empty most of the time, so the body is
// collapsed by default: the header still carries the count, which is the part
// worth seeing at a glance. An empty card does not expand at all — there is
// nothing behind it to show.
function SignalCard({
  title,
  tone,
  count,
  children,
}: {
  title: string
  tone: string
  count?: number
  children: ReactNode
}) {
  const [open, setOpen] = useState(false)
  const empty = count === 0
  return (
    <div style={{ flex: 1, minWidth: 0, alignSelf: 'flex-start', background: MC.surf, border: `1px solid ${MC.line}`, borderRadius: 9, padding: '7px 9px' }}>
      <button
        onClick={() => !empty && setOpen(v => !v)}
        disabled={empty}
        aria-expanded={open}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          width: '100%',
          padding: 0,
          background: 'transparent',
          border: 'none',
          cursor: empty ? 'default' : 'pointer',
          textAlign: 'left',
        }}
      >
        <span style={{ width: 3, height: 9, borderRadius: 2, background: empty ? MC.line : tone, flexShrink: 0 }} />
        <span style={{ fontSize: 9, fontWeight: 700, letterSpacing: '0.07em', textTransform: 'uppercase', color: MC.dim, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          {title}
        </span>
        {count != null && (
          <span style={{ fontSize: 9.5, fontFamily: "'DM Mono', monospace", color: empty ? MC.dim : tone }}>{count}</span>
        )}
        {!empty && (
          <span style={{ marginLeft: 'auto', fontSize: 8, color: MC.dim, transform: open ? 'rotate(90deg)' : 'none', transition: 'transform .12s' }}>
            ▶
          </span>
        )}
      </button>
      {open && <div style={{ marginTop: 6 }}>{children}</div>}
    </div>
  )
}

function SignalList({
  title,
  tone,
  items,
  note,
}: {
  title: string
  tone: string
  items: BandRow[]
  note?: (item: BandRow) => string
}) {
  return (
    <SignalCard title={title} tone={tone} count={items.length}>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 5 }}>
        {items.map(i => (
          <button
            key={i.symbol}
            onClick={() => openStockLookup(i.symbol)}
            title={i.symbol}
            style={{
              display: 'inline-flex',
              alignItems: 'baseline',
              gap: 4,
              padding: '2px 7px',
              borderRadius: 999,
              background: tone + '14',
              border: `1px solid ${tone}33`,
              color: tone,
              fontSize: 10.5,
              fontWeight: 600,
              fontFamily: "'DM Sans', sans-serif",
              cursor: 'pointer',
            }}
          >
            {i.symbol}
            {note && <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 9.5, opacity: 0.8 }}>{note(i)}</span>}
          </button>
        ))}
      </div>
    </SignalCard>
  )
}

// The column-mode switch sits on the caption line above the table rather than
// in the overflow menu: the caption is already telling the user the table
// scrolls, and a control two taps away behind "⋯" read as unrelated to it.
