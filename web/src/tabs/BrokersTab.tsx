import { useState } from 'react'
import type { PortfolioData, BrokerData } from '../types/portfolio'
import { fmt, fmtCurrency, fmtPct, fmtK, clr } from '../utils/format'
import { BROKER_COLORS, BROKER_LABELS } from '../constants'
import { BarChart } from '../components/charts/BarChart'
import { HorizBar } from '../components/charts/HorizBar'
import { SectionLabel } from '../components/ui/SectionLabel'
import { BrokerPill } from '../components/ui/BrokerPill'

interface Props {
  data: PortfolioData
  accent: string
}

function IndividualBroker({ b }: { b: BrokerData }) {
  const bColor = BROKER_COLORS[b.name] ?? '#c0c0c0'
  const cur = b.currency
  const fc = (n: number | null | undefined) => fmtCurrency(n, cur)
  const fk = (n: number | null | undefined) => fmtK(n, cur)
  const openMV = b.positions.reduce((s, p) => s + (p.mv ?? 0), 0)
  const openPnl = b.positions.reduce((s, p) => s + (p.pnl ?? 0), 0)
  const maxLoss = Math.max(...(b.realizedBySymbol.filter(x => x.pnl < 0).map(x => Math.abs(x.pnl))), 1)
  const maxGain = Math.max(...(b.realizedBySymbol.filter(x => x.pnl > 0).map(x => x.pnl)), 1)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div className="kpi-5" style={{ gridTemplateColumns: 'repeat(6,1fr)' }}>
        {[
          { label: 'All-Time Return', value: fmtPct(b.allTimeGain), color: clr(b.allTimeGain) },
          { label: 'All-Time Realized', value: fc(b.allTimeRPnl), color: clr(b.allTimeRPnl) },
          { label: 'YTD Return', value: fmtPct(b.ytdGain), color: clr(b.ytdGain) },
          { label: 'YTD Realized', value: fc(b.ytdRPnl), color: clr(b.ytdRPnl) },
          { label: 'MTD Return', value: fmtPct(b.mtdGain), color: clr(b.mtdGain) },
          { label: 'Total Dividends', value: fc(Math.abs(b.dividends)), color: '#facc15' },
        ].map(k => (
          <div key={k.label} style={{ background: '#0f0f0f', borderRadius: 10, padding: '12px 14px', border: `1px solid ${bColor}22` }}>
            <div style={{ fontSize: 10, color: '#555555', fontWeight: 500, textTransform: 'uppercase', letterSpacing: '0.07em', marginBottom: 4 }}>{k.label}</div>
            <div style={{ fontSize: 20, fontWeight: 700, color: k.color }}>{k.value}</div>
          </div>
        ))}
      </div>

      <div className="grid-2">
        <div style={{ background: '#0f0f0f', borderRadius: 10, padding: '16px 18px', border: '1px solid #1a1a1a' }}>
          <SectionLabel>Annual Return — {BROKER_LABELS[b.name] ?? b.name}</SectionLabel>
          {b.byYear.length > 0 ? (
            <>
              <BarChart
                data={b.byYear.map(y => ({ year: y.label.slice(2), val: y.gainPct }))}
                keyX="year" keyY="val"
                colorFn={v => v >= 0 ? bColor : '#f87171'}
                height={120}
              />
              <div style={{ display: 'flex', gap: 4, marginTop: 8, flexWrap: 'wrap' }}>
                {b.byYear.map(y => (
                  <div key={y.label} style={{ flex: 1, textAlign: 'center' }}>
                    <div style={{ fontSize: 9, color: y.gainPct >= 0 ? bColor : '#f87171', fontWeight: 600 }}>
                      {y.gainPct >= 0 ? '+' : ''}{fmt(y.gainPct, 1)}%
                    </div>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <div style={{ color: '#555555', fontSize: 12, padding: '20px 0' }}>No yearly data</div>
          )}
        </div>

        <div style={{ background: '#0f0f0f', borderRadius: 10, border: '1px solid #1a1a1a', overflow: 'hidden' }}>
          <div style={{ padding: '14px 18px', borderBottom: '1px solid #1a1a1a', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <SectionLabel>Open Positions</SectionLabel>
            <div style={{ display: 'flex', gap: 12 }}>
              <span style={{ fontSize: 12, color: '#c0c0c0' }}>MV: <strong style={{ color: '#ffffff' }}>{fc(openMV)}</strong></span>
              <span style={{ fontSize: 12, color: clr(openPnl) }}>U.PnL: <strong>{fc(openPnl)}</strong></span>
            </div>
          </div>
          <div style={{ maxHeight: 300, overflowY: 'auto', overflowX: 'auto', WebkitOverflowScrolling: 'touch' as const }}>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ background: '#080808', position: 'sticky', top: 0 }}>
                  {['Symbol', 'Qty', 'Avg Cost', 'Cur. Price', 'Mkt Value', 'Cost', 'U.PnL', 'Return'].map(h => (
                    <th key={h} style={{ padding: '7px 14px', textAlign: h === 'Symbol' ? 'left' : 'right', fontSize: 10, fontWeight: 600, color: '#555555', letterSpacing: '0.07em', textTransform: 'uppercase' }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {b.positions.slice().sort((x, y) => (y.mv ?? 0) - (x.mv ?? 0)).map(p => (
                  <tr key={p.symbol} style={{ borderTop: '1px solid #1a1a1a' }}
                    onMouseEnter={e => (e.currentTarget.style.background = '#1a1a1a44')}
                    onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                  >
                    <td style={{ padding: '8px 14px' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
                        <div style={{ width: 24, height: 24, borderRadius: 5, flexShrink: 0, background: bColor + '22', border: `1px solid ${bColor}33`, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 8, fontWeight: 700, color: bColor }}>{p.symbol.slice(0, 3)}</div>
                        <span style={{ fontWeight: 600, fontSize: 12 }}>{p.symbol}</span>
                      </div>
                    </td>
                    <td style={{ padding: '8px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: '#c0c0c0' }}>{p.quantity != null ? fmt(p.quantity, 1) : '—'}</td>
                    <td style={{ padding: '8px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: '#888888' }}>{p.avgCost != null ? fc(p.avgCost) : '—'}</td>
                    <td style={{ padding: '8px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: '#c0c0c0' }}>{p.currentPrice != null ? fc(p.currentPrice) : '—'}</td>
                    <td style={{ padding: '8px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 600 }}>{p.mv != null ? fc(p.mv) : '—'}</td>
                    <td style={{ padding: '8px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: '#888888' }}>{fc(p.cost)}</td>
                    <td style={{ padding: '8px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: clr(p.pnl) }}>{p.pnl != null ? fc(p.pnl) : '—'}</td>
                    <td style={{ padding: '8px 14px', textAlign: 'right' }}>
                      {p.pct != null
                        ? <span style={{ padding: '1px 6px', borderRadius: 3, fontSize: 10, fontWeight: 600, background: clr(p.pct) + '22', color: clr(p.pct) }}>{fmtPct(p.pct)}</span>
                        : <span style={{ color: '#555555' }}>—</span>}
                    </td>
                  </tr>
                ))}
              </tbody>
              {b.positions.some(p => p.mv != null) && (
                <tfoot>
                  <tr style={{ borderTop: '2px solid #252525', background: '#080808' }}>
                    <td style={{ padding: '8px 14px', fontSize: 11, fontWeight: 700, color: '#c0c0c0' }}>Total</td>
                    <td /><td /><td />
                    <td style={{ padding: '8px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 700 }}>{fc(openMV)}</td>
                    <td />
                    <td style={{ padding: '8px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 700, color: clr(openPnl) }}>{fc(openPnl)}</td>
                    <td />
                  </tr>
                </tfoot>
              )}
            </table>
          </div>
        </div>
      </div>

      {/* Gainers / Losers */}
      <div className="grid-2">
        {[
          { title: 'Top Realized Gainers', items: b.realizedBySymbol.filter(p => p.pnl > 0).sort((a, c) => c.pnl - a.pnl).slice(0, 8), max: maxGain, color: '#34d399' },
          { title: 'Top Realized Losers', items: b.realizedBySymbol.filter(p => p.pnl < 0).sort((a, c) => a.pnl - c.pnl).slice(0, 8), max: maxLoss, color: '#f87171' },
        ].map(({ title, items, max, color }) => (
          <div key={title} style={{ background: '#0f0f0f', borderRadius: 10, border: '1px solid #1a1a1a', overflow: 'hidden' }}>
            <div style={{ padding: '14px 18px', borderBottom: '1px solid #1a1a1a' }}>
              <SectionLabel>{title}</SectionLabel>
            </div>
            <div style={{ padding: '8px 0' }}>
              {items.length === 0 ? (
                <div style={{ padding: '16px', color: '#555555', fontSize: 12 }}>None</div>
              ) : items.map((p, i) => (
                <div key={p.symbol} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '7px 16px' }}>
                  <span style={{ width: 16, fontSize: 11, color: '#252525', fontWeight: 600, textAlign: 'right' }}>{i + 1}</span>
                  <div style={{ width: 26, height: 26, borderRadius: 5, flexShrink: 0, background: color + '22', border: `1px solid ${color}33`, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 8, fontWeight: 700, color }}>{p.symbol.slice(0, 3)}</div>
                  <span style={{ width: 52, fontSize: 11, fontWeight: 500 }}>{p.symbol}</span>
                  <div style={{ flex: 1, paddingRight: 6 }}><HorizBar value={Math.abs(p.pnl)} total={max} color={color} height={4} /></div>
                  <span style={{ fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 600, color, minWidth: 64, textAlign: 'right' }}>
                    {p.pnl >= 0 ? '+' : ''}{fc(p.pnl)}
                  </span>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>

      {/* Annual breakdown table */}
      {b.byYear.length > 0 && (
        <div style={{ background: '#0f0f0f', borderRadius: 10, border: '1px solid #1a1a1a', overflow: 'hidden' }}>
          <div style={{ padding: '14px 18px', borderBottom: '1px solid #1a1a1a' }}>
            <SectionLabel>Annual Breakdown — {BROKER_LABELS[b.name] ?? b.name}</SectionLabel>
          </div>
          <div style={{ overflowX: 'auto', WebkitOverflowScrolling: 'touch' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', minWidth: 480 }}>
            <thead>
              <tr style={{ background: '#080808' }}>
                {['Year', 'Return %', 'Realized PnL', 'Dividends', 'Deposits', 'Buy Vol', 'Sell Vol'].map(h => (
                  <th key={h} style={{ padding: '8px 14px', textAlign: h === 'Year' ? 'left' : 'right', fontSize: 10, fontWeight: 600, color: '#555555', letterSpacing: '0.07em', textTransform: 'uppercase' }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {b.byYear.map(y => (
                <tr key={y.label} style={{ borderTop: '1px solid #1a1a1a' }}
                  onMouseEnter={e => (e.currentTarget.style.background = '#1a1a1a44')}
                  onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                >
                  <td style={{ padding: '9px 14px', fontWeight: 600 }}>{y.label}</td>
                  <td style={{ padding: '9px 14px', textAlign: 'right' }}>
                    <span style={{ padding: '2px 7px', borderRadius: 4, fontSize: 11, fontWeight: 600, background: clr(y.gainPct) + '22', color: clr(y.gainPct) }}>{fmtPct(y.gainPct)}</span>
                  </td>
                  <td style={{ padding: '9px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: clr(y.rPnl) }}>{fc(y.rPnl)}</td>
                  <td style={{ padding: '9px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: '#facc15' }}>{fc(y.divs)}</td>
                  <td style={{ padding: '9px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: '#888888' }}>{fc(y.deposits)}</td>
                  <td style={{ padding: '9px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: '#555555' }}>{fk(y.buyVol)}</td>
                  <td style={{ padding: '9px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: '#555555' }}>{fk(y.sellVol)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          </div>
        </div>
      )}
    </div>
  )
}

function MiniStat({ label, value, color, large, mono }: { label: string; value: string; color: string; large?: boolean; mono?: boolean }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
      <span style={{ fontSize: 9, color: '#555555', textTransform: 'uppercase', letterSpacing: '0.05em' }}>{label}</span>
      <span style={{
        fontSize: large ? 14 : 11,
        fontWeight: large ? 700 : 500,
        color,
        fontFamily: mono ? "'DM Mono',monospace" : 'inherit',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
      }}>{value}</span>
    </div>
  )
}

export function BrokersTab({ data, accent }: Props) {
  const [activeBroker, setActiveBroker] = useState<string>('all')

  const maxRPnl = Math.max(...data.brokers.map(b => Math.abs(b.allTimeRPnl)), 1)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* Selector */}
      <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
        <button
          onClick={() => setActiveBroker('all')}
          style={{
            padding: '5px 14px', borderRadius: 999, fontSize: 11, fontWeight: 600, cursor: 'pointer',
            background: activeBroker === 'all' ? accent + '33' : '#0f0f0f',
            border: activeBroker === 'all' ? `1px solid ${accent}66` : '1px solid #1a1a1a',
            color: activeBroker === 'all' ? accent : '#888888',
          }}
        >All Brokers</button>
        {data.brokers.map(b => {
          const bc = BROKER_COLORS[b.name] ?? '#c0c0c0'
          return (
            <button
              key={b.name}
              onClick={() => setActiveBroker(b.name)}
              style={{
                padding: '5px 14px', borderRadius: 999, fontSize: 11, fontWeight: 600, cursor: 'pointer',
                background: activeBroker === b.name ? bc + '33' : '#0f0f0f',
                border: activeBroker === b.name ? `1px solid ${bc}66` : '1px solid #1a1a1a',
                color: activeBroker === b.name ? bc : '#888888',
                display: 'flex', alignItems: 'center', gap: 5,
              }}
            >
              <span style={{ width: 6, height: 6, borderRadius: '50%', background: bc }} />
              {BROKER_LABELS[b.name] ?? b.name}
            </button>
          )
        })}
      </div>

      {/* All brokers view */}
      {activeBroker === 'all' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          {/* Comparison table */}
          <div style={{ background: '#0f0f0f', borderRadius: 10, border: '1px solid #1a1a1a', overflow: 'hidden' }}>
            <div style={{ padding: '14px 18px', borderBottom: '1px solid #1a1a1a' }}>
              <SectionLabel>Broker Comparison</SectionLabel>
            </div>
            <div style={{ overflowX: 'auto', WebkitOverflowScrolling: 'touch' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', minWidth: 880 }}>
                <thead>
                  <tr style={{ background: '#080808' }}>
                    {['Broker', 'Currency', 'Value', 'Cash', 'Deposits', 'Transfer In', 'Transfer Out', 'All-Time Return', 'All-Time R.PnL', 'YTD Return', 'YTD R.PnL', 'YTD Divs', 'MTD Return', 'Dividends (AT)', 'Unrealized P&L', 'Open Positions'].map(h => (
                      <th key={h} style={{ padding: '8px 14px', textAlign: h === 'Broker' ? 'left' : 'right', fontSize: 10, fontWeight: 600, color: '#555555', letterSpacing: '0.07em', textTransform: 'uppercase', whiteSpace: 'nowrap' }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {data.brokers.map(b => (
                    <tr
                      key={b.name}
                      style={{ borderTop: '1px solid #1a1a1a', cursor: 'pointer' }}
                      onClick={() => setActiveBroker(b.name)}
                      onMouseEnter={e => (e.currentTarget.style.background = '#1a1a1a55')}
                      onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                    >
                      <td style={{ padding: '10px 14px' }}><BrokerPill name={b.name} /></td>
                      <td style={{ padding: '10px 14px', textAlign: 'right', fontSize: 11, color: '#888888' }}>{b.currency}</td>
                      <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 600, color: '#ffffff' }}>{fmtCurrency(b.positions.reduce((s, p) => s + (p.mv ?? 0), 0) + b.cashBalance, b.currency)}</td>
                      <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 600, color: b.cashBalance >= 0 ? '#34d399' : '#f87171' }}>{fmtCurrency(b.cashBalance, b.currency)}</td>
                      <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: '#c0c0c0' }}>{fmtCurrency(b.deposits, b.currency)}</td>
                      <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: '#c0c0c0' }}>{b.transferIn > 0 ? fmtCurrency(b.transferIn, b.currency) : '—'}</td>
                      <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: '#c0c0c0' }}>{b.transferOut > 0 ? fmtCurrency(b.transferOut, b.currency) : '—'}</td>
                      <td style={{ padding: '10px 14px', textAlign: 'right' }}>
                        <span style={{ padding: '2px 7px', borderRadius: 4, fontSize: 11, fontWeight: 600, background: clr(b.allTimeGain) + '22', color: clr(b.allTimeGain) }}>{fmtPct(b.allTimeGain)}</span>
                      </td>
                      <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 600, color: clr(b.allTimeRPnl) }}>{fmtCurrency(b.allTimeRPnl, b.currency)}</td>
                      <td style={{ padding: '10px 14px', textAlign: 'right' }}>
                        <span style={{ padding: '2px 7px', borderRadius: 4, fontSize: 11, fontWeight: 600, background: clr(b.ytdGain) + '22', color: clr(b.ytdGain) }}>{fmtPct(b.ytdGain)}</span>
                      </td>
                      <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: clr(b.ytdRPnl) }}>{fmtCurrency(b.ytdRPnl, b.currency)}</td>
                      <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: '#c0c0c0' }}>{fmtCurrency(b.ytdDivs, b.currency)}</td>
                      <td style={{ padding: '10px 14px', textAlign: 'right' }}>
                        <span style={{ padding: '2px 7px', borderRadius: 4, fontSize: 11, fontWeight: 600, background: clr(b.mtdGain) + '22', color: clr(b.mtdGain) }}>{fmtPct(b.mtdGain)}</span>
                      </td>
                      <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, color: '#facc15' }}>{fmtCurrency(Math.abs(b.dividends), b.currency)}</td>
                      <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 600, color: clr(b.positions.reduce((s, p) => s + (p.pnl ?? 0), 0)) }}>
                        {b.positions.some(p => p.pnl != null) ? fmtCurrency(b.positions.reduce((s, p) => s + (p.pnl ?? 0), 0), b.currency) : '—'}
                      </td>
                      <td style={{ padding: '10px 14px', textAlign: 'right', fontSize: 12, color: '#c0c0c0' }}>
                        {b.positions.filter(p => p.mv != null).length}{' '}
                        <span style={{ color: '#555555', fontSize: 10 }}>/ {b.positions.length}</span>
                      </td>
                    </tr>
                  ))}
                  <tr style={{ borderTop: '2px solid #252525', background: '#080808' }}>
                    <td style={{ padding: '10px 14px', fontWeight: 700, color: '#ffffff', fontSize: 11 }}>TOTAL / ALL</td>
                    <td style={{ padding: '10px 14px', textAlign: 'right', fontSize: 11, color: '#555555' }}>USD</td>
                    <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 700, color: '#ffffff' }}>{fmtCurrency(data.openPositions.reduce((s, p) => s + (p.mv ?? 0), 0) + data.cashBalance)}</td>
                    <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 700, color: data.cashBalance >= 0 ? '#34d399' : '#f87171' }}>{fmtCurrency(data.cashBalance)}</td>
                    <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 700, color: '#ffffff' }}>{fmtCurrency(data.allTime.deposits)}</td>
                    <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 700, color: '#ffffff' }}>{fmtCurrency(data.allTime.transferIn)}</td>
                    <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 700, color: '#ffffff' }}>{fmtCurrency(data.allTime.transferOut)}</td>
                    <td style={{ padding: '10px 14px', textAlign: 'right' }}>
                      <span style={{ padding: '2px 7px', borderRadius: 4, fontSize: 12, fontWeight: 700, background: clr(data.allTime.gainPct) + '22', color: clr(data.allTime.gainPct) }}>{fmtPct(data.allTime.gainPct)}</span>
                    </td>
                    <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 700, color: clr(data.allTime.realizedPnl) }}>{fmtCurrency(data.allTime.realizedPnl)}</td>
                    <td style={{ padding: '10px 14px', textAlign: 'right' }}>
                      <span style={{ padding: '2px 7px', borderRadius: 4, fontSize: 12, fontWeight: 700, background: clr(data.ytd.gainPct) + '22', color: clr(data.ytd.gainPct) }}>{fmtPct(data.ytd.gainPct)}</span>
                    </td>
                    <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 700, color: clr(data.ytd.realizedPnl) }}>{fmtCurrency(data.ytd.realizedPnl)}</td>
                    <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 700, color: '#ffffff' }}>{fmtCurrency(data.ytd.dividends)}</td>
                    <td style={{ padding: '10px 14px', textAlign: 'right' }}>
                      <span style={{ padding: '2px 7px', borderRadius: 4, fontSize: 12, fontWeight: 700, background: clr(data.mtd.gainPct) + '22', color: clr(data.mtd.gainPct) }}>{fmtPct(data.mtd.gainPct)}</span>
                    </td>
                    <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 700, color: '#facc15' }}>{fmtCurrency(data.allTime.dividends)}</td>
                    {(() => { const totalUpnl = data.openPositions.reduce((s, p) => s + (p.pnl ?? 0), 0); return (
                      <td style={{ padding: '10px 14px', textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 700, color: clr(totalUpnl) }}>{fmtCurrency(totalUpnl)}</td>
                    )})()}
                    <td style={{ padding: '10px 14px', textAlign: 'right', fontSize: 12, color: '#c0c0c0' }}>{data.openPositions.length}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          {/* Bar charts */}
          <div className="grid-2">
            <div style={{ background: '#0f0f0f', borderRadius: 10, padding: '16px 18px', border: '1px solid #1a1a1a' }}>
              <SectionLabel>All-Time Return by Broker</SectionLabel>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 4 }}>
                {data.brokers.map(b => {
                  const bc = BROKER_COLORS[b.name] ?? '#c0c0c0'
                  return (
                    <div key={b.name} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                      <div style={{ width: 76, flexShrink: 0 }}><BrokerPill name={b.name} /></div>
                      <div style={{ flex: 1, display: 'flex', alignItems: 'center', gap: 6 }}>
                        {b.allTimeGain < 0 && (
                          <div style={{ flex: Math.abs(b.allTimeGain) / 50, background: '#f8717133', borderRadius: 3, height: 18, display: 'flex', alignItems: 'center', justifyContent: 'flex-end', paddingRight: 4, minWidth: 4 }} />
                        )}
                        <div style={{ flex: Math.max(b.allTimeGain, 0) / 50, background: bc + '55', borderRadius: 3, height: 18, minWidth: b.allTimeGain > 0 ? 4 : 0, border: `1px solid ${bc}44` }} />
                      </div>
                      <div style={{ width: 58, textAlign: 'right', fontSize: 13, fontWeight: 700, color: clr(b.allTimeGain) }}>{fmtPct(b.allTimeGain)}</div>
                    </div>
                  )
                })}
              </div>
            </div>

            <div style={{ background: '#0f0f0f', borderRadius: 10, padding: '16px 18px', border: '1px solid #1a1a1a' }}>
              <SectionLabel>Realized P&L by Broker (All-Time)</SectionLabel>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 4 }}>
                {data.brokers.slice().sort((a, b) => b.allTimeRPnl - a.allTimeRPnl).map(b => (
                  <div key={b.name} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                    <div style={{ width: 76, flexShrink: 0 }}><BrokerPill name={b.name} /></div>
                    <div style={{ flex: 1 }}>
                      <HorizBar value={Math.abs(b.allTimeRPnl)} total={maxRPnl} color={b.allTimeRPnl >= 0 ? (BROKER_COLORS[b.name] ?? '#c0c0c0') : '#f87171'} height={6} />
                    </div>
                    <div style={{ width: 72, textAlign: 'right', fontFamily: "'DM Mono',monospace", fontSize: 12, fontWeight: 600, color: clr(b.allTimeRPnl) }}>{fmtCurrency(b.allTimeRPnl)}</div>
                  </div>
                ))}
              </div>
            </div>
          </div>

          {/* YTD/MTD mini cards */}
          <div className="broker-cards-5">
            {data.brokers.map(b => {
              const bc = BROKER_COLORS[b.name] ?? '#c0c0c0'
              return (
                <div key={b.name} style={{ background: '#0f0f0f', borderRadius: 10, border: `1px solid ${bc}33`, padding: '12px 14px', minWidth: 0 }}>
                  <BrokerPill name={b.name} />
                  <div style={{ marginTop: 10, display: 'flex', flexDirection: 'column', gap: 5 }}>
                    <MiniStat label="YTD" value={fmtPct(b.ytdGain)} color={clr(b.ytdGain)} large />
                    <MiniStat label="MTD" value={fmtPct(b.mtdGain)} color={clr(b.mtdGain)} large />
                    <div style={{ height: 1, background: '#1a1a1a', margin: '2px 0' }} />
                    <MiniStat label="Cash" value={fmtCurrency(b.cashBalance, b.currency)} color={b.cashBalance >= 0 ? '#34d399' : '#f87171'} mono />
                    <MiniStat label="R.PnL YTD" value={fmtCurrency(b.ytdRPnl, b.currency)} color={clr(b.ytdRPnl)} mono />
                    <MiniStat label="Divs YTD" value={fmtCurrency(b.ytdDivs, b.currency)} color="#facc15" mono />
                    <MiniStat label="All-Time" value={fmtPct(b.allTimeGain)} color={clr(b.allTimeGain)} mono />
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Individual broker */}
      {activeBroker !== 'all' && (() => {
        const b = data.brokers.find(x => x.name === activeBroker)
        if (!b) return null
        return <IndividualBroker b={b} />
      })()}
    </div>
  )
}
