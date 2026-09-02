import { useState } from 'react'
import type { PortfolioData, Position } from '../types/portfolio'
import { fmt, fmtCurrency, fmtPct, fmtK, clr } from '../utils/format'
import { BROKER_COLORS } from '../constants'
import { HorizBar } from '../components/charts/HorizBar'
import { StatCard } from '../components/ui/StatCard'
import { SectionLabel } from '../components/ui/SectionLabel'
import { BrokerPill } from '../components/ui/BrokerPill'
import { InfoTooltip } from '../components/ui/InfoTooltip'
import { openStockLookup } from '../components/StockLookup'
import { IndicatorCells, INDICATOR_INFO, indicatorSortValue } from '../components/IndicatorColumns'

interface Props {
  data: PortfolioData
  accent: string
}

type ExportFormat = 'csv' | 'md'

type SortKey = 'symbol' | 'quantity' | 'mv' | 'cost' | 'pnl' | 'pct' | 'range' | 'pe' | 'forwardPe' | 'today' | 'oneWeek' | 'oneMonth' | 'ytd' | 'threeYr' | 'fiveYr' | 'tenYr' | 'fcf' | 'evToEbitda' | 'debtToEquity' | 'cashFlowQuality' | 'health' | 'valuation' | 'alloc'
type SortDir = 'asc' | 'desc'

interface Column {
  key: SortKey
  label: string
  align?: 'left' | 'right'
}

const COLUMNS: Column[] = [
  { key: 'symbol', label: 'Symbol', align: 'left' },
  { key: 'quantity', label: 'Quantity' },
  { key: 'mv', label: 'Market Value' },
  { key: 'cost', label: 'Cost Basis' },
  { key: 'pnl', label: 'Unrealized P&L' },
  { key: 'pct', label: 'Return' },
  { key: 'today', label: '1D' },
  { key: 'oneWeek', label: '1W' },
  { key: 'oneMonth', label: '1M' },
  { key: 'ytd', label: 'YTD' },
  { key: 'threeYr', label: '3Y' },
  { key: 'fiveYr', label: '5Y' },
  { key: 'tenYr', label: '10Y' },
  { key: 'range', label: '52 Week Low/High' },
  { key: 'pe', label: 'P/E' },
  { key: 'forwardPe', label: 'Forward P/E' },
  { key: 'fcf', label: 'FCF' },
  { key: 'evToEbitda', label: 'EV/EBITDA' },
  { key: 'debtToEquity', label: 'Debt/Equity' },
  { key: 'cashFlowQuality', label: 'CF Quality' },
  { key: 'health', label: 'Health' },
  { key: 'valuation', label: 'Valuation' },
  { key: 'alloc', label: 'Allocation' },
]

function sortValue(p: Position, key: SortKey): number | string {
  switch (key) {
    case 'symbol':
      return p.symbol
    case 'quantity':
      return p.quantity ?? -Infinity
    case 'mv':
    case 'alloc':
      return p.mv ?? -Infinity
    case 'cost':
      return p.cost
    case 'pnl':
      return p.pnl ?? -Infinity
    case 'pct':
      return p.pct ?? -Infinity
    default:
      return indicatorSortValue(p, key)
  }
}

function sortPositions(positions: Position[], key: SortKey, dir: SortDir): Position[] {
  const mul = dir === 'asc' ? 1 : -1
  return positions.slice().sort((a, b) => {
    const va = sortValue(a, key)
    const vb = sortValue(b, key)
    if (typeof va === 'string' || typeof vb === 'string') {
      return mul * String(va).localeCompare(String(vb))
    }
    return mul * (va - vb)
  })
}

const EXPORT_HEADERS = ['Symbol', 'Quantity', 'Market Value', 'Cost Basis', 'Unrealized P&L', 'Return %', 'Allocation %']

function exportRow(p: Position, totalMV: number): (string | number)[] {
  const alloc = totalMV > 0 ? ((p.mv ?? 0) / totalMV) * 100 : 0
  const r2 = (n: number) => Number(n.toFixed(2))
  return [p.symbol, p.quantity ?? 0, r2(p.mv ?? 0), r2(p.cost), r2(p.pnl ?? 0), r2(p.pct ?? 0), r2(alloc)]
}

function toCsv(positions: Position[], totalMV: number): string {
  const esc = (v: string | number) => {
    const s = String(v)
    return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s
  }
  const lines = [EXPORT_HEADERS.join(',')]
  for (const p of positions) lines.push(exportRow(p, totalMV).map(esc).join(','))
  return lines.join('\n')
}

function toMarkdown(positions: Position[], totalMV: number): string {
  const lines = [
    `| ${EXPORT_HEADERS.join(' | ')} |`,
    `| ${EXPORT_HEADERS.map(() => '---').join(' | ')} |`,
  ]
  for (const p of positions) lines.push(`| ${exportRow(p, totalMV).join(' | ')} |`)
  return lines.join('\n')
}

function downloadPositions(positions: Position[], totalMV: number, format: ExportFormat) {
  const content = format === 'csv' ? toCsv(positions, totalMV) : toMarkdown(positions, totalMV)
  const mime = format === 'csv' ? 'text/csv' : 'text/markdown'
  const blob = new Blob([content], { type: `${mime};charset=utf-8` })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `positions.${format}`
  a.click()
  URL.revokeObjectURL(url)
}

export function PositionsTab({ data, accent }: Props) {
  const [format, setFormat] = useState<ExportFormat>('csv')
  const [sortKey, setSortKey] = useState<SortKey>('mv')
  const [sortDir, setSortDir] = useState<SortDir>('desc')
  const totalMV = data.openPositions.reduce((s, p) => s + (p.mv ?? 0), 0)
  const totalUPnl = data.openPositions.reduce((s, p) => s + (p.pnl ?? 0), 0)

  const best = data.openPositions.reduce(
    (best, p) => ((p.pct ?? -Infinity) > (best.pct ?? -Infinity) ? p : best),
    data.openPositions[0]
  )

  const sorted = sortPositions(data.openPositions, sortKey, sortDir)

  function handleSort(key: SortKey) {
    if (key === sortKey) {
      setSortDir(d => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortKey(key)
      setSortDir(key === 'symbol' ? 'asc' : 'desc')
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,1fr)', gap: 10 }}>
        <StatCard label="Portfolio Value" value={fmtCurrency(totalMV)} sub={`${data.openPositions.length} positions`} />
        <StatCard label="Unrealized P&L" value={fmtCurrency(totalUPnl)} sub={fmtPct(totalMV > 0 ? (totalUPnl / totalMV) * 100 : 0)} valueColor={clr(totalUPnl)} />
        <StatCard
          label="Best Performer"
          value={best?.symbol ?? '—'}
          sub={best ? `${fmtPct(best.pct)} unrealized` : ''}
          valueColor="#34d399"
        />
      </div>

      {/* All positions table */}
      <div style={{ background: '#090f1c', borderRadius: 10, border: '1px solid #161f31', overflow: 'hidden' }}>
        <div style={{ padding: '14px 18px', borderBottom: '1px solid #161f31', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 10 }}>
          <SectionLabel>Open Positions</SectionLabel>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <select
              value={format}
              onChange={e => setFormat(e.target.value as ExportFormat)}
              style={{ background: '#080808', color: '#c0c0c0', border: '1px solid #161f31', borderRadius: 6, padding: '5px 8px', fontSize: 11, fontWeight: 600, cursor: 'pointer' }}
            >
              <option value="csv">CSV</option>
              <option value="md">Markdown</option>
            </select>
            <button
              onClick={() => downloadPositions(sorted, totalMV, format)}
              disabled={sorted.length === 0}
              style={{ background: accent + '22', color: accent, border: `1px solid ${accent}33`, borderRadius: 6, padding: '5px 12px', fontSize: 11, fontWeight: 600, cursor: sorted.length === 0 ? 'default' : 'pointer', opacity: sorted.length === 0 ? 0.5 : 1 }}
            >
              Download
            </button>
          </div>
        </div>
        <div style={{ overflowX: 'auto', WebkitOverflowScrolling: 'touch' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', minWidth: 520 }}>
            <thead>
              <tr style={{ background: '#080808' }}>
                {COLUMNS.map(col => (
                  <th
                    key={col.key}
                    onClick={() => handleSort(col.key)}
                    style={{
                      padding: '8px 14px',
                      textAlign: col.align ?? 'right',
                      fontSize: 10,
                      fontWeight: 600,
                      color: sortKey === col.key ? '#c0c0c0' : '#8b8fa3',
                      letterSpacing: '0.08em',
                      textTransform: 'uppercase',
                      whiteSpace: 'nowrap',
                      cursor: 'pointer',
                      userSelect: 'none',
                      ...(col.key === 'symbol' ? { position: 'sticky', left: 0, background: '#080808', zIndex: 2, borderRight: '1px solid #161f31' } : {}),
                    }}
                  >
                    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, flexDirection: col.align === 'left' ? 'row' : 'row-reverse' }}>
                      {col.label}
                      <span style={{ fontSize: 8, opacity: sortKey === col.key ? 1 : 0.25 }}>{sortKey === col.key && sortDir === 'asc' ? '▲' : '▼'}</span>
                      {INDICATOR_INFO[col.key] && <InfoTooltip text={INDICATOR_INFO[col.key]!} />}
                    </span>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {sorted.map(p => (
                <tr
                  key={p.symbol}
                  onClick={() => openStockLookup(p.symbol)}
                  style={{ borderTop: '1px solid #161f31', transition: 'background 0.1s', cursor: 'pointer' }}
                  onMouseEnter={e => {
                    e.currentTarget.style.background = accent + '1c'
                    ;(e.currentTarget.firstElementChild as HTMLElement).style.background = '#1f1f1f'
                  }}
                  onMouseLeave={e => {
                    e.currentTarget.style.background = 'transparent'
                    ;(e.currentTarget.firstElementChild as HTMLElement).style.background = '#090f1c'
                  }}
                >
                  <td style={{ padding: '10px 14px', position: 'sticky', left: 0, background: '#090f1c', zIndex: 1, borderRight: '1px solid #161f31' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <div style={{ width: 26, height: 26, borderRadius: 6, background: accent + '22', border: `1px solid ${accent}33`, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 9, fontWeight: 700, color: accent, flexShrink: 0 }}>
                        {p.symbol.slice(0, 3)}
                      </div>
                      <span style={{ fontWeight: 600, fontSize: 13 }}>{p.symbol}</span>
                    </div>
                  </td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono', monospace", fontSize: 13, color: '#c0c0c0' }}>{p.quantity != null ? p.quantity.toLocaleString('en-US', { maximumFractionDigits: 4 }) : '—'}</td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', fontWeight: 600, fontFamily: "'DM Mono', monospace", fontSize: 13 }}>{fmtCurrency(p.mv)}</td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', color: '#888888', fontFamily: "'DM Mono', monospace", fontSize: 12 }}>{fmtCurrency(p.cost)}</td>
                  <td style={{ padding: '10px 14px', textAlign: 'right', fontWeight: 600, fontFamily: "'DM Mono', monospace", fontSize: 13, color: clr(p.pnl) }}>{fmtCurrency(p.pnl)}</td>
                  <td style={{ padding: '10px 14px', textAlign: 'right' }}>
                    <span style={{ padding: '2px 7px', borderRadius: 4, fontSize: 11, fontWeight: 600, background: clr(p.pct) + '22', color: clr(p.pct) }}>{fmtPct(p.pct)}</span>
                  </td>
                  <IndicatorCells p={p} />
                  <td style={{ padding: '10px 14px', textAlign: 'right' }}>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 3, alignItems: 'flex-end' }}>
                      <span style={{ fontSize: 11, color: '#c0c0c0' }}>{fmt(totalMV > 0 ? ((p.mv ?? 0) / totalMV) * 100 : 0, 1)}%</span>
                      <div style={{ width: 60 }}>
                        <HorizBar value={p.mv ?? 0} total={totalMV} color={accent} height={3} />
                      </div>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Per-broker position cards */}
      <div className="grid-2">
        {data.brokers.filter(b => b.positions.some(p => p.mv != null)).map(b => {
          const bc = BROKER_COLORS[b.name] ?? '#c0c0c0'
          return (
            <div key={b.name} style={{ background: '#090f1c', borderRadius: 10, border: '1px solid #161f31', overflow: 'hidden' }}>
              <div style={{ padding: '12px 16px', borderBottom: '1px solid #161f31', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <BrokerPill name={b.name} />
                <div style={{ fontSize: 11, color: '#8b8fa3' }}>{b.currency}</div>
              </div>
              <div style={{ padding: '8px 0' }}>
                {b.positions.filter(p => p.mv != null).map(p => (
                  <div key={p.symbol} onClick={() => openStockLookup(p.symbol)} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '7px 16px', cursor: 'pointer' }}>
                    <div style={{ width: 24, height: 24, borderRadius: 5, flexShrink: 0, background: bc + '22', border: `1px solid ${bc}33`, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 8, fontWeight: 700, color: bc }}>
                      {p.symbol.slice(0, 3)}
                    </div>
                    <span style={{ flex: 1, fontWeight: 500, fontSize: 12 }}>{p.symbol}</span>
                    <span style={{ fontFamily: "'DM Mono', monospace", fontSize: 12, color: '#c0c0c0' }}>{fmtK(p.mv)}</span>
                    <span style={{ padding: '1px 6px', borderRadius: 3, fontSize: 10, fontWeight: 600, background: clr(p.pnl) + '22', color: clr(p.pnl), minWidth: 52, textAlign: 'right' }}>
                      {fmtPct(p.pct)}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
